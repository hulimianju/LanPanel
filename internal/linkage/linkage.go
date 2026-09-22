// Package linkage 负责面板卡片与设备的联动：解析卡片地址中的 IP，
// 并在设备 IP 变化后改写地址（只替换主机部分，协议、端口、路径、参数保持不变）。
package linkage

import (
	"net"
	"strconv"
	"strings"
)

// hostSpan 返回地址中主机部分在字符串里的起止位置（不含端口与用户信息）。
func hostSpan(raw string) (int, int, bool) {
	i := strings.Index(raw, "://")
	if i <= 0 {
		return 0, 0, false
	}
	start := i + 3
	end := len(raw)
	if j := strings.IndexAny(raw[start:], "/?#"); j >= 0 {
		end = start + j
	}
	auth := raw[start:end]
	if at := strings.LastIndex(auth, "@"); at >= 0 { // user:pass@host
		start += at + 1
		auth = raw[start:end]
	}
	if strings.HasPrefix(auth, "[") { // IPv6 不处理
		return 0, 0, false
	}
	if c := strings.LastIndex(auth, ":"); c >= 0 {
		end = start + c
	}
	return start, end, end > start
}

// HostIP 返回地址中的 IPv4 主机；主机是域名或 IPv6 时返回 nil。
func HostIP(raw string) net.IP {
	s, e, ok := hostSpan(raw)
	if !ok {
		return nil
	}
	ip := net.ParseIP(raw[s:e])
	if ip == nil || ip.To4() == nil {
		return nil
	}
	return ip.To4()
}

// Port 返回地址中的端口；未写端口时按协议推断常见默认端口，无法推断返回 0。
func Port(raw string) int {
	_, e, ok := hostSpan(raw)
	if !ok {
		return 0
	}
	rest := raw[e:]
	if strings.HasPrefix(rest, ":") {
		p := rest[1:]
		if j := strings.IndexAny(p, "/?#"); j >= 0 {
			p = p[:j]
		}
		n, err := strconv.Atoi(p)
		if err == nil && n > 0 && n < 65536 {
			return n
		}
		return 0
	}
	scheme := strings.ToLower(raw[:strings.Index(raw, "://")])
	return map[string]int{"http": 80, "https": 443, "ssh": 22, "ftp": 21, "smb": 445, "rtsp": 554, "vnc": 5900, "afp": 548}[scheme]
}

// ReplaceHost 把地址中的 IPv4 主机替换为 newIP；主机不是 IP（如域名）时不做修改。
func ReplaceHost(raw, newIP string) (string, bool) {
	s, e, ok := hostSpan(raw)
	if !ok || net.ParseIP(raw[s:e]) == nil || raw[s:e] == newIP {
		return raw, false
	}
	return raw[:s] + newIP + raw[e:], true
}

// IsLAN 判断地址是否为局域网地址（只改写局域网 IP，公网 IP 属于外网地址）。
func IsLAN(ip net.IP) bool {
	return ip != nil && (ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		(ip.To4() != nil && ip.To4()[0] == 100 && ip.To4()[1]&0xC0 == 64))
}
