package discovery

import (
	"os/exec"
	"regexp"
	"strings"
)

// macOS（开发环境）：解析 arp -an 输出。MAC 可能省略前导零，如 0:11:32:6b:4c:1。
var arpLine = regexp.MustCompile(`\((\d+\.\d+\.\d+\.\d+)\) at ([0-9a-fA-F:]+) on`)

func readARP() map[string]string {
	out := map[string]string{}
	b, err := exec.Command("arp", "-an").Output()
	if err != nil {
		return out
	}
	for _, m := range arpLine.FindAllStringSubmatch(string(b), -1) {
		parts := strings.Split(m[2], ":")
		if len(parts) != 6 {
			continue
		}
		for i, p := range parts {
			if len(p) == 1 {
				parts[i] = "0" + p
			}
		}
		if mac := normMAC(strings.Join(parts, ":")); mac != "" && mac != "ff:ff:ff:ff:ff:ff" {
			out[m[1]] = mac
		}
	}
	return out
}

func defaultGateway() string {
	b, err := exec.Command("route", "-n", "get", "default").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		if k, v, ok := strings.Cut(strings.TrimSpace(line), ":"); ok && k == "gateway" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
