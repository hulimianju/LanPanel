package discovery

import (
	"context"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/dns/dnsmessage"
	"golang.org/x/net/ipv4"
)

// mDNS / DNS-SD 发现。
//
// 采用"传统单播"查询（RFC 6762 §6.7）：从随机端口向 224.0.0.251:5353 发送查询，
// 响应方会把应答单播回来。这样无需占用 5353 端口，不会与系统里的 avahi / mDNSResponder 冲突。

// 常见服务类型，与 _services._dns-sd._udp 枚举结果合并查询
var mdnsTypes = []string{
	"_http._tcp", "_https._tcp", "_smb._tcp", "_afpovertcp._tcp", "_nfs._tcp", "_webdav._tcp", "_ftp._tcp",
	"_ssh._tcp", "_sftp-ssh._tcp", "_rfb._tcp", "_workstation._tcp", "_device-info._tcp", "_adisk._tcp",
	"_airplay._tcp", "_raop._tcp", "_companion-link._tcp", "_googlecast._tcp", "_spotify-connect._tcp",
	"_ipp._tcp", "_ipps._tcp", "_printer._tcp", "_pdl-datastream._tcp", "_scanner._tcp", "_uscan._tcp",
	"_hap._tcp", "_homekit._tcp", "_home-assistant._tcp", "_esphomelib._tcp", "_hue._tcp", "_matter._tcp",
	"_miio._udp", "_qdiscover._tcp", "_dlna._tcp", "_daap._tcp", "_mqtt._tcp", "_trim._tcp", "_nut._tcp",
}

type mdnsRecord struct {
	instances map[string]*mdnsInstance // 实例全名 → 信息
	hosts     map[string][]string      // 主机名（xxx.local.）→ IPv4
}

type mdnsInstance struct {
	Name    string // 实例全名（小写），如 "fnos-home._http._tcp.local."
	Display string // 原始大小写的实例全名
	Type    string // "_http._tcp"
	Target  string // SRV 目标主机
	Port    int
	TXT     map[string]string
}

// instNamed 按原始名称取实例：以小写为键，保留首次出现的原始大小写用于显示。
func (r *mdnsRecord) instNamed(orig string) *mdnsInstance {
	i := r.inst(strings.ToLower(orig))
	if i.Display == "" {
		i.Display = orig
	}
	return i
}

func (r *mdnsRecord) inst(name string) *mdnsInstance {
	if i, ok := r.instances[name]; ok {
		return i
	}
	i := &mdnsInstance{Name: name, TXT: map[string]string{}}
	// 实例名形如 "<实例>.<_服务>.<_协议>.local."
	parts := strings.Split(strings.TrimSuffix(name, "."), ".")
	for k := 0; k+1 < len(parts); k++ {
		if strings.HasPrefix(parts[k], "_") && strings.HasPrefix(parts[k+1], "_") {
			i.Type = parts[k] + "." + parts[k+1]
			break
		}
	}
	r.instances[name] = i
	return i
}

func buildQuery(names []string, qtype dnsmessage.Type) ([]byte, error) {
	b := dnsmessage.NewBuilder(nil, dnsmessage.Header{})
	b.EnableCompression()
	if err := b.StartQuestions(); err != nil {
		return nil, err
	}
	for _, n := range names {
		name, err := dnsmessage.NewName(n)
		if err != nil {
			continue
		}
		if err := b.Question(dnsmessage.Question{Name: name, Type: qtype, Class: dnsmessage.ClassINET}); err != nil {
			return nil, err
		}
	}
	return b.Finish()
}

// mdnsConn 在每块网卡上各开一个套接字，保证多网卡时查询从正确的网卡发出。
type mdnsConn struct {
	conns []*net.UDPConn
}

