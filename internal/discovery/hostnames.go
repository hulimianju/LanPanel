package discovery

import (
	"context"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

// DHCP 租约文件：OpenWrt / dnsmasq 在路由器本机上运行时可以拿到最准确的主机名。
var leaseFiles = []string{"/tmp/dhcp.leases", "/var/lib/misc/dnsmasq.leases", "/var/lib/dnsmasq/dnsmasq.leases"}

func readLeases(col *collector) int {
	n := 0
	for _, f := range leaseFiles {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		// 格式：过期时间 MAC IP 主机名 客户端ID
		for _, line := range strings.Split(string(b), "\n") {
			fl := strings.Fields(line)
			if len(fl) < 4 || fl[3] == "*" {
				continue
			}
			mac, ip, name := normMAC(fl[1]), fl[2], fl[3]
			if net.ParseIP(ip) == nil {
				continue
			}
			col.with(ip, func(o *hostObs) {
				o.Sources["DHCP"] = true
				o.Hostnames["DHCP"] = name
				if o.MAC == "" {
					o.MAC = mac
				}
			})
			n++
		}
	}
	return n
}

// reverseDNS 反查 PTR（路由器的 DNS 通常能解析 DHCP 分配的主机名）。
func reverseDNS(ctx context.Context, ips []string, col *collector) {
	r := &net.Resolver{}
	sem := make(chan struct{}, 16)
	var wg sync.WaitGroup
	for _, ip := range ips {
		wg.Add(1)
		sem <- struct{}{}
		go func(ip string) {
			defer wg.Done()
			defer func() { <-sem }()
			c, cancel := context.WithTimeout(ctx, 1200*time.Millisecond)
			defer cancel()
			names, err := r.LookupAddr(c, ip)
			if err != nil || len(names) == 0 {
				return
			}
			name := strings.TrimSuffix(names[0], ".")
			for _, suf := range []string{".lan", ".local", ".home", ".localdomain", ".home.arpa"} {
				name = strings.TrimSuffix(name, suf)
			}
			if name == "" || net.ParseIP(name) != nil || strings.Contains(name, ip) || strings.Contains(name, strings.ReplaceAll(ip, ".", "-")) {
				return
			}
			col.with(ip, func(o *hostObs) {
				o.Sources["DNS"] = true
				o.Hostnames["DNS"] = name
			})
		}(ip)
	}
	wg.Wait()
}
