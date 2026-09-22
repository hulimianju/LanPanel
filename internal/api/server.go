// Package api 提供 LanPanel 的 HTTP 接口与静态资源服务。
package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log"
	"mime"
	"net"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"time"

	"lanpanel/internal/auth"
	"lanpanel/internal/discovery"
	"lanpanel/internal/store"
	"lanpanel/internal/sysinfo"
)

type ctxKey int

const userKey ctxKey = 1

type Server struct {
	store      *store.Store
	scanner    *discovery.Scanner
	sampler    *sysinfo.Sampler
	limiter    *auth.Limiter
	uploadsDir string
	web        fs.FS // 前端构建产物（dist）
	version    string
}

func New(st *store.Store, devices *discovery.DeviceStore, dataDir string, web fs.FS, version string) *Server {
	s := &Server{
		store:      st,
		sampler:    sysinfo.NewSampler(),
		limiter:    auth.NewLimiter(8, 10*time.Minute),
		uploadsDir: filepath.Join(dataDir, "uploads"),
		web:        web,
		version:    version,
	}
	s.scanner = discovery.NewScanner(devices, s.DiscoveryConfig, s.RuleList, nil)
	return s
}

// Scanner 返回设备扫描器（由 main 启动定时调度）。
func (s *Server) Scanner() *discovery.Scanner { return s.scanner }

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// 公开接口
	mux.HandleFunc("GET /api/bootstrap", s.bootstrap)
	mux.HandleFunc("POST /api/setup", s.setup)
	mux.HandleFunc("POST /api/login", s.login)
	mux.HandleFunc("POST /api/logout", s.logout)
	mux.HandleFunc("GET /api/panel", s.viewer(s.getPanel))
	mux.HandleFunc("GET /api/system", s.viewer(s.system))
	mux.HandleFunc("GET /uploads/{name}", s.serveUpload)

	// 管理接口
	mux.HandleFunc("PUT /api/account/password", s.admin(s.changePassword))
	mux.HandleFunc("POST /api/groups", s.admin(s.createGroup))
	mux.HandleFunc("PUT /api/groups/order", s.admin(s.orderGroups))
	mux.HandleFunc("PUT /api/groups/{id}", s.admin(s.updateGroup))
	mux.HandleFunc("DELETE /api/groups/{id}", s.admin(s.deleteGroup))
	mux.HandleFunc("POST /api/items", s.admin(s.createItem))
	mux.HandleFunc("PUT /api/items/layout", s.admin(s.layoutItems))
	mux.HandleFunc("PUT /api/items/{id}", s.admin(s.updateItem))
	mux.HandleFunc("DELETE /api/items/{id}", s.admin(s.deleteItem))
	mux.HandleFunc("GET /api/settings", s.admin(s.getSettings))
	mux.HandleFunc("PUT /api/settings", s.admin(s.putSettings))
	mux.HandleFunc("POST /api/upload", s.admin(s.upload))
	mux.HandleFunc("POST /api/icon/fetch", s.admin(s.fetchIcon))
	mux.HandleFunc("GET /api/export", s.admin(s.exportData))
	mux.HandleFunc("POST /api/import", s.admin(s.importData))

	// 设备发现
	mux.HandleFunc("GET /api/discovery/devices", s.admin(s.listDevices))
	mux.HandleFunc("GET /api/discovery/devices/{key}", s.admin(s.getDevice))
	mux.HandleFunc("PUT /api/discovery/devices/{key}", s.admin(s.updateDevice))
	mux.HandleFunc("DELETE /api/discovery/devices/{key}", s.admin(s.deleteDevice))
	mux.HandleFunc("POST /api/discovery/ack-all", s.admin(s.ackAllDevices))
	mux.HandleFunc("GET /api/discovery/status", s.admin(s.discoveryStatus))
	mux.HandleFunc("GET /api/discovery/logs", s.admin(s.discoveryLogs))
	mux.HandleFunc("POST /api/discovery/scan", s.admin(s.startScan))
	mux.HandleFunc("POST /api/discovery/cancel", s.admin(s.cancelScan))
	mux.HandleFunc("GET /api/discovery/scans", s.admin(s.listScans))
	mux.HandleFunc("GET /api/discovery/config", s.admin(s.getDiscoveryConfig))
	mux.HandleFunc("PUT /api/discovery/config", s.admin(s.putDiscoveryConfig))

	// 端口规则
	mux.HandleFunc("GET /api/rules", s.admin(s.listRules))
	mux.HandleFunc("POST /api/rules", s.admin(s.saveRule))
	mux.HandleFunc("GET /api/rules/export", s.admin(s.exportRules))
	mux.HandleFunc("POST /api/rules/import", s.admin(s.importRules))
	mux.HandleFunc("POST /api/rules/test", s.admin(s.testRule))
	mux.HandleFunc("PUT /api/rules/{id}", s.admin(s.saveRule))
	mux.HandleFunc("DELETE /api/rules/{id}", s.admin(s.deleteRule))

	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeErr(w, http.StatusNotFound, "接口不存在")
	})
	mux.HandleFunc("/", s.serveWeb)
	return s.middleware(mux)
}

