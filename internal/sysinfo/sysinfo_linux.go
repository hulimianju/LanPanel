package sysinfo

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

func readPlatform(info *Info) (*sample, bool) {
	cur := &sample{}
	if f, err := os.Open("/proc/stat"); err == nil {
		sc := bufio.NewScanner(f)
		if sc.Scan() {
			fields := strings.Fields(sc.Text())
			for i, v := range fields[1:] {
				n, _ := strconv.ParseUint(v, 10, 64)
				if i < 8 { // user nice system idle iowait irq softirq steal
					cur.cpuTotal += n
				}
				if i == 3 || i == 4 {
					cur.cpuIdle += n
				}
			}
		}
		f.Close()
	}
	if b, err := os.ReadFile("/proc/meminfo"); err == nil {
		kv := map[string]uint64{}
		for _, line := range strings.Split(string(b), "\n") {
			k, v, ok := strings.Cut(line, ":")
			if !ok {
				continue
			}
			f := strings.Fields(v)
			if len(f) > 0 {
				n, _ := strconv.ParseUint(f[0], 10, 64)
				kv[k] = n * 1024
			}
		}
		info.MemTotal = kv["MemTotal"]
		avail, ok := kv["MemAvailable"]
		if !ok {
			avail = kv["MemFree"] + kv["Cached"] + kv["Buffers"]
		}
		info.MemUsed = info.MemTotal - avail
	}
	if b, err := os.ReadFile("/proc/uptime"); err == nil {
		if f := strings.Fields(string(b)); len(f) > 0 {
			v, _ := strconv.ParseFloat(f[0], 64)
			info.Uptime = int64(v)
		}
	}
	// 只统计默认路由所在网卡；路由器上累加所有网卡会把转发流量重复计算
	gw := defaultIface()
	if b, err := os.ReadFile("/proc/net/dev"); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			name, data, ok := strings.Cut(line, ":")
			if !ok {
				continue
			}
			name = strings.TrimSpace(name)
			if gw != "" {
				if name != gw {
					continue
				}
			} else if name == "lo" || strings.HasPrefix(name, "veth") || strings.HasPrefix(name, "docker") ||
				strings.HasPrefix(name, "br-") || strings.HasPrefix(name, "tap") || strings.HasPrefix(name, "fw") {
				continue
			}
			f := strings.Fields(data)
			if len(f) >= 9 {
				rx, _ := strconv.ParseUint(f[0], 10, 64)
				tx, _ := strconv.ParseUint(f[8], 10, 64)
				cur.netRx += rx
				cur.netTx += tx
			}
		}
	}
	return cur, cur.cpuTotal > 0
}

func defaultIface() string {
	b, err := os.ReadFile("/proc/net/route")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n")[1:] {
		f := strings.Fields(line)
		if len(f) > 2 && f[1] == "00000000" {
			return f[0]
		}
	}
	return ""
}
