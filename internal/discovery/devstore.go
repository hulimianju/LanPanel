package discovery

import (
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"lanpanel/internal/oui"
)

const (
	maxHistory    = 30 // 保留的扫描历史条数
	maxIPHistory  = 20 // 每台设备保留的 IP 变更记录
	maxRawEntries = 40
)

type devFile struct {
	Devices []*Device     `json:"devices"`
	Scans   []ScanSummary `json:"scans"`
	NextID  int           `json:"nextId"`
}

// DeviceStore 持久化设备库到 devices.json（与面板配置分开，扫描时频繁写入）。
type DeviceStore struct {
	mu   sync.RWMutex
	path string
	data devFile
	byK  map[string]*Device
}

func OpenDeviceStore(dir string) (*DeviceStore, error) {
	s := &DeviceStore{path: filepath.Join(dir, "devices.json"), byK: map[string]*Device{}}
	raw, err := os.ReadFile(s.path)
	if err == nil {
		if err := json.Unmarshal(raw, &s.data); err != nil {
			return nil, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	for _, d := range s.data.Devices {
		s.byK[d.Key] = d
	}
	if s.data.NextID == 0 {
		s.data.NextID = 1
	}
	return s, nil
}

func (s *DeviceStore) saveLocked() error {
	raw, err := json.Marshal(s.data)
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func clone(d *Device) Device {
	c := *d
	c.Sources = append([]string{}, d.Sources...)
	c.Services = append([]Service{}, d.Services...)
	c.IPHistory = append([]IPRecord{}, d.IPHistory...)
	c.Raw = append([]RawEntry{}, d.Raw...)
	return c
}

// List 返回全部设备（按在线、IP 排序），不含原始数据以减小体积。
func (s *DeviceStore) List() []Device {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Device, 0, len(s.data.Devices))
	for _, d := range s.data.Devices {
		c := clone(d)
		c.Raw = nil
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Online != out[j].Online {
			return out[i].Online
		}
		return ipLess(out[i].IP, out[j].IP)
	})
	return out
}

func ipLess(a, b string) bool {
	pa, pb := net.ParseIP(a).To16(), net.ParseIP(b).To16()
	if pa == nil || pb == nil {
		return a < b
	}
	return string(pa) < string(pb)
}

func (s *DeviceStore) Get(key string) (Device, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.byK[key]
	if !ok {
		return Device{}, false
	}
	return clone(d), true
}

// FindByMAC 供面板卡片按 MAC 查询设备当前 IP。
func (s *DeviceStore) FindByMAC(mac string) (Device, bool) {
	return s.Get(normMAC(mac))
}

func (s *DeviceStore) Edit(key string, fn func(d *Device)) (Device, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.byK[key]
	if !ok {
		return Device{}, errors.New("设备不存在")
	}
	fn(d)
	return clone(d), s.saveLocked()
}

func (s *DeviceStore) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.byK[key]; !ok {
		return errors.New("设备不存在")
	}
	delete(s.byK, key)
	out := s.data.Devices[:0]
	for _, d := range s.data.Devices {
		if d.Key != key {
			out = append(out, d)
		}
	}
	s.data.Devices = out
	return s.saveLocked()
}

func (s *DeviceStore) AckAll() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, d := range s.data.Devices {
		d.Acked = true
	}
	return s.saveLocked()
}

func (s *DeviceStore) Scans() []ScanSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := append([]ScanSummary{}, s.data.Scans...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out
}

// Summary 统计：在线数、总数、未确认的新设备、近 7 天 IP 变化次数。
type Summary struct {
	Online    int `json:"online"`
	Total     int `json:"total"`
	New       int `json:"new"`
	IPChanged int `json:"ipChanged"`
	Web       int `json:"web"`
	WebHosts  int `json:"webHosts"`
}