func (s *Server) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic: %v", rec)
				writeErr(w, http.StatusInternalServerError, "服务器内部错误")
			}
		}()
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("X-Frame-Options", "SAMEORIGIN")

		// 修改类请求必须是 JSON 或上传表单，配合 SameSite Cookie 防御 CSRF
		if r.Method != http.MethodGet && r.Method != http.MethodHead && strings.HasPrefix(r.URL.Path, "/api/") {
			ct, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if ct != "application/json" && ct != "multipart/form-data" && r.ContentLength > 0 {
				writeErr(w, http.StatusUnsupportedMediaType, "请求格式错误")
				return
			}
		}
		if user := s.currentUser(r); user != "" {
			r = r.WithContext(context.WithValue(r.Context(), userKey, user))
		}
		next.ServeHTTP(w, r)
		if strings.HasPrefix(r.URL.Path, "/api/") && r.Method != http.MethodGet {
			log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
		}
	})
}

func (s *Server) currentUser(r *http.Request) string {
	c, err := r.Cookie(auth.CookieName)
	if err != nil || c.Value == "" {
		return ""
	}
	var secret string
	s.store.View(func(d *store.Data) { secret = d.Secret })
	user, err := auth.Verify(secret, c.Value, func(name string) (hash string, ok bool) {
		s.store.View(func(d *store.Data) {
			for _, u := range d.Users {
				if u.Username == name {
					hash, ok = u.PasswordHash, true
				}
			}
		})
		return
	})
	if err != nil {
		return ""
	}
	return user
}

func userOf(r *http.Request) string {
	u, _ := r.Context().Value(userKey).(string)
	return u
}

// admin 要求已登录。
func (s *Server) admin(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if userOf(r) == "" {
			writeErr(w, http.StatusUnauthorized, "请先登录")
			return
		}
		h(w, r)
	}
}

// viewer 允许已登录用户，或在开启访客模式时允许匿名访问。
func (s *Server) viewer(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if userOf(r) == "" {
			var public bool
			s.store.View(func(d *store.Data) { public = d.Settings.PublicPanel })
			if !public {
				writeErr(w, http.StatusUnauthorized, "请先登录")
				return
			}
		}
		h(w, r)
	}
}

// ---- 静态资源 ----

func (s *Server) serveWeb(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeErr(w, http.StatusMethodNotAllowed, "不支持的请求方法")
		return
	}
	p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if p != "" && p != "index.html" {
		if f, err := s.web.Open(p); err == nil {
			st, _ := f.Stat()
			f.Close()
			if st != nil && !st.IsDir() {
				if strings.HasPrefix(p, "assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				http.ServeFileFS(w, r, s.web, p)
				return
			}
		}
		if path.Ext(p) != "" {
			http.NotFound(w, r)
			return
		}
	}
	// 单页应用：其余路径都返回 index.html
	index, err := fs.ReadFile(s.web, "index.html")
	if err != nil {
		http.Error(w, "前端未构建，请先执行 make web", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(index)
}

// ---- 工具函数 ----

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func readJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, 4<<20))
	if err := dec.Decode(v); err != nil {
		if errors.Is(err, io.EOF) {
			return errors.New("请求体为空")
		}
		return errors.New("JSON 格式错误: " + err.Error())
	}
	return nil
}

// clientIP 返回访问者 IP；只有直连来源本身是内网地址（反向代理）时才信任转发头。
func clientIP(r *http.Request) net.IP {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	if ip != nil && isLAN(ip) {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			first, _, _ := strings.Cut(xff, ",")
			if fip := net.ParseIP(strings.TrimSpace(first)); fip != nil {
				return fip
			}
		}
		if xr := net.ParseIP(r.Header.Get("X-Real-IP")); xr != nil {
			return xr
		}
	}
	return ip
}

func isLAN(ip net.IP) bool {
	return ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() ||
		// 100.64.0.0/10：CGNAT，常见于 Tailscale 等组网工具
		(ip.To4() != nil && ip.To4()[0] == 100 && ip.To4()[1]&0xC0 == 64)
}
