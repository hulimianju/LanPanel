package api

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"lanpanel/internal/icon"
	"lanpanel/internal/store"
)

// ---- 设置 ----

func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	var st store.Settings
	s.store.View(func(d *store.Data) { st = d.Settings })
	writeJSON(w, http.StatusOK, st)
}

var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func oneOf(v string, allowed ...string) bool {
	for _, a := range allowed {
		if v == a {
			return true
		}
	}
	return false
}

func clamp(v, lo, hi int) int { return max(lo, min(hi, v)) }

func validateSettings(st *store.Settings) error {
	st.SiteTitle = strings.TrimSpace(st.SiteTitle)
	if st.SiteTitle == "" {
		st.SiteTitle = "LanPanel"
	}
	if utf8.RuneCountInString(st.SiteTitle) > 32 {
		return errors.New("站点标题不能超过 32 个字符")
	}
	if !oneOf(st.Theme, "auto", "light", "dark") {
		return errors.New("主题取值无效")
	}
	wp := &st.Wallpaper
	switch wp.Type {
	case "none":
		wp.Value = ""
	case "image":
		if !strings.HasPrefix(wp.Value, "/uploads/") && validateLink(wp.Value) != nil {
			return errors.New("壁纸地址无效")
		}
	case "color":
		if !hexColor.MatchString(wp.Value) {
			return errors.New("壁纸颜色需为 #RRGGBB 格式")
		}
	default:
		return errors.New("壁纸类型无效")
	}
	wp.Blur, wp.Dim = clamp(wp.Blur, 0, 40), clamp(wp.Dim, 0, 80)
	if !oneOf(wp.Tone, "auto", "light", "dark") {
		return errors.New("面板色调取值无效")
	}
	if wp.Luminance < 0 || wp.Luminance > 1 {
		wp.Luminance = -1
	}
	if !oneOf(st.SearchEngine, "bing", "baidu", "google", "duckduckgo", "custom") {
		return errors.New("搜索引擎取值无效")
	}
	if st.SearchEngine == "custom" && !strings.Contains(st.CustomSearchURL, "%s") {
		return errors.New("自定义搜索地址需包含 %s 作为关键词占位符")
	}
	if !oneOf(st.AddressMode, "auto", "lan", "wan") {
		return errors.New("地址模式取值无效")
	}
	if !oneOf(st.CardSize, "comfortable", "compact") {
		return errors.New("卡片尺寸取值无效")
	}
	return nil
}

func (s *Server) putSettings(w http.ResponseWriter, r *http.Request) {
	var st store.Settings
	s.store.View(func(d *store.Data) { st = d.Settings }) // 未提交的字段保持原值
	if err := readJSON(r, &st); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateSettings(&st); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.store.Update(func(d *store.Data) error { d.Settings = st; return nil }); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.gcUploads()
	writeJSON(w, http.StatusOK, st)
}

// ---- 上传 ----

var imageExt = map[string]string{
	"image/png": ".png", "image/jpeg": ".jpg", "image/gif": ".gif", "image/webp": ".webp",
	"image/x-icon": ".ico", "image/svg+xml": ".svg",
}

// saveUpload 以内容哈希命名保存文件，相同文件只存一份。
func (s *Server) saveUpload(data []byte, mime string) (string, error) {
	ext, ok := imageExt[mime]
	if !ok {
		return "", errors.New("仅支持 PNG / JPG / GIF / WebP / ICO / SVG 图片")
	}
	if err := os.MkdirAll(s.uploadsDir, 0o755); err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	name := hex.EncodeToString(sum[:10]) + ext
	p := filepath.Join(s.uploadsDir, name)
	if _, err := os.Stat(p); err != nil {
		if err := os.WriteFile(p, data, 0o644); err != nil {
			return "", err
		}
	}
	return "/uploads/" + name, nil
}

func (s *Server) upload(w http.ResponseWriter, r *http.Request) {
	kind := r.URL.Query().Get("kind") // wallpaper | icon
	limit := int64(1 << 20)
	if kind == "wallpaper" {
		limit = 12 << 20
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit+64<<10)
	f, _, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("读取文件失败（单个文件不超过 %d MB）", limit>>20))
		return
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil || int64(len(data)) > limit {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("文件不能超过 %d MB", limit>>20))
		return
	}
	mime := icon.Sniff(data)
	if kind == "wallpaper" && (mime == "image/svg+xml" || mime == "image/x-icon") {
		mime = ""
	}
	u, err := s.saveUpload(data, mime)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"url": u})
}