func (s *DeviceStore) Summary() Summary {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var sm Summary
	week := time.Now().Add(-7 * 24 * time.Hour)
	for _, d := range s.data.Devices {
		sm.Total++
		if d.Online {
			sm.Online++
		}
		if !d.Acked && !d.Self {
			sm.New++
		}
		for _, h := range d.IPHistory {
			if h.From.After(week) && len(d.IPHistory) > 1 && h.To == nil {
				sm.IPChanged++
			}
		}
		hasWeb := false
		for _, sv := range d.Services {
			if sv.Web() {
				sm.Web++
				hasWeb = true
			}
		}
		if hasWeb {
			sm.WebHosts++
		}
	}
	return sm
}

// ---- 扫描结果合并 ----

// hostObs 是单次扫描中对某个 IP 的全部观测。
type hostObs struct {
	IP          string
	MAC         string
	Hostnames   map[string]string // 来源 → 名称
	Vendor      string
	Model       string
	OS          string
	Hints       map[string]int // 设备类型 → 权重
	Sources     map[string]bool
	Services    map[int]Service
	Raw         []RawEntry
	PortScanned bool
	Self        bool
	Gateway     bool
}

func newObs(ip string) *hostObs {
	return &hostObs{IP: ip, Hostnames: map[string]string{}, Hints: map[string]int{}, Sources: map[string]bool{}, Services: map[int]Service{}}
}

func (o *hostObs) hint(t string, w int) {
	if t != "" && o.Hints[t] < w {
		o.Hints[t] = w
	}
}

func (o *hostObs) raw(src, text string) {
	if len(o.Raw) < maxRawEntries {
		o.Raw = append(o.Raw, RawEntry{Source: src, Text: text})
	}
}

// addService 合并服务：端口扫描的识别结果优先于协议广播给出的服务。
func (o *hostObs) addService(sv Service) {
	if old, ok := o.Services[sv.Port]; ok {
		if old.Source == "port" && old.RuleID != "" && old.RuleID != "web-generic" {
			return
		}
		if sv.Source != "port" && old.Source == "port" {
			// 保留端口扫描的协议信息，用广播中的名称补充
			if old.RuleID == "web-generic" && sv.Name != "" {
				old.Name = sv.Name
			}
			o.Services[sv.Port] = old
			return
		}
	}
	o.Services[sv.Port] = sv
}

var hostnamePriority = []string{"DHCP", "mDNS", "NetBIOS", "DNS", "SSDP", "WSD"}

func (o *hostObs) bestHostname() string {
	for _, src := range hostnamePriority {
		if n := o.Hostnames[src]; n != "" {
			return n
		}
	}
	return ""
}

func (o *hostObs) bestType() string {
	best, bw := "", 0
	for t, w := range o.Hints {
		if w > bw || (w == bw && t < best) {
			best, bw = t, w
		}
	}
	return best
}

func normMAC(mac string) string {
	hw, err := net.ParseMAC(strings.TrimSpace(mac))
	if err != nil || len(hw) != 6 {
		return ""
	}
	return strings.ToLower(hw.String())
}

type mergeResult struct {
	Online, New, IPChanged, Services int
	Changes                          []IPChange
	NewDevices                       []string
}

