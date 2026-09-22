// Package discovery 实现局域网设备发现：主机发现（ARP / ICMP / TCP）、
// 协议广播（mDNS / SSDP / NetBIOS / WS-Discovery / DHCP 租约 / 反向 DNS）、
// 端口扫描与服务指纹识别，并把结果合并进以 MAC 为身份的设备库。
package discovery

import "time"

// Device 是一台被发现的设备。以 MAC 为主键；跨网段拿不到 MAC 时用 "ip:<地址>"。
type Device struct {
	Key        string     `json:"key"`
	MAC        string     `json:"mac,omitempty"`
	IP         string     `json:"ip"`
	Hostname   string     `json:"hostname,omitempty"` // 自动发现的主机名
	Name       string     `json:"name,omitempty"`     // 用户自定义名称
	Vendor     string     `json:"vendor,omitempty"`
	Model      string     `json:"model,omitempty"`
	OS         string     `json:"os,omitempty"`       // 识别出的系统，如「飞牛 fnOS」
	Type       string     `json:"type,omitempty"`     // 自动推断的类型
	UserType   string     `json:"userType,omitempty"` // 用户指定的类型，优先
	Sources    []string   `json:"sources"`
	Services   []Service  `json:"services"`
	Randomized bool       `json:"randomized,omitempty"` // 随机（私有）MAC
	Self       bool       `json:"self,omitempty"`       // 运行 LanPanel 的本机
	Gateway    bool       `json:"gateway,omitempty"`
	Online     bool       `json:"online"`
	FirstSeen  time.Time  `json:"firstSeen"`
	LastSeen   time.Time  `json:"lastSeen"`
	IPHistory  []IPRecord `json:"ipHistory"`
	Raw        []RawEntry `json:"raw,omitempty"`
	Acked      bool       `json:"acked"`            // 新设备已被用户确认
	Missed     int        `json:"missed,omitempty"` // 连续未被扫描到的次数（≥2 判为离线，避免偶发丢包误报）
}

// DisplayName 返回设备的显示名称。
func (d *Device) DisplayName() string {
	for _, v := range []string{d.Name, d.Hostname, d.Model, d.OS, d.Vendor} {
		if v != "" {
			return v
		}
	}
	return d.IP
}

type Service struct {
	Port     int    `json:"port"`
	Proto    string `json:"proto"`  // tcp
	Scheme   string `json:"scheme"` // http | https | tcp
	Name     string `json:"name"`
	RuleID   string `json:"ruleId,omitempty"`
	Category string `json:"category,omitempty"`
	Icon     string `json:"icon,omitempty"`
	Color    string `json:"color,omitempty"`
	URL      string `json:"url,omitempty"`
	Title    string `json:"title,omitempty"`
	Server   string `json:"server,omitempty"`
	Banner   string `json:"banner,omitempty"`
	Source   string `json:"source"` // port | mdns | ssdp | wsd
}

// Web 表示服务可以在浏览器中打开。
func (s Service) Web() bool { return s.Scheme == "http" || s.Scheme == "https" }

type IPRecord struct {
	IP   string     `json:"ip"`
	From time.Time  `json:"from"`
	To   *time.Time `json:"to,omitempty"` // 为空表示当前使用中
}

// RawEntry 保存最近一次扫描中各协议的原始应答，便于排查识别结果。
type RawEntry struct {
	Source string `json:"source"`
	Text   string `json:"text"`
}

// Config 是扫描配置，保存在 data.json。
type Config struct {
	AutoSubnets   bool     `json:"autoSubnets"`   // 自动扫描本机所在网段
	Subnets       []string `json:"subnets"`       // 额外网段（CIDR）
	ExtraPorts    []int    `json:"extraPorts"`    // 规则之外额外扫描的端口
	Concurrency   int      `json:"concurrency"`   // 端口扫描并发数
	TimeoutMs     int      `json:"timeoutMs"`     // 单次连接超时
	FullInterval  int      `json:"fullInterval"`  // 完整扫描间隔（分钟），0 关闭
	QuickInterval int      `json:"quickInterval"` // 快速扫描间隔（分钟，仅主机发现与广播），0 关闭
	ScanOnStart   bool     `json:"scanOnStart"`
	Protocols     struct {
		ICMP    bool `json:"icmp"`
		MDNS    bool `json:"mdns"`
		SSDP    bool `json:"ssdp"`
		NetBIOS bool `json:"netbios"`
		WSD     bool `json:"wsd"`
		DNS     bool `json:"dns"`
		SNMP    bool `json:"snmp"`
	} `json:"protocols"`
	SNMPCommunity string `json:"snmpCommunity"`
}

func DefaultConfig(lowResource bool) Config {
	c := Config{
		AutoSubnets:   true,
		Subnets:       []string{},
		ExtraPorts:    []int{},
		Concurrency:   256,
		TimeoutMs:     800,
		FullInterval:  30,
		QuickInterval: 5,
		ScanOnStart:   true,
	}
	if lowResource { // 路由器等小内存设备降低并发
		c.Concurrency = 64
		c.TimeoutMs = 1000
	}
	p := &c.Protocols
	p.ICMP, p.MDNS, p.SSDP, p.NetBIOS, p.WSD, p.DNS, p.SNMP = true, true, true, true, true, true, true
	c.SNMPCommunity = "public"
	return c
}

// ScanSummary 记录一次扫描的结果，用于扫描历史。
type ScanSummary struct {
	ID        int       `json:"id"`
	Mode      string    `json:"mode"`    // full | quick
	Trigger   string    `json:"trigger"` // manual | schedule | startup
	Subnets   []string  `json:"subnets"`
	Start     time.Time `json:"start"`
	End       time.Time `json:"end"`
	Online    int       `json:"online"`
	New       int       `json:"new"`
	IPChanged int       `json:"ipChanged"`
	Services  int       `json:"services"`
	Canceled  bool      `json:"canceled,omitempty"`
	Error     string    `json:"error,omitempty"`
}

// Event 是一条设备动态：新设备、IP 变化，以及被关注设备（绑定了面板卡片）的上线 / 离线。
type Event struct {
	ID     int       `json:"id"`
	Time   time.Time `json:"time"`
	Type   string    `json:"type"` // new | ip_changed | offline | online
	Key    string    `json:"key"`
	MAC    string    `json:"mac,omitempty"`
	Name   string    `json:"name"`
	IP     string    `json:"ip"`
	OldIP  string    `json:"oldIp,omitempty"`
	Vendor string    `json:"vendor,omitempty"`
	Detail string    `json:"detail,omitempty"` // 由上层补充，如"已更新 2 张面板卡片"
}

// Hooks 让上层（面板）参与扫描流程。
type Hooks struct {
	// Watched 判断设备是否被关注；只为被关注的设备记录上线 / 离线动态
	Watched func(mac string) bool
	// OnScan 在每次扫描合并完成后调用，可修改事件的 Detail 后再保存
	OnScan func(sum ScanSummary, events []Event) []Event
}
