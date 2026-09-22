package discovery

import (
	"strconv"
	"strings"
)

// 设备类型推断：各来源给出带权重的线索，取权重最高者。
// 权重参考：用户指定 > 默认网关(12) = 系统级规则(12) > SNMP 描述(9–11) > WSD/UPnP/mDNS(6–9) > 厂商(2–6)

func itoa(n int) string { return strconv.Itoa(n) }

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

// classifyMDNS 根据服务类型与 TXT 记录推断类型、型号。
func classifyMDNS(o *hostObs, in *mdnsInstance) {
	switch in.Type {
	case "_ipp._tcp", "_ipps._tcp", "_printer._tcp", "_pdl-datastream._tcp":
		o.hint("printer", 8)
		if m := in.TXT["ty"]; m != "" && o.Model == "" {
			o.Model = m
		}
	case "_googlecast._tcp":
		o.hint("tv", 7)
		if m := in.TXT["md"]; m != "" {
			o.Model = m
		}
		if fn := in.TXT["fn"]; fn != "" {
			o.Hostnames["mDNS"] = fn
		}
	case "_hap._tcp", "_homekit._tcp", "_esphomelib._tcp", "_hue._tcp", "_matter._tcp", "_miio._udp":
		o.hint("iot", 6)
	case "_workstation._tcp":
		o.hint("pc", 3)
	case "_home-assistant._tcp":
		o.hint("server", 4)
	case "_qdiscover._tcp":
		o.hint("nas", 8)
		o.Vendor = "QNAP"
	case "_trim._tcp": // 飞牛 fnOS 的专属服务
		o.hint("nas", 12)
		o.OS = "飞牛 fnOS"
	}
	if v := strings.ToLower(in.TXT["vendor"]); v != "" {
		switch {
		case strings.Contains(v, "synology"):
			o.hint("nas", 9)
			o.Vendor = "Synology"
		case strings.Contains(v, "qnap"):
			o.hint("nas", 9)
		}
		if m := in.TXT["model"]; m != "" && o.Model == "" {
			o.Model = m
		}
	}
	// Apple 设备：_device-info 的 model，如 MacBookPro18,3 / iPhone15,4 / AppleTV11,1
	if m := in.TXT["model"]; m != "" && (in.Type == "_device-info._tcp" || in.Type == "_airplay._tcp" || in.Type == "_raop._tcp" || in.Type == "_companion-link._tcp") {
		if t := appleType(m); t != "" {
			o.hint(t, 8)
			o.Model = m
			o.Vendor = "Apple"
		}
	}
}

func appleType(model string) string {
	m := strings.ToLower(model)
	switch {
	case strings.HasPrefix(m, "iphone"), strings.HasPrefix(m, "ipad"), strings.HasPrefix(m, "ipod"):
		return "phone"
	case strings.HasPrefix(m, "appletv"), strings.HasPrefix(m, "audioaccessory"):
		return "tv"
	case strings.HasPrefix(m, "mac"), strings.HasPrefix(m, "imac"):
		return "pc"
	case strings.HasPrefix(m, "airport"):
		return "router"
	}
	return ""
}

// classifyUPnP 根据 UPnP 设备类型推断。
func classifyUPnP(o *hostObs, d *upnpDevice) {
	t := strings.ToLower(d.DeviceType)
	switch {
	case strings.Contains(t, "internetgatewaydevice"), strings.Contains(t, "wandevice"):
		o.hint("router", 8)
	case strings.Contains(t, "wlanaccesspoint"):
		o.hint("ap", 8)
	case strings.Contains(t, "mediarenderer"):
		o.hint("tv", 6)
	case strings.Contains(t, "mediaserver"):
		o.hint("nas", 4)
	case strings.Contains(t, "printer"):
		o.hint("printer", 8)
	case strings.Contains(t, "digitalsecuritycamera"), strings.Contains(t, "camera"):
		o.hint("camera", 8)
	}
	md := strings.ToLower(d.ModelDescription + " " + d.ModelName)
	switch {
	case strings.Contains(md, "nas"), strings.Contains(md, "diskstation"):
		o.hint("nas", 7)
	case strings.Contains(md, "router"), strings.Contains(md, "路由"):
		o.hint("router", 7)
	case strings.Contains(md, "tv"), strings.Contains(md, "电视"):
		o.hint("tv", 6)
	}
}

