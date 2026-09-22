package api

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"lanpanel/internal/auth"
	"lanpanel/internal/store"
)

// ---- 引导与账号 ----

func (s *Server) bootstrap(w http.ResponseWriter, r *http.Request) {
	var needSetup bool
	var settings store.Settings
	s.store.View(func(d *store.Data) {
		needSetup = len(d.Users) == 0
		settings = d.Settings
	})
	var user any
	if u := userOf(r); u != "" {
		user = u
	}
	ip := clientIP(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"needSetup": needSetup,
		"user":      user,
		"settings":  settings,
		"clientLan": ip != nil && isLAN(ip),
		"version":   s.version,
	})
}

type credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Remember bool   `json:"remember"`
}

func (s *Server) setup(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if err := readJSON(r, &c); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	var exists bool
	s.store.View(func(d *store.Data) { exists = len(d.Users) > 0 })
	if exists {
		writeErr(w, http.StatusConflict, "管理员已创建")
		return
	}
	c.Username = strings.TrimSpace(c.Username)
	if err := validateCredentials(c.Username, c.Password); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	hash, err := auth.HashPassword(c.Password)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	var secret string
	err = s.store.Update(func(d *store.Data) error {
		if len(d.Users) > 0 {
			return errors.New("管理员已创建")
		}
		d.Users = append(d.Users, store.User{Username: c.Username, PasswordHash: hash, CreatedAt: time.Now()})
		if len(d.Groups) == 0 {
			seedPanel(d)
		}
		secret = d.Secret
		return nil
	})
	if err != nil {
		writeErr(w, http.StatusConflict, err.Error())
		return
	}
	ttl := 30 * 24 * time.Hour
	auth.SetCookie(w, r, auth.Sign(secret, c.Username, hash, ttl), ttl, true)
	writeJSON(w, http.StatusOK, map[string]string{"user": c.Username})
}

func validateCredentials(user, pw string) error {
	if n := utf8.RuneCountInString(user); n < 2 || n > 32 {
		return errors.New("用户名长度需为 2–32 个字符")
	}
	if len(pw) < 6 || len(pw) > 128 {
		return errors.New("密码长度需为 6–128 位")
	}
	return nil
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	key := clientIP(r).String()
	if s.limiter.Blocked(key) {
		writeErr(w, http.StatusTooManyRequests, "登录失败次数过多，请 10 分钟后再试")
		return
	}
	var c credentials
	if err := readJSON(r, &c); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	var hash, secret string
	s.store.View(func(d *store.Data) {
		secret = d.Secret
		for _, u := range d.Users {
			if u.Username == strings.TrimSpace(c.Username) {
				hash = u.PasswordHash
			}
		}
	})
	if hash == "" || !auth.CheckPassword(hash, c.Password) {
		s.limiter.Fail(key)
		writeErr(w, http.StatusUnauthorized, "用户名或密码错误")
		return
	}
	s.limiter.Reset(key)
	ttl := 12 * time.Hour
	if c.Remember {
		ttl = 30 * 24 * time.Hour
	}
	auth.SetCookie(w, r, auth.Sign(secret, strings.TrimSpace(c.Username), hash, ttl), ttl, c.Remember)
	writeJSON(w, http.StatusOK, map[string]string{"user": strings.TrimSpace(c.Username)})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	auth.ClearCookie(w)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Old string `json:"old"`
		New string `json:"new"`
	}
	if err := readJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	user := userOf(r)
	if err := validateCredentials(user, body.New); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	hash, err := auth.HashPassword(body.New)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	var secret string
	err = s.store.Update(func(d *store.Data) error {
		secret = d.Secret
		for i, u := range d.Users {
			if u.Username == user {
				if !auth.CheckPassword(u.PasswordHash, body.Old) {
					return errors.New("原密码错误")
				}
				d.Users[i].PasswordHash = hash
				return nil
			}
		}
		return errors.New("用户不存在")
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	// 修改密码会使旧会话失效，这里为当前浏览器签发新会话
	ttl := 30 * 24 * time.Hour
	auth.SetCookie(w, r, auth.Sign(secret, user, hash, ttl), ttl, true)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// seedPanel 首次初始化时创建示例分组，便于用户上手。
func seedPanel(d *store.Data) {
	now := time.Now()
	g := store.Group{ID: store.NewID(), Name: "常用", Order: 0}
	d.Groups = append(d.Groups, g)
	d.Items = append(d.Items, store.Item{
		ID: store.NewID(), GroupID: g.ID, Title: "LanPanel 使用说明", Desc: "项目主页",
		URLLan: "https://github.com/", Icon: store.Icon{Type: "lucide", Value: "book-open", Color: "blue"},
		OpenMode: "new", CreatedAt: now, UpdatedAt: now,
	})
}

// ---- 面板 ----

type groupView struct {
	store.Group
	Items []store.Item `json:"items"`
}

func panelView(d *store.Data) []groupView {
	out := make([]groupView, 0, len(d.Groups))
	idx := map[string]int{}
	for _, g := range d.Groups {
		idx[g.ID] = len(out)
		out = append(out, groupView{Group: g, Items: []store.Item{}})
	}
	for _, it := range d.Items {
		if i, ok := idx[it.GroupID]; ok {
			out[i].Items = append(out[i].Items, it)
		}
	}
	return out
}

func (s *Server) getPanel(w http.ResponseWriter, r *http.Request) {
	var groups []groupView
	s.store.View(func(d *store.Data) { groups = panelView(d) })
	writeJSON(w, http.StatusOK, map[string]any{"groups": groups})
}

func (s *Server) system(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.sampler.Get())
}

func cleanName(name string, max int) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("名称不能为空")
	}
	if utf8.RuneCountInString(name) > max {
		return "", fmt.Errorf("名称不能超过 %d 个字符", max)
	}
	return name, nil
}

