package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"lanpanel/internal/discovery"
	"lanpanel/internal/linkage"
	"lanpanel/internal/notify"
	"lanpanel/internal/rules"
	"lanpanel/internal/store"
)

// ---- 卡片与设备联动 ----

// watched 判断设备是否绑定了面板卡片（只为这类设备推送上线 / 离线）。
func (s *Server) watched(mac string) bool {
	found := false
	s.store.View(func(d *store.Data) {
		for _, it := range d.Items {
			if it.DeviceMAC != "" && it.DeviceMAC == mac {
				found = true
				return
			}
		}
	})
	return found
}

type itemChange struct {
	ItemID, Title, MAC, Old, New string
}

// reconcile 让绑定了设备的卡片地址与设备当前 IP 保持一致。
// 只改写地址中的局域网 IP；地址是域名或公网 IP 时保持原样。
func (s *Server) reconcile() []itemChange {
	devices := s.scanner.Store()
	var changes []itemChange
	err := s.store.Update(func(d *store.Data) error {
		for i := range d.Items {
			it := &d.Items[i]
			if it.DeviceMAC == "" {
				continue
			}
			dev, ok := devices.FindByMAC(it.DeviceMAC)
			if !ok || dev.IP == "" {
				continue
			}
			for _, u := range []*string{&it.URLLan, &it.URLWan} {
				ip := linkage.HostIP(*u)
				if ip == nil || !linkage.IsLAN(ip) || ip.String() == dev.IP {
					continue
				}
				if nu, ok := linkage.ReplaceHost(*u, dev.IP); ok {
					changes = append(changes, itemChange{ItemID: it.ID, Title: it.Title, MAC: it.DeviceMAC, Old: *u, New: nu})
					*u = nu
					it.UpdatedAt = time.Now()
				}
			}
		}
		if len(changes) == 0 {
			return errNoChange
		}
		return nil
	})
	if err != nil && err != errNoChange {
		log.Printf("同步卡片地址失败: %v", err)
	}
	for _, c := range changes {
		log.Printf("卡片「%s」地址已更新：%s → %s", c.Title, c.Old, c.New)
	}
	return changes
}

var errNoChange = fmt.Errorf("无变化")

// onScan 在每次扫描后执行：同步卡片地址、补充动态说明、推送通知。
func (s *Server) onScan(_ discovery.ScanSummary, events []discovery.Event) []discovery.Event {
	changes := s.reconcile()
	byMAC := map[string]int{}
	for _, c := range changes {
		byMAC[c.MAC]++
	}
	for i := range events {
		if events[i].Type == "ip_changed" && byMAC[events[i].MAC] > 0 {
			events[i].Detail = fmt.Sprintf("已更新 %d 张面板卡片", byMAC[events[i].MAC])
		}
	}
	if len(events) > 0 {
		go s.notifyEvents(events)
	}
	return events
}

// ---- 通知 ----

type notifyState struct {
	mu       sync.Mutex
	lastSent time.Time
	lastErr  string
}

func eventLine(e discovery.Event) string {
	switch e.Type {
	case "new":
		extra := e.IP
		if e.Vendor != "" && e.Vendor != e.Name {
			extra += "，" + e.Vendor
		}
		return fmt.Sprintf("新设备：%s（%s）", e.Name, extra)
	case "ip_changed":
		line := fmt.Sprintf("IP 变化：%s %s → %s", e.Name, e.OldIP, e.IP)
		if e.Detail != "" {
			line += "，" + e.Detail
		}
		return line
	case "offline":
		return fmt.Sprintf("设备离线：%s（%s）", e.Name, e.IP)
	case "online":
		return fmt.Sprintf("恢复在线：%s（%s）", e.Name, e.IP)
	}
	return e.Name
}

func (s *Server) notifyConfig() notify.Config {
	var c notify.Config
	s.store.View(func(d *store.Data) { c = d.Notify })
	return c
}

