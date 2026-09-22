package discovery

import (
	"encoding/binary"
	"encoding/hex"
	"net"
	"os"
	"strings"
)

// readARP 读取内核邻居表：IP → MAC（仅完整条目）。
func readARP() map[string]string {
	out := map[string]string{}
	b, err := os.ReadFile("/proc/net/arp")
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(b), "\n")[1:] {
		f := strings.Fields(line)
		// IP address  HW type  Flags  HW address  Mask  Device
		if len(f) < 6 || f[2] == "0x0" || f[3] == "00:00:00:00:00:00" {
			continue
		}
		if mac := normMAC(f[3]); mac != "" {
			out[f[0]] = mac
		}
	}
	return out
}

func defaultGateway() string {
	b, err := os.ReadFile("/proc/net/route")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n")[1:] {
		f := strings.Fields(line)
		if len(f) > 2 && f[1] == "00000000" {
			raw, err := hex.DecodeString(f[2])
			if err != nil || len(raw) != 4 {
				continue
			}
			ip := make(net.IP, 4)
			binary.LittleEndian.PutUint32(ip, binary.BigEndian.Uint32(raw))
			return ip.String()
		}
	}
	return ""
}
