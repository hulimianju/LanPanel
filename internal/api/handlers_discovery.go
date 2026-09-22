package api

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"lanpanel/internal/discovery"
	"lanpanel/internal/rules"
	"lanpanel/internal/store"
)

// ---- 设备 ----

func (s *Server) listDevices(w http.ResponseWriter, r *http.Request) {
	st := s.scanner.Store()
	writeJSON(w, http.StatusOK, map[string]any{"devices": st.List(), "summary": st.Summary()})
}

func (s *Server) getDevice(w http.ResponseWriter, r *http.Request) {
	d, ok := s.scanner.Store().Get(r.PathValue("key"))
	if !ok {
		writeErr(w, http.StatusNotFound, "设备不存在")
		return
	}
	writeJSON(w, http.StatusOK, d)
}

var deviceTypes = map[string]bool{}

func init() {
	for _, t := range rules.DeviceTypes {
		deviceTypes[t] = true
	}
}

func (s *Server) updateDevice(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name     *string `json:"name"`
		UserType *string `json:"userType"`
		Acked    *bool   `json:"acked"`
	}
	if err := readJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.Name != nil && utf8.RuneCountInString(strings.TrimSpace(*body.Name)) > 40 {
		writeErr(w, http.StatusBadRequest, "名称不能超过 40 个字符")
		return
	}
	if body.UserType != nil && !deviceTypes[*body.UserType] {
		writeErr(w, http.StatusBadRequest, "未知设备类型")
		return
	}
	d, err := s.scanner.Store().Edit(r.PathValue("key"), func(d *discovery.Device) {
		if body.Name != nil {
			d.Name = strings.TrimSpace(*body.Name)
		}
		if body.UserType != nil {
			d.UserType = *body.UserType
		}
		if body.Acked != nil {
			d.Acked = *body.Acked
		}
	})
	if err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (s *Server) deleteDevice(w http.ResponseWriter, r *http.Request) {
	if err := s.scanner.Store().Delete(r.PathValue("key")); err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) ackAllDevices(w http.ResponseWriter, r *http.Request) {
	if err := s.scanner.Store().AckAll(); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ---- 扫描 ----

func (s *Server) discoveryStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  s.scanner.Status(),
		"summary": s.scanner.Store().Summary(),
	})
}

func (s *Server) discoveryLogs(w http.ResponseWriter, r *http.Request) {
	since, _ := strconv.Atoi(r.URL.Query().Get("since"))
	writeJSON(w, http.StatusOK, map[string]any{"logs": s.scanner.Logs(since)})
}

func (s *Server) startScan(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Mode string `json:"mode"`
	}
	_ = readJSON(r, &body)
	if body.Mode != "quick" {
		body.Mode = "full"
	}
	if err := s.scanner.Start(body.Mode, "manual"); err != nil {
		writeErr(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) cancelScan(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"canceled": s.scanner.Cancel()})
}

func (s *Server) listScans(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"scans": s.scanner.Store().Scans()})
}

// DiscoveryConfig 与 RuleList 供扫描器读取最新配置。
func (s *Server) DiscoveryConfig() discovery.Config {
	var c discovery.Config
	s.store.View(func(d *store.Data) { c = d.Discovery })
	return c
}

func (s *Server) RuleList() []rules.Rule {
	var user []rules.Rule
	s.store.View(func(d *store.Data) { user = append(user, d.RuleOverrides...) })
	return rules.Merge(user)
}

func (s *Server) getDiscoveryConfig(w http.ResponseWriter, r *http.Request) {
	cfg := s.DiscoveryConfig()
	writeJSON(w, http.StatusOK, map[string]any{
		"config":      cfg,
		"ruleCount":   len(s.RuleList()),
		"ports":       len(rules.Ports(s.RuleList())) + len(cfg.ExtraPorts),
		"lowResource": discovery.LowResource(),
	})
}