func (s *Server) serveUpload(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if strings.ContainsAny(name, `/\`) || strings.HasPrefix(name, ".") {
		http.NotFound(w, r)
		return
	}
	h := w.Header()
	// 上传的 SVG 可能包含脚本：沙箱化，禁止执行
	h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; img-src data:; sandbox")
	h.Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeFile(w, r, filepath.Join(s.uploadsDir, name))
}

// gcUploads 删除不再被引用、且创建超过 1 小时的上传文件（留出"已上传未保存"的窗口）。
func (s *Server) gcUploads() {
	used := map[string]bool{}
	s.store.View(func(d *store.Data) {
		if d.Settings.Wallpaper.Type == "image" {
			used[strings.TrimPrefix(d.Settings.Wallpaper.Value, "/uploads/")] = true
		}
		for _, it := range d.Items {
			if it.Icon.Type == "image" || it.Icon.Type == "auto" {
				used[strings.TrimPrefix(it.Icon.Value, "/uploads/")] = true
			}
		}
	})
	entries, err := os.ReadDir(s.uploadsDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if used[e.Name()] || e.IsDir() {
			continue
		}
		if info, err := e.Info(); err == nil && time.Since(info.ModTime()) > time.Hour {
			_ = os.Remove(filepath.Join(s.uploadsDir, e.Name()))
		}
	}
}

// ---- 图标抓取 ----

func (s *Server) fetchIcon(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL string `json:"url"`
	}
	if err := readJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	res, err := icon.Fetch(r.Context(), body.URL)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "无法访问该地址: "+err.Error())
		return
	}
	out := map[string]any{"title": res.Title, "icon": nil}
	if res.Icon != nil {
		if u, err := s.saveUpload(res.Icon, res.IconType); err == nil {
			out["icon"] = u
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// ---- 导入导出 ----

type exportData struct {
	App      string         `json:"app"`
	Version  int            `json:"version"`
	Exported time.Time      `json:"exported"`
	Settings store.Settings `json:"settings"`
	Groups   []store.Group  `json:"groups"`
	Items    []store.Item   `json:"items"`
}

func (s *Server) exportData(w http.ResponseWriter, r *http.Request) {
	out := exportData{App: "lanpanel", Version: 1, Exported: time.Now()}
	s.store.View(func(d *store.Data) {
		out.Settings, out.Groups, out.Items = d.Settings, d.Groups, d.Items
	})
	w.Header().Set("Content-Disposition",
		fmt.Sprintf(`attachment; filename="lanpanel-%s.json"`, time.Now().Format("20060102-150405")))
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) importData(w http.ResponseWriter, r *http.Request) {
	var in struct {
		exportData
		Mode string `json:"mode"` // replace | merge
	}
	if err := readJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if in.App != "lanpanel" {
		writeErr(w, http.StatusBadRequest, "不是 LanPanel 导出的文件")
		return
	}
	// 重新分配 ID，避免与现有数据冲突
	idMap := map[string]string{}
	groups := make([]store.Group, 0, len(in.Groups))
	for _, g := range in.Groups {
		name, err := cleanName(g.Name, 32)
		if err != nil {
			continue
		}
		idMap[g.ID] = store.NewID()
		groups = append(groups, store.Group{ID: idMap[g.ID], Name: name, Order: g.Order})
	}
	items := make([]store.Item, 0, len(in.Items))
	for _, it := range in.Items {
		gid, ok := idMap[it.GroupID]
		if !ok {
			continue
		}
		v := itemInput{GroupID: gid, Title: it.Title, Desc: it.Desc, URLLan: it.URLLan, URLWan: it.URLWan, Icon: it.Icon, OpenMode: it.OpenMode}
		if v.validate() != nil {
			continue
		}
		now := time.Now()
		items = append(items, store.Item{
			ID: store.NewID(), GroupID: gid, Title: v.Title, Desc: v.Desc, URLLan: v.URLLan, URLWan: v.URLWan,
			Icon: v.Icon, OpenMode: v.OpenMode, Order: it.Order, CreatedAt: now, UpdatedAt: now,
		})
	}
	err := s.store.Update(func(d *store.Data) error {
		if in.Mode == "replace" {
			d.Groups, d.Items = nil, nil
			if validateSettings(&in.Settings) == nil {
				d.Settings = in.Settings
			}
		}
		offset := len(d.Groups)
		for _, g := range groups {
			g.Order += offset
			d.Groups = append(d.Groups, g)
		}
		d.Items = append(d.Items, items...)
		return nil
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"groups": len(groups), "items": len(items)})
}