// merge 把一次扫描的观测合并进设备库。
// scanned 为本次扫描覆盖的网段：其中未被观测到的设备标记为离线。
func (s *DeviceStore) merge(obs map[string]*hostObs, scanned []*net.IPNet, full bool, now time.Time) mergeResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	var res mergeResult

	// 同一 MAC 出现在多个 IP 上时，保留服务最多的那个
	byMAC := map[string]*hostObs{}
	var list []*hostObs
	for _, o := range obs {
		o.MAC = normMAC(o.MAC)
		if o.MAC == "" {
			list = append(list, o)
			continue
		}
		if prev, ok := byMAC[o.MAC]; ok && len(prev.Services) >= len(o.Services) {
			continue
		}
		byMAC[o.MAC] = o
	}
	for _, o := range byMAC {
		list = append(list, o)
	}

	// 首次扫描的结果作为基准，不标记为"新设备"
	baseline := len(s.data.Devices) == 0
	seen := map[string]bool{}
	for _, o := range list {
		key := o.MAC
		if key == "" {
			key = "ip:" + o.IP
		}
		d, exists := s.byK[key]
		// 之前只知道 IP、现在拿到了 MAC：迁移用户设置
		if o.MAC != "" {
			if legacy, ok := s.byK["ip:"+o.IP]; ok && !exists {
				delete(s.byK, legacy.Key)
				legacy.Key, legacy.MAC = key, o.MAC
				d, exists = legacy, true
				s.byK[key] = d
			}
		}
		if !exists {
			d = &Device{Key: key, MAC: o.MAC, IP: o.IP, FirstSeen: now, IPHistory: []IPRecord{{IP: o.IP, From: now}}, Acked: baseline}
			s.byK[key] = d
			s.data.Devices = append(s.data.Devices, d)
			res.New++
			res.NewDevices = append(res.NewDevices, key)
		}
		seen[key] = true

		if d.IP != o.IP && d.IP != "" {
			res.IPChanged++
			res.Changes = append(res.Changes, IPChange{MAC: d.MAC, OldIP: d.IP, NewIP: o.IP})
			if n := len(d.IPHistory); n > 0 && d.IPHistory[n-1].To == nil {
				t := now
				d.IPHistory[n-1].To = &t
			}
			d.IPHistory = append(d.IPHistory, IPRecord{IP: o.IP, From: now})
			if len(d.IPHistory) > maxIPHistory {
				d.IPHistory = d.IPHistory[len(d.IPHistory)-maxIPHistory:]
			}
		}
		d.IP = o.IP
		d.Online = true
		d.LastSeen = now
		d.Self, d.Gateway = o.Self, o.Gateway
		if o.MAC != "" {
			d.Vendor = oui.Lookup(o.MAC)
			d.Randomized = oui.IsRandomized(o.MAC)
		}
		if o.Vendor != "" && d.Vendor == "" {
			d.Vendor = o.Vendor
		}
		if h := o.bestHostname(); h != "" {
			d.Hostname = h
		}
		if o.Model != "" {
			d.Model = o.Model
		}
		if o.OS != "" {
			d.OS = o.OS
		}
		if t := o.bestType(); t != "" {
			d.Type = t
		}
		d.Sources = sortedKeys(o.Sources)
		if full || !exists {
			d.Raw = o.Raw
		}
		// 完整扫描替换服务列表；快速扫描只追加广播发现的服务
		if o.PortScanned {
			d.Services = sortedServices(o.Services)
		} else {
			m := map[int]Service{}
			for _, sv := range d.Services {
				m[sv.Port] = sv
			}
			for p, sv := range o.Services {
				if _, ok := m[p]; !ok {
					m[p] = sv
				}
			}
			d.Services = sortedServices(m)
		}
		res.Services += len(d.Services)
		res.Online++
	}

	// 覆盖网段内本次未出现的设备标记为离线
	for _, d := range s.data.Devices {
		if seen[d.Key] || !d.Online {
			continue
		}
		ip := net.ParseIP(d.IP)
		for _, n := range scanned {
			if ip != nil && n.Contains(ip) {
				d.Online = false
				break
			}
		}
	}
	_ = s.saveLocked()
	return res
}

func (s *DeviceStore) addScan(sum ScanSummary) ScanSummary {
	s.mu.Lock()
	defer s.mu.Unlock()
	sum.ID = s.data.NextID
	s.data.NextID++
	s.data.Scans = append(s.data.Scans, sum)
	if len(s.data.Scans) > maxHistory {
		s.data.Scans = s.data.Scans[len(s.data.Scans)-maxHistory:]
	}
	_ = s.saveLocked()
	return sum
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedServices(m map[int]Service) []Service {
	out := make([]Service, 0, len(m))
	for _, sv := range m {
		out = append(out, sv)
	}
	// Web 服务在前，其余按端口
	sort.Slice(out, func(i, j int) bool {
		if out[i].Web() != out[j].Web() {
			return out[i].Web()
		}
		return out[i].Port < out[j].Port
	})
	return out
}
