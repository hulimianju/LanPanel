package discovery

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"lanpanel/internal/rules"
)

// ---- 观测收集器 ----

type collector struct {
	mu   sync.Mutex
	obs  map[string]*hostObs
	logf func(kind, target, msg string)
}

func (c *collector) with(ip string, fn func(o *hostObs)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	o, ok := c.obs[ip]
	if !ok {
		o = newObs(ip)
		c.obs[ip] = o
	}
	fn(o)
}

func (c *collector) log(kind, target, format string, args ...any) {
	c.logf(kind, target, fmt.Sprintf(format, args...))
}

// ---- 进度与日志 ----

type Phase struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	Desc   string `json:"desc"`
	Status string `json:"status"` // wait | run | done | skip
	Done   int    `json:"done"`
	Total  int    `json:"total"`
	Detail string `json:"detail"`
}

type Status struct {
	Running   bool      `json:"running"`
	Mode      string    `json:"mode,omitempty"`
	Trigger   string    `json:"trigger,omitempty"`
	Subnets   []string  `json:"subnets,omitempty"`
	Start     time.Time `json:"start,omitempty"`
	Phases    []Phase   `json:"phases,omitempty"`
	Percent   float64   `json:"percent"`
	Found     int       `json:"found"`
	NextFull  time.Time `json:"nextFull,omitempty"`
	NextQuick time.Time `json:"nextQuick,omitempty"`
	Privilege string    `json:"privilege"`         // 主机发现方式说明
	Warning   string    `json:"warning,omitempty"` // 运行环境问题（如容器使用 bridge 网络）
}

type LogLine struct {
	Seq    int       `json:"seq"`
	Time   time.Time `json:"time"`
	Kind   string    `json:"kind"`
	Target string    `json:"target"`
	Msg    string    `json:"msg"`
}

const maxLogs = 400

// Scanner 负责执行扫描与定时调度。
type Scanner struct {
	store  *DeviceStore
	config func() Config
	rules  func() []rules.Rule
	hooks  Hooks

	mu        sync.Mutex
	status    Status
	cancel    context.CancelFunc
	logs      []LogLine
	seq       int
	weights   map[string]float64
	lastFull  time.Time
	lastQuick time.Time
}

func NewScanner(store *DeviceStore, config func() Config, ruleList func() []rules.Rule, hooks Hooks) *Scanner {
	return &Scanner{store: store, config: config, rules: ruleList, hooks: hooks}
}

func (s *Scanner) Store() *DeviceStore { return s.store }

func (s *Scanner) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.status
	st.Phases = append([]Phase(nil), s.status.Phases...)
	st.Warning = EnvWarning()
	cfg := s.config()
	if cfg.FullInterval > 0 {
		st.NextFull = s.lastFull.Add(time.Duration(cfg.FullInterval) * time.Minute)
	}
	if cfg.QuickInterval > 0 {
		st.NextQuick = s.lastQuick.Add(time.Duration(cfg.QuickInterval) * time.Minute)
	}
	return st
}

// Logs 返回序号大于 since 的日志。
func (s *Scanner) Logs(since int) []LogLine {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []LogLine{}
	for _, l := range s.logs {
		if l.Seq > since {
			out = append(out, l)
		}
	}
	return out
}

func (s *Scanner) addLog(kind, target, msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	s.logs = append(s.logs, LogLine{Seq: s.seq, Time: time.Now(), Kind: kind, Target: target, Msg: msg})
	if len(s.logs) > maxLogs {
		s.logs = s.logs[len(s.logs)-maxLogs:]
	}
}

func (s *Scanner) setPhase(key, status string, done, total int, detail string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.status.Phases {
		p := &s.status.Phases[i]
		if p.Key != key {
			continue
		}
		if status != "" {
			p.Status = status
		}
		if total >= 0 {
			p.Done, p.Total = done, total
		}
		if detail != "" {
			p.Detail = detail
		}
	}
	// 按权重计算总进度
	var pct float64
	for _, p := range s.status.Phases {
		w := s.weights[p.Key]
		switch {
		case p.Status == "done" || p.Status == "skip":
			pct += w
		case p.Status == "run" && p.Total > 0:
			pct += w * float64(p.Done) / float64(p.Total)
		}
	}
	s.status.Percent = min(pct, 100)
}

