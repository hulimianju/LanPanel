package discovery

import (
	"bufio"
	"net"
	"os"
	"regexp"
	"strings"
)

// localIface 是参与扫描的本机网卡。
type localIface struct {
	Name  string
	IP    net.IP
	Net   *net.IPNet
	MAC   string
	Iface net.Interface
}

// Docker 为自定义网络创建的网桥名为 br-<12 位十六进制>；OpenWrt 的 br-lan 等需要保留
var dockerBridge = regexp.MustCompile(`^br-[0-9a-f]{12}$`)

func virtualIface(name string) bool {
	for _, p := range []string{"docker", "veth", "virbr", "tap", "fwbr", "fwpr", "fwln", "tun", "wg", "utun", "awdl", "llw", "gif", "stf", "anpi", "cali", "flannel", "cni", "kube", "zt", "tailscale", "lo", "vmnet", "bridge1"} { // bridge1xx：macOS 虚拟机网桥
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return dockerBridge.MatchString(name)
}

// localIfaces 返回已启用、带私有 IPv4 地址的物理/网桥网卡。
func localIfaces() []localIface {
	ifs, _ := net.Interfaces()
	var out []localIface
	for _, ifi := range ifs {
		if ifi.Flags&net.FlagUp == 0 || ifi.Flags&net.FlagLoopback != 0 || virtualIface(ifi.Name) {
			continue
		}
		addrs, _ := ifi.Addrs()
		for _, a := range addrs {
			ipn, ok := a.(*net.IPNet)
			if !ok || ipn.IP.To4() == nil || !ipn.IP.IsPrivate() {
				continue
			}
			// Docker 默认网段
			if ipn.IP.To4()[0] == 172 && ipn.IP.To4()[1] == 17 {
				continue
			}
			out = append(out, localIface{
				Name: ifi.Name, IP: ipn.IP.To4(), Net: &net.IPNet{IP: ipn.IP.Mask(ipn.Mask), Mask: ipn.Mask},
				MAC: normMAC(ifi.HardwareAddr.String()), Iface: ifi,
			})
		}
	}
	return out
}

// maxHostsPerNet 限制单个网段的扫描规模；更大的网段收缩为本机所在的 /24。
const maxPrefix = 22

// clampNet 把过大的网段收缩为以 ip 为中心的 /24。
func clampNet(n *net.IPNet, ip net.IP) (*net.IPNet, bool) {
	ones, bits := n.Mask.Size()
	if bits != 32 || ones >= maxPrefix {
		return n, false
	}
	m := net.CIDRMask(24, 32)
	return &net.IPNet{IP: ip.Mask(m), Mask: m}, true
}

// hostsOf 列出网段内所有可用主机地址（去掉网络地址与广播地址）。
func hostsOf(n *net.IPNet) []net.IP {
	ones, bits := n.Mask.Size()
	if bits != 32 {
		return nil
	}
	base := ipToU32(n.IP.To4())
	size := uint32(1) << (32 - ones)
	var out []net.IP
	for i := uint32(1); i+1 < size || (size <= 2 && i < size); i++ {
		out = append(out, u32ToIP(base+i))
	}
	return out
}

func ipToU32(ip net.IP) uint32 {
	ip = ip.To4()
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}

func u32ToIP(v uint32) net.IP {
	return net.IPv4(byte(v>>24), byte(v>>16), byte(v>>8), byte(v)).To4()
}

// lowResource 判断是否为小内存设备（路由器），用于调低默认并发。
func LowResource() bool {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if strings.HasPrefix(sc.Text(), "MemTotal:") {
			var kb int
			fields := strings.Fields(sc.Text())
			if len(fields) >= 2 {
				for _, ch := range fields[1] {
					kb = kb*10 + int(ch-'0')
				}
			}
			return kb > 0 && kb < 768*1024
		}
	}
	return false
}