// vendorHints 根据 MAC 厂商给出弱线索。
var vendorHints = []struct {
	kw  []string
	typ string
	w   int
}{
	{[]string{"hikvision", "dahua", "uniview", "ezviz", "imou", "reolink", "axis comm", "hanwha"}, "camera", 6},
	{[]string{"synology", "qnap", "terramaster", "asustor", "ugreen", "zspace"}, "nas", 6},
	{[]string{"espressif", "tuya", "broadlink", "yeelight", "lumi united", "sonoff", "shelly", "philips lighting", "signify"}, "iot", 6},
	{[]string{"canon", "epson", "brother", "kyocera", "lexmark", "pantum", "ricoh", "fuji xerox", "xerox"}, "printer", 5},
	{[]string{"h3c", "ruijie", "juniper", "arista", "maipu"}, "switch", 3},
	{[]string{"cisco", "huawei", "tp-link", "netgear", "d-link", "mikrotik", "ubiquiti", "tenda", "mercury", "zte", "fiberhome"}, "router", 2},
	{[]string{"vmware", "proxmox", "qemu", "xensource", "microsoft corporation"}, "server", 3}, // 虚拟机网卡
	{[]string{"raspberry"}, "server", 3},
	{[]string{"samsung", "lg electronics", "sony", "tcl", "hisense", "skyworth", "changhong", "roku"}, "tv", 2},
	{[]string{"xiaomi", "oppo", "vivo", "honor", "oneplus", "meizu", "realme", "motorola"}, "phone", 2},
	{[]string{"apple"}, "pc", 2},
	{[]string{"intel", "realtek", "micro-star", "asustek", "gigabyte", "asrock", "dell", "lenovo", "hewlett", "liteon", "azurewave", "elitegroup"}, "pc", 1},
}

// hostnameHints 根据常见的默认主机名推断类型（随机 MAC 的手机主要靠这个）。
var hostnameHints = []struct {
	kw  []string
	typ string
}{
	{[]string{"iphone", "ipad", "android", "redmi", "xiaomi-", "huawei-", "honor-", "oppo", "vivo", "oneplus", "galaxy", "pixel"}, "phone"},
	{[]string{"macbook", "imac", "mac-mini", "macmini", "desktop-", "laptop-", "-pc", "thinkpad"}, "pc"},
	{[]string{"appletv", "apple-tv", "mitv", "tv-", "-tv", "chromecast", "firetv", "shield"}, "tv"},
	{[]string{"raspberrypi", "ubuntu", "debian", "pve", "proxmox", "esxi", "server", "nuc"}, "server"},
	{[]string{"diskstation", "nas", "fnos", "truenas", "unraid"}, "nas"},
	{[]string{"printer", "epson", "canon", "brother"}, "printer"},
	{[]string{"ipcam", "ipc-", "nvr", "camera"}, "camera"},
	{[]string{"esp-", "esp32", "esp8266", "tasmota", "shelly", "tuya", "yeelight", "zigbee", "miio"}, "iot"},
	{[]string{"router", "openwrt", "xiaoqiang", "ikuai", "gateway"}, "router"},
}

func hostnameHint(o *hostObs, name string) {
	n := strings.ToLower(name)
	for _, h := range hostnameHints {
		for _, k := range h.kw {
			if strings.Contains(n, k) {
				o.hint(h.typ, 5)
				return
			}
		}
	}
}

func vendorHint(o *hostObs, vendor string) {
	v := strings.ToLower(vendor)
	for _, h := range vendorHints {
		for _, k := range h.kw {
			if strings.Contains(v, k) {
				o.hint(h.typ, h.w)
				return
			}
		}
	}
}