func (s *Server) putDiscoveryConfig(w http.ResponseWriter, r *http.Request) {
	cfg := s.DiscoveryConfig()
	if err := readJSON(r, &cfg); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	var subnets []string
	for _, c := range cfg.Subnets {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		_, n, err := net.ParseCIDR(c)
		if err != nil || n.IP.To4() == nil {
			writeErr(w, http.StatusBadRequest, "网段格式错误："+c+"（应为 192.168.1.0/24 形式）")
			return
		}
		if ones, _ := n.Mask.Size(); ones < 22 {
			writeErr(w, http.StatusBadRequest, "网段 "+c+" 过大，最大支持 /22（1022 台主机）")
			return
		}
		subnets = append(subnets, n.String())
	}
	cfg.Subnets = subnets
	if cfg.Subnets == nil {
		cfg.Subnets = []string{}
	}
	var ports []int
	for _, p := range cfg.ExtraPorts {
		if p < 1 || p > 65535 {
			writeErr(w, http.StatusBadRequest, fmt.Sprintf("端口超出范围: %d", p))
			return
		}
		ports = append(ports, p)
	}
	cfg.ExtraPorts = ports
	if cfg.ExtraPorts == nil {
		cfg.ExtraPorts = []int{}
	}
	cfg.Concurrency = clamp(cfg.Concurrency, 8, 1024)
	cfg.TimeoutMs = clamp(cfg.TimeoutMs, 200, 5000)
	cfg.FullInterval = clamp(cfg.FullInterval, 0, 24*60)
	cfg.QuickInterval = clamp(cfg.QuickInterval, 0, 24*60)
	cfg.SNMPCommunity = strings.TrimSpace(cfg.SNMPCommunity)
	if cfg.SNMPCommunity == "" {
		cfg.SNMPCommunity = "public"
	}
	if err := s.store.Update(func(d *store.Data) error { d.Discovery = cfg; return nil }); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

// ---- 规则 ----

func (s *Server) listRules(w http.ResponseWriter, r *http.Request) {
	list := s.RuleList()
	writeJSON(w, http.StatusOK, map[string]any{"rules": list, "ports": len(rules.Ports(list))})
}

// saveRule 新建或修改规则。修改内置规则会保存为覆盖项，可随时恢复默认。
func (s *Server) saveRule(w http.ResponseWriter, r *http.Request) {
	var rule rules.Rule
	if err := readJSON(r, &rule); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	pathID := r.PathValue("id")
	if pathID != "" {
		rule.ID = pathID
	}
	if err := rules.Validate(&rule); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	creating := pathID == ""
	builtin, _ := rules.Builtin()
	isBuiltin := false
	for _, b := range builtin {
		if b.ID == rule.ID {
			isBuiltin = true
		}
	}
	err := s.store.Update(func(d *store.Data) error {
		for i, o := range d.RuleOverrides {
			if o.ID == rule.ID {
				if creating {
					return errors.New("规则 ID 已存在：" + rule.ID)
				}
				d.RuleOverrides[i] = rule
				return nil
			}
		}
		if creating && isBuiltin {
			return errors.New("规则 ID 与内置规则重复：" + rule.ID)
		}
		if !creating && !isBuiltin {
			return errNotFound
		}
		d.RuleOverrides = append(d.RuleOverrides, rule)
		return nil
	})
	if err != nil {
		respondUpdate(w, err)
		return
	}
	for _, x := range s.RuleList() {
		if x.ID == rule.ID {
			writeJSON(w, http.StatusOK, x)
			return
		}
	}
	writeJSON(w, http.StatusOK, rule)
}

// deleteRule 删除自定义规则，或把内置规则恢复为默认。
func (s *Server) deleteRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	err := s.store.Update(func(d *store.Data) error {
		for i, o := range d.RuleOverrides {
			if o.ID == id {
				d.RuleOverrides = append(d.RuleOverrides[:i], d.RuleOverrides[i+1:]...)
				return nil
			}
		}
		return errors.New("内置规则未修改过，无需恢复")
	})
	respondUpdate(w, err)
}

func (s *Server) exportRules(w http.ResponseWriter, r *http.Request) {
	list := s.RuleList()
	if r.URL.Query().Get("scope") == "custom" {
		var out []rules.Rule
		for _, x := range list {
			if !x.Builtin || x.Modified {
				out = append(out, x)
			}
		}
		list = out
	}
	b, err := rules.ExportYAML(list)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/x-yaml; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="lanpanel-rules-%s.yaml"`, time.Now().Format("20060102")))
	_, _ = w.Write(append([]byte("# LanPanel 端口与指纹规则\n"), b...))
}

// importRules 导入 YAML：同 ID 覆盖，其余追加为自定义规则。
func (s *Server) importRules(w http.ResponseWriter, r *http.Request) {
	var body struct {
		YAML string `json:"yaml"`
	}
	if err := readJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	list, err := rules.ParseYAML([]byte(body.YAML))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	builtin, _ := rules.Builtin()
	bmap := map[string]rules.Rule{}
	for _, b := range builtin {
		bmap[b.ID] = b
	}
	var changed int
	err = s.store.Update(func(d *store.Data) error {
		idx := map[string]int{}
		for i, o := range d.RuleOverrides {
			idx[o.ID] = i
		}
		for _, rl := range list {
			// 与内置规则完全相同的条目无需保存
			if b, ok := bmap[rl.ID]; ok && sameRule(b, rl) {
				continue
			}
			if i, ok := idx[rl.ID]; ok {
				d.RuleOverrides[i] = rl
			} else {
				idx[rl.ID] = len(d.RuleOverrides)
				d.RuleOverrides = append(d.RuleOverrides, rl)
			}
			changed++
		}
		return nil
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"total": len(list), "changed": changed})
}

func sameRule(a, b rules.Rule) bool {
	a.Builtin, b.Builtin, a.Modified, b.Modified = false, false, false, false
	x, _ := rules.ExportYAML([]rules.Rule{a})
	y, _ := rules.ExportYAML([]rules.Rule{b})
	return string(x) == string(y)
}

// testRule 用当前规则（可附带未保存的草稿规则）探测指定 IP，便于调试规则。
func (s *Server) testRule(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IP    string      `json:"ip"`
		Ports []int       `json:"ports"`
		Rule  *rules.Rule `json:"rule"`
	}
	if err := readJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	ip := net.ParseIP(strings.TrimSpace(body.IP))
	if ip == nil || ip.To4() == nil || !isLAN(ip) {
		writeErr(w, http.StatusBadRequest, "请输入局域网内的 IPv4 地址")
		return
	}
	list := s.RuleList()
	if body.Rule != nil {
		draft := *body.Rule
		if err := rules.Validate(&draft); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		replaced := false
		for i := range list {
			if list[i].ID == draft.ID {
				list[i], replaced = draft, true
			}
		}
		if !replaced {
			list = append(list, draft)
		}
		if len(body.Ports) == 0 {
			body.Ports = draft.Ports
		}
	}
	if len(body.Ports) == 0 || len(body.Ports) > 32 {
		writeErr(w, http.StatusBadRequest, "请指定 1–32 个端口")
		return
	}
	cfg := s.DiscoveryConfig()
	results := discovery.TestPorts(r.Context(), ip.String(), body.Ports, list, time.Duration(cfg.TimeoutMs)*time.Millisecond)
	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}
