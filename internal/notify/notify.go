// Package notify 把设备动态推送到常见的消息渠道：企业微信、钉钉、飞书群机器人，
// Bark、Server 酱，以及通用 Webhook（可对接 Home Assistant、Node-RED 等）。
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Config struct {
	Enabled bool   `json:"enabled"`
	Type    string `json:"type"` // generic | wecom | dingtalk | feishu | bark | serverchan
	URL     string `json:"url"`
	Events  struct {
		New       bool `json:"new"`
		IPChanged bool `json:"ipChanged"`
		Offline   bool `json:"offline"`
		Online    bool `json:"online"`
	} `json:"events"`
}

func DefaultConfig() Config {
	c := Config{Type: "wecom"}
	c.Events.New, c.Events.IPChanged, c.Events.Offline, c.Events.Online = true, true, true, true
	return c
}

var Types = []string{"generic", "wecom", "dingtalk", "feishu", "bark", "serverchan"}

// Wants 判断该类型的动态是否需要推送。
func (c Config) Wants(eventType string) bool {
	switch eventType {
	case "new":
		return c.Events.New
	case "ip_changed":
		return c.Events.IPChanged
	case "offline":
		return c.Events.Offline
	case "online":
		return c.Events.Online
	}
	return false
}

func (c *Config) Validate() error {
	ok := false
	for _, t := range Types {
		if c.Type == t {
			ok = true
		}
	}
	if !ok {
		return errors.New("未知的推送方式")
	}
	c.URL = strings.TrimSpace(c.URL)
	if c.Enabled || c.URL != "" {
		u, err := url.Parse(c.URL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return errors.New("推送地址需以 http:// 或 https:// 开头")
		}
	}
	return nil
}

// Message 是一条推送消息；Lines 为逐条动态，Data 附带原始数据（仅通用 Webhook 使用）。
type Message struct {
	Title string
	Lines []string
	Data  any
}

func (m Message) Text() string {
	return strings.Join(m.Lines, "\n")
}

var client = &http.Client{Timeout: 10 * time.Second}

// Send 按配置的渠道发送消息。
func Send(ctx context.Context, c Config, m Message) error {
	var (
		body        []byte
		contentType = "application/json"
		err         error
	)
	full := m.Title + "\n" + m.Text()
	switch c.Type {
	case "wecom":
		body, err = json.Marshal(map[string]any{"msgtype": "text", "text": map[string]string{"content": full}})
	case "dingtalk":
		// 钉钉机器人若设置了"自定义关键词"，请包含 LanPanel
		body, err = json.Marshal(map[string]any{"msgtype": "text", "text": map[string]string{"content": full}})
	case "feishu":
		body, err = json.Marshal(map[string]any{"msg_type": "text", "content": map[string]string{"text": full}})
	case "bark":
		body, err = json.Marshal(map[string]string{"title": m.Title, "body": m.Text(), "group": "LanPanel"})
	case "serverchan":
		form := url.Values{"title": {m.Title}, "desp": {strings.ReplaceAll(m.Text(), "\n", "\n\n")}}
		body, contentType = []byte(form.Encode()), "application/x-www-form-urlencoded"
	default:
		body, err = json.Marshal(map[string]any{"source": "lanpanel", "title": m.Title, "text": m.Text(), "time": time.Now().Format(time.RFC3339), "data": m.Data})
	}
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("User-Agent", "LanPanel")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败：%w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("推送服务返回 HTTP %d：%s", resp.StatusCode, snippet(raw))
	}
	// 群机器人类接口即使失败也返回 200，需要检查业务错误码
	var r struct {
		ErrCode *int   `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		Code    *int   `json:"code"`
		Msg     string `json:"msg"`
		Message string `json:"message"`
	}
	if json.Unmarshal(raw, &r) == nil {
		if r.ErrCode != nil && *r.ErrCode != 0 {
			return fmt.Errorf("推送失败（%d）：%s", *r.ErrCode, r.ErrMsg)
		}
		if c.Type == "feishu" && r.Code != nil && *r.Code != 0 {
			return fmt.Errorf("推送失败（%d）：%s", *r.Code, r.Msg)
		}
		if c.Type == "bark" && r.Code != nil && *r.Code != 200 {
			return fmt.Errorf("推送失败（%d）：%s", *r.Code, r.Message)
		}
		if c.Type == "serverchan" && r.Code != nil && *r.Code != 0 {
			return fmt.Errorf("推送失败（%d）：%s", *r.Code, r.Message)
		}
	}
	return nil
}

func snippet(b []byte) string {
	s := strings.TrimSpace(string(b))
	if r := []rune(s); len(r) > 120 {
		s = string(r[:120]) + "…"
	}
	return s
}