func (s *Scanner) incPhase(key string) {
	s.mu.Lock()
	for i := range s.status.Phases {
		if s.status.Phases[i].Key == key {
			s.status.Phases[i].Done++
		}
	}
	s.mu.Unlock()
	s.setPhase(key, "", -1, -1, "")
}

// Cancel 取消正在进行的扫描。
func (s *Scanner) Cancel() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
		return true
	}
	return false
}

var ErrBusy = errors.New("已有扫描正在进行")

// Start 启动一次扫描（异步）。mode：full | quick。
func (s *Scanner) Start(mode, trigger string) error {
	s.mu.Lock()
	if s.status.Running {
		s.mu.Unlock()
		return ErrBusy
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	s.cancel = cancel
	phases := []Phase{
		{Key: "hosts", Label: "主机发现", Desc: "ARP 扫描 + ICMP", Status: "wait"},
		{Key: "broadcast", Label: "协议广播", Desc: "mDNS · SSDP · WSD · NetBIOS · SNMP", Status: "wait"},
		{Key: "ports", Label: "端口扫描", Desc: "规则端口 × 在线主机", Status: "wait"},
		{Key: "fingerprint", Label: "指纹识别", Desc: "页面标题 / 响应头 / 欢迎信息", Status: "wait"},
	}
	s.weights = map[string]float64{"hosts": 15, "broadcast": 15, "ports": 50, "fingerprint": 20}
	if mode == "quick" {
		phases[2].Status, phases[3].Status = "skip", "skip"
		s.weights = map[string]float64{"hosts": 50, "broadcast": 50}
	}
	s.status = Status{Running: true, Mode: mode, Trigger: trigger, Start: time.Now(), Phases: phases}
	s.mu.Unlock()

	go func() {
		defer cancel()
		sum := s.run(ctx, mode, trigger)
		s.mu.Lock()
		s.status.Running = false
		s.cancel = nil
		if mode == "full" {
			s.lastFull, s.lastQuick = sum.End, sum.End
		} else {
			s.lastQuick = sum.End
		}
		s.mu.Unlock()
	}()
	return nil
}

// buildTargets 汇总要扫描的网段。
func (s *Scanner) buildTargets(cfg Config, ifaces []localIface) []target {
	var out []target
	seen := map[string]bool{}
	add := func(n *net.IPNet, ifi *localIface) {
		k := n.String()
		if seen[k] {
			return
		}
		seen[k] = true
		out = append(out, target{Net: n, Iface: ifi, Hosts: hostsOf(n)})
	}
	if w := EnvWarning(); w != "" && cfg.AutoSubnets {
		// 容器内部网段不是用户的局域网，扫描它只会得到误导性的结果
		s.addLog("WARN", "-", w)
		cfg.AutoSubnets = false
	}
	if cfg.AutoSubnets {
		for i := range ifaces {
			n, clamped := clampNet(ifaces[i].Net, ifaces[i].IP)
			if clamped {
				s.addLog("WARN", ifaces[i].Net.String(), "网段过大，只扫描本机所在的 "+n.String())
			}
			add(n, &ifaces[i])
		}
	}
	for _, c := range cfg.Subnets {
		_, n, err := net.ParseCIDR(strings.TrimSpace(c))
		if err != nil || n.IP.To4() == nil {
			continue
		}
		if ones, _ := n.Mask.Size(); ones < maxPrefix {
			s.addLog("WARN", c, fmt.Sprintf("网段大于 /%d，已跳过", maxPrefix))
			continue
		}
		var ifi *localIface
		for i := range ifaces {
			if n.Contains(ifaces[i].IP) {
				ifi = &ifaces[i]
			}
		}
		add(n, ifi)
	}
	return out
}

func (s *Scanner) run(ctx context.Context, mode, trigger string) ScanSummary {
	start := time.Now()
	cfg := s.config()
	ruleList := s.rules()
	ifaces := localIfaces()
	targets := s.buildTargets(cfg, ifaces)
	sum := ScanSummary{Mode: mode, Trigger: trigger, Start: start}
	for _, t := range targets {
		sum.Subnets = append(sum.Subnets, t.Net.String())
	}
	s.mu.Lock()
	s.status.Subnets = sum.Subnets
	s.mu.Unlock()
	modeName := map[string]string{"full": "完整扫描", "quick": "快速扫描"}[mode]
	s.addLog("INFO", strings.Join(sum.Subnets, ", "), "开始"+modeName)
	if len(targets) == 0 {
		sum.Error = "没有可扫描的网段（未找到私有 IPv4 网卡，也未配置网段）"
		s.addLog("WARN", "-", sum.Error)
		sum.End = time.Now()
		return s.store.addScan(sum)
	}

	col := &collector{obs: map[string]*hostObs{}, logf: s.addLog}

	// 协议广播与主机发现并行进行：广播需要等待应答，正好覆盖 ARP 扫描的时间
	var bwg sync.WaitGroup
	bctx, bcancel := context.WithTimeout(ctx, 20*time.Second)
	defer bcancel()
	s.setPhase("broadcast", "run", 0, 0, "")
	var connected []localIface
	for _, t := range targets {
		if t.Iface != nil {
			connected = append(connected, *t.Iface)
		}
	}
	runB := func(enabled bool, fn func()) {
		if !enabled || len(connected) == 0 {
			return
		}
		bwg.Add(1)
		go func() { defer bwg.Done(); fn() }()
	}
	runB(cfg.Protocols.MDNS, func() { discoverMDNS(bctx, connected, col) })
	runB(cfg.Protocols.SSDP, func() { discoverSSDP(bctx, connected, col) })
	runB(cfg.Protocols.WSD, func() { discoverWSD(bctx, connected, col) })

	// ---- 主机发现 ----
	s.setPhase("hosts", "run", 0, len(targets), "")
	alive := map[string]string{} // IP → MAC（可能为空）
	privilege := ""
	for _, t := range targets {
		if ctx.Err() != nil {
			break
		}
		if t.Iface != nil {
			arp, err := rawARPScan(ctx, *t.Iface, t.Hosts, 1500*time.Millisecond)
			if err != nil {
				arp = arpByTrigger(ctx, t.Hosts)
				privilege = "邻居表（无原始套接字权限，离线判断可能滞后）"
			} else if privilege == "" {
				privilege = "原始 ARP"
			}
			for ip, mac := range arp {
				alive[ip] = mac
				col.log("ARP", ip, "%s", mac)
			}
		} else {
			for ip := range tcpAlive(ctx, t.Hosts, cfg.Concurrency, time.Duration(cfg.TimeoutMs)*time.Millisecond) {
				if _, ok := alive[ip]; !ok {
					alive[ip] = ""
				}
				col.log("TCP", ip, "主机在线（非直连网段，无法获取 MAC）")
			}
		}
		if cfg.Protocols.ICMP {
			pings, pmode := pingSweep(ctx, t.Hosts, 1200*time.Millisecond)
			n := 0
			for ip := range pings {
				if _, ok := alive[ip]; !ok {
					alive[ip] = ""
					n++
				}
			}
			if n > 0 || strings.HasPrefix(pmode, "不可用") {
				s.addLog("ICMP", t.Net.String(), fmt.Sprintf("%s，新增 %d 台", pmode, n))
			}
		}
		s.incPhase("hosts")
	}
	s.mu.Lock()
	s.status.Privilege = privilege
	s.mu.Unlock()

	// 本机与网关
	gw := defaultGateway()
	for _, ifi := range ifaces {
		ip := ifi.IP.String()
		alive[ip] = ifi.MAC
		col.with(ip, func(o *hostObs) {
			o.Self = true
			if h, err := osHostname(); err == nil {
				o.Hostnames["DHCP"] = h
			}
		})
	}
	for ip, mac := range alive {
		col.with(ip, func(o *hostObs) {
			if mac != "" {
				o.MAC = mac
				o.Sources["ARP"] = true
				o.raw("ARP", ip+" → "+mac)
			} else {
				o.Sources["ICMP"] = true
			}
			if ip == gw {
				o.Gateway = true
				o.hint("router", 12)
			}
		})
	}
	aliveIPs := make([]string, 0, len(alive))
	for ip := range alive {
		aliveIPs = append(aliveIPs, ip)
	}
	sort.Slice(aliveIPs, func(i, j int) bool { return ipLess(aliveIPs[i], aliveIPs[j]) })
	s.mu.Lock()
	s.status.Found = len(aliveIPs)
	s.mu.Unlock()
	s.setPhase("hosts", "done", len(targets), len(targets), fmt.Sprintf("%d 台在线", len(aliveIPs)))
	s.addLog("INFO", "-", fmt.Sprintf("主机发现完成：%d 台在线（%s）", len(aliveIPs), privilege))

	// 依赖在线列表的协议
	if n := readLeases(col); n > 0 {
		s.addLog("DHCP", "-", fmt.Sprintf("读取 DHCP 租约 %d 条", n))
	}
	runB(cfg.Protocols.NetBIOS, func() { discoverNetBIOS(bctx, aliveIPs, col) })
	runB(cfg.Protocols.SNMP, func() { discoverSNMP(bctx, aliveIPs, cfg.SNMPCommunity, col) })
	if cfg.Protocols.DNS {
		bwg.Add(1)
		go func() { defer bwg.Done(); reverseDNS(bctx, aliveIPs, col) }()
	}

	// ---- 端口扫描与指纹识别（与广播收尾并行）----
	if mode == "full" && ctx.Err() == nil {
		ports := rules.Ports(ruleList)
		for _, p := range cfg.ExtraPorts {
			if p > 0 && p < 65536 {
				ports = append(ports, p)
			}
		}
		ports = dedupInts(ports)
		total := len(aliveIPs) * len(ports)
		s.setPhase("ports", "run", 0, total, fmt.Sprintf("%d 个端口 × %d 台", len(ports), len(aliveIPs)))
		open := portScan(ctx, aliveIPs, ports, cfg.Concurrency, time.Duration(cfg.TimeoutMs)*time.Millisecond, func() { s.incPhase("ports") })
		nOpen := 0
		for ip, ps := range open {
			nOpen += len(ps)
			sort.Ints(ps)
			col.log("PORT", ip, "开放端口 %s", joinInts(ps))
		}
		s.setPhase("ports", "done", total, total, fmt.Sprintf("%d 个开放端口", nOpen))

		matcher := rules.NewMatcher(ruleList)
		pr := newProber(matcher, time.Duration(cfg.TimeoutMs)*time.Millisecond)
		needFav := matcher.NeedFavicon()
		s.setPhase("fingerprint", "run", 0, nOpen, "")
		sem := make(chan struct{}, 24)
		var fwg sync.WaitGroup
		for ip, ps := range open {
			for _, p := range ps {
				if ctx.Err() != nil {
					break
				}
				fwg.Add(1)
				sem <- struct{}{}
				go func(ip string, p int) {
					defer fwg.Done()
					defer func() { <-sem }()
					sv, rule, ok := pr.identify(ctx, ip, p, needFav)
					col.with(ip, func(o *hostObs) {
						o.addService(sv)
						if ok && rule.Device != "" {
							w := 10
							if rule.System {
								w = 12
							}
							o.hint(rule.Device, w)
						}
						if ok && rule.System && o.OS == "" {
							o.OS = rule.Name
						}
						detail := sv.Name
						if sv.Title != "" && sv.Title != sv.Name {
							detail += `  title="` + sv.Title + `"`
						}
						if sv.Banner != "" {
							detail += "  " + sv.Banner
						}
						o.raw(strings.ToUpper(sv.Scheme), fmt.Sprintf(":%d  %s", p, detail))
					})
					kind := "HTTP"
					if sv.Scheme == "tcp" {
						kind = "TCP"
					}
					msg := sv.Name
					if ok && rule.ID != "web-generic" {
						msg += "  → 规则 " + rule.ID
					}
					col.log(kind, fmt.Sprintf("%s:%d", ip, p), "%s", msg)
					s.incPhase("fingerprint")
				}(ip, p)
			}
		}
		fwg.Wait()
		if ctx.Err() == nil {
			col.mu.Lock()
			for _, ip := range aliveIPs {
				if o, ok := col.obs[ip]; ok {
					o.PortScanned = true
				}
			}
			col.mu.Unlock()
		}
		s.setPhase("fingerprint", "done", nOpen, nOpen, "")
	}

	bwg.Wait()
	s.setPhase("broadcast", "done", 0, 0, "")

	// 厂商线索、只保留在线主机的观测（广播可能来自其他网段的设备）
	col.mu.Lock()
	obs := map[string]*hostObs{}
	for ip, o := range col.obs {
		if _, ok := alive[ip]; !ok {
			// 广播发现但 ARP 未发现：多为跨网段设备，同样记录
			if !o.Sources["mDNS"] && !o.Sources["SSDP"] && !o.Sources["WSD"] {
				continue
			}
		}
		if mac := normMAC(o.MAC); mac != "" {
			vendorHint(o, ouiLookup(mac))
		}
		if h := o.bestHostname(); h != "" {
			hostnameHint(o, h)
		}
		obs[ip] = o
	}
	col.mu.Unlock()

	sum.End = time.Now()
	if ctx.Err() != nil {
		sum.Canceled = true
		s.addLog("WARN", "-", "扫描已取消，已发现的结果仍会保存")
	}
	var scanned []*net.IPNet
	if !sum.Canceled {
		for _, t := range targets {
			scanned = append(scanned, t.Net)
		}
	}
	res := s.store.merge(obs, scanned, mode == "full" && !sum.Canceled, sum.End, s.hooks.Watched)
	sum.Online, sum.New, sum.IPChanged, sum.Services = res.Online, res.New, res.IPChanged, res.Services
	events := res.Events
	if s.hooks.OnScan != nil {
		events = s.hooks.OnScan(sum, events)
	}
	for _, e := range events {
		msg := map[string]string{"new": "新设备", "ip_changed": "IP 变化：" + e.OldIP + " →", "offline": "设备离线", "online": "设备恢复在线"}[e.Type]
		detail := strings.TrimSpace(fmt.Sprintf("%s %s %s", msg, e.IP, e.Detail))
		s.addLog("EVENT", e.Name, detail)
	}
	s.store.addEvents(events)
	s.addLog("INFO", "-", fmt.Sprintf("%s完成：%d 台在线，新设备 %d 台，IP 变化 %d 台，服务 %d 个，耗时 %s",
		modeName, sum.Online, sum.New, sum.IPChanged, sum.Services, sum.End.Sub(start).Round(100*time.Millisecond)))
	log.Printf("%s完成：%d 台在线，新设备 %d，耗时 %s", modeName, sum.Online, sum.New, sum.End.Sub(start).Round(time.Second))
	return s.store.addScan(sum)
}

func dedupInts(in []int) []int {
	sort.Ints(in)
	out := in[:0]
	for i, v := range in {
		if i == 0 || v != in[i-1] {
			out = append(out, v)
		}
	}
	return out
}

func joinInts(v []int) string {
	s := make([]string, len(v))
	for i, n := range v {
		s[i] = itoa(n)
	}
	return strings.Join(s, ", ")
}

// RunScheduler 按配置定时执行扫描，直到 ctx 结束。
func (s *Scanner) RunScheduler(ctx context.Context) {
	cfg := s.config()
	if cfg.ScanOnStart {
		time.AfterFunc(3*time.Second, func() { _ = s.Start("full", "startup") })
	} else {
		now := time.Now()
		s.mu.Lock()
		s.lastFull, s.lastQuick = now, now
		s.mu.Unlock()
	}
	t := time.NewTicker(20 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			s.Cancel()
			return
		case <-t.C:
		}
		cfg := s.config()
		s.mu.Lock()
		running, lastFull, lastQuick := s.status.Running, s.lastFull, s.lastQuick
		s.mu.Unlock()
		if running {
			continue
		}
		now := time.Now()
		switch {
		case cfg.FullInterval > 0 && now.Sub(lastFull) >= time.Duration(cfg.FullInterval)*time.Minute:
			_ = s.Start("full", "schedule")
		case cfg.QuickInterval > 0 && now.Sub(lastQuick) >= time.Duration(cfg.QuickInterval)*time.Minute:
			_ = s.Start("quick", "schedule")
		}
	}
}
