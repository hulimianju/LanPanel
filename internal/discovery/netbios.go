package discovery

import (
	"context"
	"encoding/binary"
	"net"
	"strings"
	"sync"
	"time"
)

// NetBIOS 节点状态查询（NBSTAT，UDP 137）：获取 Windows / Samba 主机的计算机名与工作组。

func nbstatQuery(id uint16) []byte {
	q := make([]byte, 0, 50)
	q = binary.BigEndian.AppendUint16(q, id)
	q = append(q, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0) // flags=0, qd=1
	// 名称 "*" 补齐到 16 字节后做一级编码（每个半字节 + 'A'）
	name := make([]byte, 16)
	name[0] = '*'
	q = append(q, 0x20)
	for _, b := range name {
		q = append(q, 'A'+(b>>4), 'A'+(b&0x0f))
	}
	q = append(q, 0)
	q = append(q, 0, 0x21, 0, 1) // NBSTAT, IN
	return q
}

type nbResult struct {
	Name  string
	Group string
}

func parseNBSTAT(b []byte) (nbResult, bool) {
	var r nbResult
	if len(b) < 12+34+10+1 {
		return r, false
	}
	off := 12
	// 跳过名称（可能是压缩指针）
	if b[off]&0xC0 == 0xC0 {
		off += 2
	} else {
		for off < len(b) && b[off] != 0 {
			off += int(b[off]) + 1
		}
		off++
	}
	off += 10 // TYPE CLASS TTL RDLENGTH
	if off >= len(b) {
		return r, false
	}
	count := int(b[off])
	off++
	for i := 0; i < count && off+18 <= len(b); i++ {
		raw := strings.TrimRight(string(b[off:off+15]), " \x00")
		suffix := b[off+15]
		flags := binary.BigEndian.Uint16(b[off+16 : off+18])
		group := flags&0x8000 != 0
		if suffix == 0x00 {
			if !group && r.Name == "" {
				r.Name = raw
			} else if group && r.Group == "" {
				r.Group = raw
			}
		}
		off += 18
	}
	return r, r.Name != ""
}

// discoverNetBIOS 向每个在线主机单播查询；无应答的主机直接忽略。
func discoverNetBIOS(ctx context.Context, ips []string, col *collector) {
	c, err := net.ListenUDP("udp4", nil)
	if err != nil {
		return
	}
	defer c.Close()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		deadline := time.Now().Add(1500 * time.Millisecond)
		if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
			deadline = d
		}
		_ = c.SetReadDeadline(deadline)
		buf := make([]byte, 1500)
		for {
			n, src, err := c.ReadFromUDP(buf)
			if err != nil {
				return
			}
			r, ok := parseNBSTAT(buf[:n])
			if !ok {
				continue
			}
			ip := src.IP.String()
			col.with(ip, func(o *hostObs) {
				o.Sources["NetBIOS"] = true
				o.Hostnames["NetBIOS"] = r.Name
				o.raw("NetBIOS", r.Name+"  <"+r.Group+">")
				o.hint("pc", 3)
			})
			col.log("NBNS", ip, "%s  <%s>", r.Name, r.Group)
		}
	}()
	for i, ip := range ips {
		dst := &net.UDPAddr{IP: net.ParseIP(ip), Port: 137}
		_, _ = c.WriteToUDP(nbstatQuery(uint16(0x4c50+i)), dst)
	}
	wg.Wait()
}
