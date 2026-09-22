// Package icon 从目标网站抓取标题与图标，用于自动填充面板卡片。
// 局域网设备大多是自签证书，因此跳过证书校验；该功能仅管理员可用。
package icon

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const maxBody = 1 << 20 // 1MB

var client = &http.Client{
	Timeout: 8 * time.Second,
	Transport: &http.Transport{
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // 局域网自签证书
		ResponseHeaderTimeout: 6 * time.Second,
		MaxIdleConns:          4,
	},
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("重定向次数过多")
		}
		return nil
	},
}

type Result struct {
	Title    string
	Icon     []byte
	IconType string // MIME
}

type candidate struct {
	href  string
	score int
}

// Fetch 抓取页面标题和最合适的图标。图标获取失败不视为错误。
func Fetch(ctx context.Context, rawURL string) (*Result, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, errors.New("地址必须以 http:// 或 https:// 开头")
	}
	body, final, err := get(ctx, u.String())
	if err != nil {
		return nil, err
	}
	title, cands := parse(body)
	res := &Result{Title: title}

	base := final
	cands = append(cands, candidate{href: "/favicon.ico", score: 1})
	best := pickOrder(cands)
	for _, c := range best {
		ref, err := url.Parse(c.href)
		if err != nil {
			continue
		}
		iconURL := base.ResolveReference(ref)
		if iconURL.Scheme == "data" {
			continue
		}
		data, _, err := get(ctx, iconURL.String())
		if err != nil || len(data) == 0 {
			continue
		}
		mime := sniff(data)
		if mime == "" {
			continue
		}
		res.Icon, res.IconType = data, mime
		break
	}
	return res, nil
}

func get(ctx context.Context, u string) ([]byte, *url.URL, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (LanPanel)")
	req.Header.Set("Accept", "text/html,image/*,*/*;q=0.8")
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, nil, errors.New("HTTP " + strconv.Itoa(resp.StatusCode))
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	return data, resp.Request.URL, err
}

// parse 提取 <title> 与所有图标链接，按类型和尺寸打分。
func parse(body []byte) (string, []candidate) {
	z := html.NewTokenizer(bytes.NewReader(body))
	var title string
	var cands []candidate
	inTitle := false
	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			return strings.TrimSpace(title), cands
		case html.StartTagToken, html.SelfClosingTagToken:
			name, hasAttr := z.TagName()
			tag := string(name)
			if tag == "title" && title == "" {
				inTitle = true
			}
			if tag == "body" {
				return strings.TrimSpace(title), cands
			}
			if tag != "link" || !hasAttr {
				continue
			}
			var rel, href, sizes string
			for {
				k, v, more := z.TagAttr()
				switch string(k) {
				case "rel":
					rel = strings.ToLower(string(v))
				case "href":
					href = string(v)
				case "sizes":
					sizes = string(v)
				}
				if !more {
					break
				}
			}
			if href == "" || !strings.Contains(rel, "icon") {
				continue
			}
			score := 10
			if strings.Contains(rel, "apple-touch-icon") {
				score = 40
			}
			if strings.HasSuffix(strings.ToLower(href), ".svg") {
				score = 50
			}
			if w, _, ok := strings.Cut(sizes, "x"); ok {
				if n, err := strconv.Atoi(w); err == nil {
					score += min(n, 256) / 8
				}
			}
			cands = append(cands, candidate{href: href, score: score})
		case html.TextToken:
			if inTitle {
				title += string(z.Text())
			}
		case html.EndTagToken:
			if name, _ := z.TagName(); string(name) == "title" {
				inTitle = false
			}
		}
	}
}

func pickOrder(c []candidate) []candidate {
	out := append([]candidate(nil), c...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].score > out[j-1].score; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// sniff 仅接受常见图片格式，返回 MIME；不是图片返回空串。
func sniff(b []byte) string {
	switch {
	case bytes.HasPrefix(b, []byte("\x89PNG")):
		return "image/png"
	case bytes.HasPrefix(b, []byte("\xff\xd8\xff")):
		return "image/jpeg"
	case bytes.HasPrefix(b, []byte("GIF8")):
		return "image/gif"
	case len(b) > 12 && string(b[:4]) == "RIFF" && string(b[8:12]) == "WEBP":
		return "image/webp"
	case bytes.HasPrefix(b, []byte{0, 0, 1, 0}):
		return "image/x-icon"
	}
	head := strings.ToLower(string(b[:min(len(b), 512)]))
	if strings.Contains(head, "<svg") {
		return "image/svg+xml"
	}
	return ""
}

// Sniff 供上传接口复用。
func Sniff(b []byte) string { return sniff(b) }
