package discovery

import (
	"context"
	"errors"
	"net"
	"os"
	"sync"
	"syscall"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

// target 是一个待扫描网段，Iface 非空表示与本机直连（可用 ARP）。
type target struct {
	Net   *net.IPNet
	Iface *localIface
	Hosts []net.IP
}

// arpByTrigger 在没有原始套接字权限时使用：向每个地址发一个 UDP 包，
// 触发内核 ARP 解析，然后读取邻居表。
func arpByTrigger(ctx context.Context, hosts []net.IP) map[string]string {
	c, err := net.ListenUDP("udp4", nil)
	if err == nil {
		defer c.Close()
		// 分批发送：邻居表容量有限（Linux 默认 gc_thresh3=1024）
		for i := 0; i < len(hosts); i += 256 {
			for _, ip := range hosts[i:min(i+256, len(hosts))] {
				_, _ = c.WriteToUDP([]byte{0}, &net.UDPAddr{IP: ip, Port: 9})
			}
			sleepCtx(ctx, 800*time.Millisecond)
		}
		sleepCtx(ctx, 1200*time.Millisecond)
	}
	want := map[string]bool{}
	for _, ip := range hosts {
		want[ip.String()] = true
	}
	out := map[string]string{}
	for ip, mac := range readARP() {
		if want[ip] {
			out[ip] = mac
		}
	}
	return out
}

func sleepCtx(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}

// pingSweep 发送 ICMP Echo。优先原始套接字，其次非特权 ICMP（udp4），都不可用则跳过。
func pingSweep(ctx context.Context, hosts []net.IP, wait time.Duration) (map[string]bool, string) {
	alive := map[string]bool{}
	network, mode := "ip4:icmp", "原始套接字"
	c, err := icmp.ListenPacket(network, "0.0.0.0")
	if err != nil {
		network, mode = "udp4", "非特权模式"
		c, err = icmp.ListenPacket(network, "0.0.0.0")
	}
	if err != nil {
		return alive, "不可用（无权限）"
	}
	defer c.Close()
	id := os.Getpid() & 0xffff
	var mu sync.Mutex
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 1500)
		_ = c.SetReadDeadline(time.Now().Add(time.Duration(len(hosts))*time.Millisecond + wait))
		for {
			n, peer, err := c.ReadFrom(buf)
			if err != nil {
				return
			}
			m, err := icmp.ParseMessage(1, buf[:n])
			if err != nil || m.Type != ipv4.ICMPTypeEchoReply {
				continue
			}
			var ip string
			switch a := peer.(type) {
			case *net.IPAddr:
				ip = a.IP.String()
			case *net.UDPAddr:
				ip = a.IP.String()
			}
			mu.Lock()
			alive[ip] = true
			mu.Unlock()
		}
	}()
	for i, ip := range hosts {
		if ctx.Err() != nil {
			break
		}
		msg := icmp.Message{Type: ipv4.ICMPTypeEcho, Body: &icmp.Echo{ID: id, Seq: i, Data: []byte("lanpanel")}}
		b, _ := msg.Marshal(nil)
		var dst net.Addr = &net.IPAddr{IP: ip}
		if network == "udp4" {
			dst = &net.UDPAddr{IP: ip}
		}
		_, _ = c.WriteTo(b, dst)
		if i%64 == 63 {
			time.Sleep(20 * time.Millisecond)
		}
	}
	<-done
	mu.Lock()
	defer mu.Unlock()
	out := make(map[string]bool, len(alive))
	for k := range alive {
		out[k] = true
	}
	return out, mode
}

// tcpAlive 用于非直连网段：连接成功或被拒绝（RST）都说明主机在线。
func tcpAlive(ctx context.Context, hosts []net.IP, conc int, timeout time.Duration) map[string]bool {
	ports := []int{80, 443, 22, 445, 8080, 3389}
	alive := map[string]bool{}
	var mu sync.Mutex
	sem := make(chan struct{}, conc)
	var wg sync.WaitGroup
	for _, ip := range hosts {
		for _, p := range ports {
			if ctx.Err() != nil {
				break
			}
			wg.Add(1)
			sem <- struct{}{}
			go func(ip net.IP, p int) {
				defer wg.Done()
				defer func() { <-sem }()
				mu.Lock()
				done := alive[ip.String()]
				mu.Unlock()
				if done {
					return
				}
				c, err := (&net.Dialer{Timeout: timeout}).DialContext(ctx, "tcp4", net.JoinHostPort(ip.String(), itoa(p)))
				if err == nil {
					c.Close()
				}
				if err == nil || errors.Is(err, syscall.ECONNREFUSED) {
					mu.Lock()
					alive[ip.String()] = true
					mu.Unlock()
				}
			}(ip, p)
		}
	}
	wg.Wait()
	return alive
}