func openMDNS(ifaces []localIface) *mdnsConn {
	mc := &mdnsConn{}
	for _, ifi := range ifaces {
		c, err := net.ListenUDP("udp4", &net.UDPAddr{IP: ifi.IP})
		if err != nil {
			continue
		}
		pc := ipv4.NewPacketConn(c)
		_ = pc.SetMulticastInterface(&ifi.Iface)
		_ = pc.SetMulticastTTL(255)
		mc.conns = append(mc.conns, c)
	}
	return mc
}

func (mc *mdnsConn) send(pkt []byte) {
	dst := &net.UDPAddr{IP: net.IPv4(224, 0, 0, 251), Port: 5353}
	for _, c := range mc.conns {
		_, _ = c.WriteToUDP(pkt, dst)
	}
}

func (mc *mdnsConn) close() {
	for _, c := range mc.conns {
		c.Close()
	}
}

// receive 在截止时间前持续收包并解析。
func (mc *mdnsConn) receive(deadline time.Time, rec *mdnsRecord, mu *sync.Mutex, onPacket func(src net.IP)) {
	var wg sync.WaitGroup
	for _, c := range mc.conns {
		wg.Add(1)
		go func(c *net.UDPConn) {
			defer wg.Done()
			buf := make([]byte, 9000)
			_ = c.SetReadDeadline(deadline)
			for {
				n, src, err := c.ReadFromUDP(buf)
				if err != nil {
					return
				}
				mu.Lock()
				parseMDNS(buf[:n], rec)
				mu.Unlock()
				if onPacket != nil {
					onPacket(src.IP)
				}
			}
		}(c)
	}
	wg.Wait()
}

func parseMDNS(pkt []byte, rec *mdnsRecord) {
	var p dnsmessage.Parser
	if _, err := p.Start(pkt); err != nil {
		return
	}
	if err := p.SkipAllQuestions(); err != nil {
		return
	}
	// 应答段
	for {
		h, err := p.AnswerHeader()
		if err != nil {
			break
		}
		if !parseRR(&p, h, rec) {
			if p.SkipAnswer() != nil {
				return
			}
		}
	}
	if err := p.SkipAllAuthorities(); err != nil {
		return
	}
	// 附加段：SRV / TXT / A 常放在这里
	for {
		h, err := p.AdditionalHeader()
		if err != nil {
			return
		}
		if !parseRR(&p, h, rec) {
			if p.SkipAdditional() != nil {
				return
			}
		}
	}
}

// parseRR 解析一条记录；返回 false 表示未消费记录体，调用方需按所在段跳过。
func parseRR(p *dnsmessage.Parser, h dnsmessage.ResourceHeader, rec *mdnsRecord) bool {
	orig := h.Name.String()
	name := strings.ToLower(orig)
	switch h.Type {
	case dnsmessage.TypePTR:
		r, err := p.PTRResource()
		if err != nil {
			return true
		}
		target := r.PTR.String()
		if name == "_services._dns-sd._udp.local." {
			rec.inst("_enum." + strings.ToLower(target)) // 记录发现的服务类型，稍后再查询
			return true
		}
		rec.instNamed(target)
	case dnsmessage.TypeSRV:
		r, err := p.SRVResource()
		if err != nil {
			return true
		}
		i := rec.instNamed(orig)
		i.Target, i.Port = strings.ToLower(r.Target.String()), int(r.Port)
	case dnsmessage.TypeTXT:
		r, err := p.TXTResource()
		if err != nil {
			return true
		}
		i := rec.instNamed(orig)
		for _, t := range r.TXT {
			k, v, _ := strings.Cut(t, "=")
			if k != "" {
				i.TXT[strings.ToLower(k)] = v
			}
		}
	case dnsmessage.TypeA:
		r, err := p.AResource()
		if err != nil {
			return true
		}
		ip := net.IP(r.A[:]).String()
		for _, x := range rec.hosts[name] {
			if x == ip {
				return true
			}
		}
		rec.hosts[name] = append(rec.hosts[name], ip)
	default:
		return false
	}
	return true
}