func (s *Server) createGroup(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := readJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	name, err := cleanName(body.Name, 32)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	g := store.Group{ID: store.NewID(), Name: name}
	err = s.store.Update(func(d *store.Data) error {
		g.Order = len(d.Groups)
		d.Groups = append(d.Groups, g)
		return nil
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, groupView{Group: g, Items: []store.Item{}})
}

func (s *Server) updateGroup(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := readJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	name, err := cleanName(body.Name, 32)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	id := r.PathValue("id")
	err = s.store.Update(func(d *store.Data) error {
		for i := range d.Groups {
			if d.Groups[i].ID == id {
				d.Groups[i].Name = name
				return nil
			}
		}
		return errNotFound
	})
	respondUpdate(w, err)
}

func (s *Server) deleteGroup(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	err := s.store.Update(func(d *store.Data) error {
		for i := range d.Groups {
			if d.Groups[i].ID == id {
				d.Groups = append(d.Groups[:i], d.Groups[i+1:]...)
				return nil // 该分组下的卡片由 normalize 清理
			}
		}
		return errNotFound
	})
	if err == nil {
		s.gcUploads()
	}
	respondUpdate(w, err)
}

func (s *Server) orderGroups(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs []string `json:"ids"`
	}
	if err := readJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	err := s.store.Update(func(d *store.Data) error {
		pos := map[string]int{}
		for i, id := range body.IDs {
			pos[id] = i
		}
		for i := range d.Groups {
			if p, ok := pos[d.Groups[i].ID]; ok {
				d.Groups[i].Order = p
			} else {
				d.Groups[i].Order = len(body.IDs) + i
			}
		}
		return nil
	})
	respondUpdate(w, err)
}

type itemInput struct {
	GroupID  string     `json:"groupId"`
	Title    string     `json:"title"`
	Desc     string     `json:"desc"`
	URLLan   string     `json:"urlLan"`
	URLWan   string     `json:"urlWan"`
	Icon     store.Icon `json:"icon"`
	OpenMode string     `json:"openMode"`
}

var iconColors = map[string]bool{"": true, "blue": true, "teal": true, "amber": true, "violet": true, "rose": true, "green": true, "slate": true}

func (in *itemInput) validate() error {
	var err error
	if in.Title, err = cleanName(in.Title, 48); err != nil {
		return errors.New("标题" + strings.TrimPrefix(err.Error(), "名称"))
	}
	if utf8.RuneCountInString(in.Desc) > 120 {
		return errors.New("描述不能超过 120 个字符")
	}
	in.URLLan, in.URLWan = strings.TrimSpace(in.URLLan), strings.TrimSpace(in.URLWan)
	if in.URLLan == "" && in.URLWan == "" {
		return errors.New("内网地址和外网地址至少填写一个")
	}
	for _, u := range []string{in.URLLan, in.URLWan} {
		if err := validateLink(u); err != nil {
			return err
		}
	}
	switch in.Icon.Type {
	case "auto", "lucide", "text":
	case "image":
		if !strings.HasPrefix(in.Icon.Value, "/uploads/") && validateLink(in.Icon.Value) != nil {
			return errors.New("图标地址无效")
		}
	default:
		return errors.New("未知的图标类型")
	}
	if !iconColors[in.Icon.Color] {
		return errors.New("未知的图标颜色")
	}
	if in.OpenMode != "self" {
		in.OpenMode = "new"
	}
	return nil
}