func (s *Server) notifyEvents(events []discovery.Event) {
	cfg := s.notifyConfig()
	if !cfg.Enabled || cfg.URL == "" {
		return
	}
	var lines []string
	var data []discovery.Event
	for _, e := range events {
		if cfg.Wants(e.Type) {
			lines = append(lines, eventLine(e))
			data = append(data, e)
		}
	}
	if len(lines) == 0 {
		return
	}
	title := "LanPanel 设备动态"
	if len(lines) > 1 {
		title = fmt.Sprintf("LanPanel：%d 条设备动态", len(lines))
	}
	if len(lines) > 30 { // 首次扫描前后可能有大量新设备，避免超长消息
		n := len(lines)
		lines = append(lines[:30], fmt.Sprintf("……以及另外 %d 条，详见 LanPanel", n-30))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	err := notify.Send(ctx, cfg, notify.Message{Title: title, Lines: lines, Data: data})
	s.notifyState.mu.Lock()
	defer s.notifyState.mu.Unlock()
	if err != nil {
		s.notifyState.lastErr = err.Error()
		log.Printf("推送通知失败: %v", err)
		return
	}
	s.notifyState.lastErr = ""
	s.notifyState.lastSent = time.Now()
}

func (s *Server) getNotify(w http.ResponseWriter, r *http.Request) {
	s.notifyState.mu.Lock()
	lastErr, lastSent := s.notifyState.lastErr, s.notifyState.lastSent
	s.notifyState.mu.Unlock()
	out := map[string]any{"config": s.notifyConfig(), "lastError": lastErr}
	if !lastSent.IsZero() {
		out["lastSent"] = lastSent
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) putNotify(w http.ResponseWriter, r *http.Request) {
	cfg := s.notifyConfig()
	if err := readJSON(r, &cfg); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := cfg.Validate(); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.store.Update(func(d *store.Data) error { d.Notify = cfg; return nil }); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

// testNotify 用提交的配置（可以是尚未保存的）发送一条测试消息。
func (s *Server) testNotify(w http.ResponseWriter, r *http.Request) {
	cfg := s.notifyConfig()
	if err := readJSON(r, &cfg); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	cfg.Enabled = true
	if err := cfg.Validate(); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	msg := notify.Message{
		Title: "LanPanel 测试消息",
		Lines: []string{"这是一条测试消息，收到说明推送配置正确。", "IP 变化示例：客厅 NAS 192.168.1.23 → 192.168.1.105，已更新 2 张面板卡片"},
		Data:  []discovery.Event{{Type: "test", Time: time.Now(), Name: "LanPanel"}},
	}
	if err := notify.Send(r.Context(), cfg, msg); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) listEvents(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 300 {
		limit = 50
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": s.scanner.Store().Events(limit)})
}

// ---- 卡片状态 ----

type itemStatus struct {
	State  string `json:"state"` // online | warn | offline
	Bound  bool   `json:"bound"`
	Device string `json:"device,omitempty"` // 以下字段仅管理员可见
	Key    string `json:"key,omitempty"`
	MAC    string `json:"mac,omitempty"`
	IP     string `json:"ip,omitempty"`
	Note   string `json:"note,omitempty"`
}

// panelStatus 计算卡片对应设备的状态：绑定的按 MAC，未绑定的按地址中的 IP 匹配。
func (s *Server) panelStatus(items []store.Item, admin bool) map[string]itemStatus {
	devices := s.scanner.Store()
	cfg := s.DiscoveryConfig()
	scanned := map[int]bool{}
	for _, p := range rules.Ports(s.RuleList()) {
		scanned[p] = true
	}
	for _, p := range cfg.ExtraPorts {
		scanned[p] = true
	}
	out := map[string]itemStatus{}
	for _, it := range items {
		var dev discovery.Device
		var ok, bound bool
		if it.DeviceMAC != "" {
			dev, ok = devices.FindByMAC(it.DeviceMAC)
			bound = ok
		}
		url := it.URLLan
		if url == "" {
			url = it.URLWan
		}
		if !ok {
			if ip := linkage.HostIP(url); ip != nil && linkage.IsLAN(ip) {
				dev, ok = devices.ByIP(ip.String())
			}
		}
		if !ok {
			continue
		}
		st := itemStatus{State: "offline", Bound: bound}
		if dev.Online {
			st.State = "online"
			port := it.DevicePort
			if port == 0 {
				port = linkage.Port(url)
			}
			// 只有端口在扫描范围内、且设备做过端口扫描时，才能判断服务是否在运行
			portScanned := false
			found := false
			for _, sv := range dev.Services {
				if sv.Source == "port" {
					portScanned = true
				}
				if sv.Port == port {
					found = true
				}
			}
			if port > 0 && scanned[port] && portScanned && !found {
				st.State = "warn"
				st.Note = fmt.Sprintf("设备在线，但上次扫描时端口 %d 没有响应", port)
			}
		}
		if admin {
			st.Device, st.Key, st.MAC, st.IP = dev.DisplayName(), dev.Key, dev.MAC, dev.IP
		}
		out[it.ID] = st
	}
	return out
}