// discoverMDNS 执行三轮查询：常见类型 + 服务枚举 → 枚举出的新类型 → Apple 设备信息。
func discoverMDNS(ctx context.Context, ifaces []localIface, col *collector) {
	mc := openMDNS(ifaces)
	defer mc.close()
	if len(mc.conns) == 0 {
		col.log("mDNS", "-", "无法创建套接字，跳过")
		return
	}
	rec := &mdnsRecord{instances: map[string]*mdnsInstance{}, hosts: map[string][]string{}}
	var mu sync.Mutex
	responders := map[string]bool{}
	onPacket := func(src net.IP) {
		mu.Lock()
		responders[src.String()] = true
		mu.Unlock()
	}

	round := func(names []string, qtype dnsmessage.Type, wait time.Duration) {
		// 分批发送，避免单个包过大
		for i := 0; i < len(names); i += 12 {
			if pkt, err := buildQuery(names[i:min(i+12, len(names))], qtype); err == nil {
				mc.send(pkt)
			}
		}
		deadline := time.Now().Add(wait)
		if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
			deadline = d
		}
		mc.receive(deadline, rec, &mu, onPacket)
	}

	first := []string{"_services._dns-sd._udp.local."}
	for _, t := range mdnsTypes {
		first = append(first, t+".local.")
	}
	round(first, dnsmessage.TypePTR, 2200*time.Millisecond)
	if ctx.Err() != nil {
		return
	}

	// 第二轮：枚举出来但不在常见列表里的类型
	known := map[string]bool{}
	for _, t := range mdnsTypes {
		known[t+".local."] = true
	}
	var extra []string
	mu.Lock()
	for name := range rec.instances {
		if t, ok := strings.CutPrefix(name, "_enum."); ok {
			if !known[t] {
				extra = append(extra, t)
			}
			delete(rec.instances, name)
		}
	}
	// 第三轮顺带查询：只有 PTR 没有 SRV 的实例、以及 Apple 设备的 _device-info
	var needSRV []string
	devInfo := map[string]bool{}
	for name, i := range rec.instances {
		if i.Target == "" {
			needSRV = append(needSRV, name)
		}
		disp := i.Display
		if disp == "" {
			disp = name
		}
		if base, _, ok := strings.Cut(disp, "._"); ok && (i.Type == "_companion-link._tcp" || i.Type == "_airplay._tcp" || i.Type == "_raop._tcp" || i.Type == "_smb._tcp") {
			// RAOP 实例名形如 "AABBCCDDEEFF@客厅"，设备名在 @ 之后
			base = base[strings.Index(base, "@")+1:]
			devInfo[base+"._device-info._tcp.local."] = true
		}
	}
	mu.Unlock()
	if len(extra) > 0 {
		sort.Strings(extra)
		round(extra, dnsmessage.TypePTR, 1200*time.Millisecond)
	}
	if len(needSRV) > 0 && ctx.Err() == nil {
		round(needSRV, dnsmessage.TypeSRV, 800*time.Millisecond)
	}
	if len(devInfo) > 0 && ctx.Err() == nil {
		round(sortedKeys(devInfo), dnsmessage.TypeTXT, 800*time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()
	applyMDNS(rec, col)
	col.log("mDNS", "-", "收到 %d 台设备的应答，%d 个服务实例", len(responders), len(rec.instances))
}

// mdnsServiceNames 把服务类型映射为易读名称与图标。
var mdnsServiceNames = map[string][3]string{
	"_http._tcp":            {"网页", "globe", "blue"},
	"_https._tcp":           {"网页", "globe", "blue"},
	"_smb._tcp":             {"SMB 共享", "folder-open", "slate"},
	"_afpovertcp._tcp":      {"AFP 共享", "folder-open", "slate"},
	"_nfs._tcp":             {"NFS", "folder-open", "slate"},
	"_webdav._tcp":          {"WebDAV", "folder", "slate"},
	"_ftp._tcp":             {"FTP", "folder", "slate"},
	"_ssh._tcp":             {"SSH", "terminal", "slate"},
	"_sftp-ssh._tcp":        {"SFTP", "terminal", "slate"},
	"_rfb._tcp":             {"VNC / 屏幕共享", "monitor", "slate"},
	"_airplay._tcp":         {"AirPlay", "cast", "slate"},
	"_raop._tcp":            {"AirPlay 音频", "speaker", "slate"},
	"_googlecast._tcp":      {"Chromecast", "cast", "slate"},
	"_spotify-connect._tcp": {"Spotify Connect", "music", "green"},
	"_ipp._tcp":             {"IPP 打印", "printer", "slate"},
	"_ipps._tcp":            {"IPP 打印", "printer", "slate"},
	"_printer._tcp":         {"LPD 打印", "printer", "slate"},
	"_pdl-datastream._tcp":  {"RAW 打印", "printer", "slate"},
	"_uscan._tcp":           {"扫描仪", "scan-search", "slate"},
	"_hap._tcp":             {"HomeKit", "house", "amber"},
	"_home-assistant._tcp":  {"Home Assistant", "house", "teal"},
	"_esphomelib._tcp":      {"ESPHome", "cpu", "slate"},
	"_adisk._tcp":           {"时间机器", "archive", "slate"},
	"_mqtt._tcp":            {"MQTT", "message", "green"},
	"_daap._tcp":            {"iTunes 共享", "music", "rose"},
	"_trim._tcp":            {"飞牛 fnOS", "hard-drive", "blue"},
	"_nut._tcp":             {"NUT 不间断电源", "zap", "amber"},
}

func applyMDNS(rec *mdnsRecord, col *collector) {
	for _, in := range rec.instances {
		ips := rec.hosts[in.Target]
		if len(ips) == 0 {
			continue
		}
		host := strings.TrimSuffix(strings.TrimSuffix(in.Target, "."), ".local")
		instName := in.Display
		if instName == "" {
			instName = in.Name
		}
		if i := strings.Index(instName, "._"); i > 0 {
			instName = instName[:i]
		}
		instName = unescapeDNS(instName)
		for _, ip := range ips {
			col.with(ip, func(o *hostObs) {
				o.Sources["mDNS"] = true
				if host != "" {
					o.Hostnames["mDNS"] = host
				}
				txt := formatTXT(in.TXT)
				o.raw("mDNS", strings.TrimSpace(instName+"  "+in.Type+"  port="+itoa(in.Port)+"  "+txt))
				classifyMDNS(o, in)
				if meta, ok := mdnsServiceNames[in.Type]; ok && in.Port > 0 && in.Type != "_device-info._tcp" {
					sv := Service{Port: in.Port, Proto: "tcp", Scheme: "tcp", Name: meta[0], Icon: meta[1], Color: meta[2], Source: "mdns"}
					if in.Type == "_http._tcp" || in.Type == "_https._tcp" {
						sv.Scheme = strings.TrimSuffix(strings.TrimPrefix(in.Type, "_"), "._tcp")
						path := in.TXT["path"]
						if path == "" || path[0] != '/' {
							path = "/"
						}
						sv.URL = sv.Scheme + "://" + ip + ":" + itoa(in.Port) + strings.TrimSuffix(path, "/")
						if instName != "" {
							sv.Name = instName
						}
					}
					o.addService(sv)
				}
			})
		}
		col.log("mDNS", ips[0], "%s  %s  :%d", instName, in.Type, in.Port)
	}
}

func formatTXT(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		if len(b.String()) > 160 {
			break
		}
		b.WriteString(k + "=" + m[k] + " ")
	}
	return strings.TrimSpace(b.String())
}

// unescapeDNS 还原实例名中的转义（如 "\032" 表示空格）。
func unescapeDNS(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+3 < len(s) && s[i+1] >= '0' && s[i+1] <= '9' {
			v := int(s[i+1]-'0')*100 + int(s[i+2]-'0')*10 + int(s[i+3]-'0')
			b.WriteByte(byte(v))
			i += 3
			continue
		}
		if s[i] == '\\' && i+1 < len(s) {
			i++
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