// validateLink 允许 http/https 以及 smb、ssh 等常见协议，拒绝 javascript: 等可执行脚本的协议。
func validateLink(raw string) error {
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" {
		return fmt.Errorf("地址格式错误: %s（需包含协议，如 http://）", raw)
	}
	switch strings.ToLower(u.Scheme) {
	case "javascript", "data", "vbscript", "file":
		return fmt.Errorf("不允许的协议: %s", u.Scheme)
	}
	return nil
}

func (s *Server) createItem(w http.ResponseWriter, r *http.Request) {
	var in itemInput
	if err := readJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := in.validate(); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	now := time.Now()
	it := store.Item{
		ID: store.NewID(), GroupID: in.GroupID, Title: in.Title, Desc: in.Desc,
		URLLan: in.URLLan, URLWan: in.URLWan, Icon: in.Icon, OpenMode: in.OpenMode,
		CreatedAt: now, UpdatedAt: now,
	}
	err := s.store.Update(func(d *store.Data) error {
		if !hasGroup(d, in.GroupID) {
			return errors.New("分组不存在")
		}
		it.Order = 1 << 30 // 追加到末尾，由 normalize 重新编号
		d.Items = append(d.Items, it)
		return nil
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	s.store.View(func(d *store.Data) {
		for _, x := range d.Items {
			if x.ID == it.ID {
				it = x
			}
		}
	})
	writeJSON(w, http.StatusCreated, it)
}

func (s *Server) updateItem(w http.ResponseWriter, r *http.Request) {
	var in itemInput
	if err := readJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := in.validate(); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	id := r.PathValue("id")
	var out store.Item
	err := s.store.Update(func(d *store.Data) error {
		if !hasGroup(d, in.GroupID) {
			return errors.New("分组不存在")
		}
		for i := range d.Items {
			it := &d.Items[i]
			if it.ID != id {
				continue
			}
			if it.GroupID != in.GroupID {
				it.Order = 1 << 30 // 换分组后排到末尾
			}
			it.GroupID, it.Title, it.Desc = in.GroupID, in.Title, in.Desc
			it.URLLan, it.URLWan, it.Icon, it.OpenMode = in.URLLan, in.URLWan, in.Icon, in.OpenMode
			it.UpdatedAt = time.Now()
			out = *it
			return nil
		}
		return errNotFound
	})
	if err != nil {
		respondUpdate(w, err)
		return
	}
	s.gcUploads()
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) deleteItem(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	err := s.store.Update(func(d *store.Data) error {
		for i := range d.Items {
			if d.Items[i].ID == id {
				d.Items = append(d.Items[:i], d.Items[i+1:]...)
				return nil
			}
		}
		return errNotFound
	})
	if err == nil {
		s.gcUploads()
	}
	respondUpdate(w, err)
}

// layoutItems 接收拖拽后的完整布局，支持跨分组移动。
func (s *Server) layoutItems(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Groups []struct {
			ID      string   `json:"id"`
			ItemIDs []string `json:"itemIds"`
		} `json:"groups"`
	}
	if err := readJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	err := s.store.Update(func(d *store.Data) error {
		type pos struct {
			group string
			order int
		}
		want := map[string]pos{}
		for _, g := range body.Groups {
			if !hasGroup(d, g.ID) {
				return errors.New("分组不存在")
			}
			for i, id := range g.ItemIDs {
				want[id] = pos{g.ID, i}
			}
		}
		for i := range d.Items {
			if p, ok := want[d.Items[i].ID]; ok {
				d.Items[i].GroupID, d.Items[i].Order = p.group, p.order
			}
		}
		return nil
	})
	respondUpdate(w, err)
}

func hasGroup(d *store.Data, id string) bool {
	for _, g := range d.Groups {
		if g.ID == id {
			return true
		}
	}
	return false
}

var errNotFound = errors.New("记录不存在")

func respondUpdate(w http.ResponseWriter, err error) {
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case errors.Is(err, errNotFound):
		writeErr(w, http.StatusNotFound, err.Error())
	default:
		writeErr(w, http.StatusBadRequest, err.Error())
	}
}
