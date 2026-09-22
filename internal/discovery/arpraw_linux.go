package discovery

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"syscall"
	"time"
)

// rawARPScan 通过 AF_PACKET 原始套接字主动发送 ARP 请求，只采信真实应答（需要 root 或 CAP_NET_RAW）。
// 相比读取内核邻居表，不会把已离线但尚未过期的条目误判为在线。
func rawARPScan(ctx context.Context, ifi localIface, targets []net.IP, wait time.Duration) (map[string]string, error) {
	if len(ifi.Iface.HardwareAddr) != 6 {
		return nil, errors.New("网卡没有 MAC 地址")
	}
	proto := htons(syscall.ETH_P_ARP)
	fd, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_RAW, int(proto))
	if err != nil {
		return nil, err
	}
	defer syscall.Close(fd)
	addr := &syscall.SockaddrLinklayer{Protocol: proto, Ifindex: ifi.Iface.Index}
	if err := syscall.Bind(fd, addr); err != nil {
		return nil, err
	}
	tv := syscall.NsecToTimeval(int64(200 * time.Millisecond))
	_ = syscall.SetsockoptTimeval(fd, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &tv)

	src := ifi.Iface.HardwareAddr
	srcIP := ifi.IP.To4()
	frame := make([]byte, 42)
	copy(frame[0:6], []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff})
	copy(frame[6:12], src)
	binary.BigEndian.PutUint16(frame[12:14], syscall.ETH_P_ARP)
	binary.BigEndian.PutUint16(frame[14:16], 1)      // 硬件类型：以太网
	binary.BigEndian.PutUint16(frame[16:18], 0x0800) // 协议类型：IPv4
	frame[18], frame[19] = 6, 4
	binary.BigEndian.PutUint16(frame[20:22], 1) // 操作：请求
	copy(frame[22:28], src)
	copy(frame[28:32], srcIP)
	dst := &syscall.SockaddrLinklayer{Ifindex: ifi.Iface.Index, Halen: 6}
	copy(dst.Addr[:], []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff})

	want := map[string]bool{}
	for _, ip := range targets {
		want[ip.String()] = true
	}
	found := map[string]string{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 128)
		deadline := time.Now().Add(time.Duration(len(targets))*time.Millisecond*2 + wait)
		for time.Now().Before(deadline) && ctx.Err() == nil {
			n, _, err := syscall.Recvfrom(fd, buf, 0)
			if err != nil || n < 42 {
				continue
			}
			if binary.BigEndian.Uint16(buf[12:14]) != syscall.ETH_P_ARP || binary.BigEndian.Uint16(buf[20:22]) != 2 {
				continue
			}
			ip := net.IP(buf[28:32]).String()
			if want[ip] {
				found[ip] = normMAC(net.HardwareAddr(buf[22:28]).String())
			}
		}
	}()
	// 发送两轮，间隔发送避免瞬时拥塞
	for round := 0; round < 2; round++ {
		for _, ip := range targets {
			if ctx.Err() != nil {
				break
			}
			copy(frame[38:42], ip.To4())
			_ = syscall.Sendto(fd, frame, 0, dst)
			time.Sleep(time.Millisecond)
		}
		time.Sleep(300 * time.Millisecond)
	}
	<-done
	return found, nil
}

func htons(v uint16) uint16 { return v<<8 | v>>8 }
