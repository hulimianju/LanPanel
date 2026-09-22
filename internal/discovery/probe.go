package discovery

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"html"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"

	"lanpanel/internal/rules"
)

// portScan 对在线主机做 TCP 连接扫描，返回 IP → 开放端口。
func portScan(ctx context.Context, hosts []string, ports []int, conc int, timeout time.Duration, progress func()) map[string][]int {
	open := map[string][]int{}
	var mu sync.Mutex
	sem := make(chan struct{}, max(conc, 1))
	var wg sync.WaitGroup
	d := &net.Dialer{Timeout: timeout}
outer:
	for _, ip := range hosts {
		for _, p := range ports {
			select {
			case <-ctx.Done():
				break outer
			case sem <- struct{}{}:
			}
			wg.Add(1)
			go func(ip string, p int) {
				defer wg.Done()
				defer func() { <-sem }()
				c, err := d.DialContext(ctx, "tcp4", net.JoinHostPort(ip, itoa(p)))
				if err == nil {
					c.Close()
					mu.Lock()
					open[ip] = append(open[ip], p)
					mu.Unlock()
				}
				progress()
			}(ip, p)
		}
	}
	wg.Wait()
	return open
}

// ---- 指纹识别 ----

type prober struct {
	matcher *rules.Matcher
	client  *http.Client
	timeout time.Duration
}

func newProber(m *rules.Matcher, timeout time.Duration) *prober {
	tr := &http.Transport{
		TLSClientConfig:        &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // 局域网设备多为自签证书
		DialContext:            (&net.Dialer{Timeout: 2 * time.Second}).DialContext,
		TLSHandshakeTimeout:    3 * time.Second,
		ResponseHeaderTimeout:  4 * time.Second,
		DisableKeepAlives:      true,
		MaxResponseHeaderBytes: 64 << 10,
	}
	p := &prober{matcher: m, timeout: timeout}
	p.client = &http.Client{
		Transport: tr,
		Timeout:   6 * time.Second,
		// 只跟随同主机的跳转（很多应用把 / 重定向到 /login），最多 3 次
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 || req.URL.Hostname() != via[0].URL.Hostname() {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
	return p
}

var titleRe = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
var charsetRe = regexp.MustCompile(`(?i)charset=["']?([\w-]+)`)
var spaceRe = regexp.MustCompile(`\s+`)

// decodeBody 处理 GBK 编码的国产设备页面。
func decodeBody(b []byte, contentType string) string {
	cs := strings.ToLower(firstGroup(charsetRe, contentType))
	if cs == "" {
		head := b[:min(len(b), 2048)]
		cs = strings.ToLower(firstGroup(charsetRe, string(head)))
	}
	if strings.HasPrefix(cs, "gb") || (!utf8.Valid(b) && cs != "utf-8") {
		if out, err := simplifiedchinese.GB18030.NewDecoder().Bytes(b); err == nil {
			return string(out)
		}
	}
	return string(b)
}

func extractTitle(body string) string {
	t := html.UnescapeString(firstGroup(titleRe, body))
	t = strings.TrimSpace(spaceRe.ReplaceAllString(t, " "))
	if r := []rune(t); len(r) > 80 {
		t = string(r[:80])
	}
	return t
}

// fetchHTTP 抓取首页，返回用于匹配的响应。
func (p *prober) fetchHTTP(ctx context.Context, scheme, ip string, port int) (*rules.Response, error) {
	u := scheme + "://" + net.JoinHostPort(ip, itoa(port)) + "/"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (LanPanel Discovery)")
	req.Header.Set("Accept", "text/html,*/*;q=0.8")
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 128<<10))
	body := decodeBody(raw, resp.Header.Get("Content-Type"))
	r := &rules.Response{Port: port, Scheme: scheme, Status: resp.StatusCode, Header: resp.Header, Body: body, Title: extractTitle(body)}
	// HTTP 请求打到 HTTPS 端口：nginx 等会返回 400 并提示
	if scheme == "http" && resp.StatusCode == 400 && (strings.Contains(body, "HTTPS port") || strings.Contains(body, "plain HTTP") || strings.Contains(body, "TLS")) {
		return r, errWrongScheme
	}
	return r, nil
}

var realmRe = regexp.MustCompile(`(?i)realm="([^"]+)"`)

var errWrongScheme = errors.New("协议不匹配")

func (p *prober) favicon(ctx context.Context, scheme, ip string, port int) string {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, scheme+"://"+net.JoinHostPort(ip, itoa(port))+"/favicon.ico", nil)
	resp, err := p.client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return ""
	}
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 256<<10))
	if len(b) == 0 {
		return ""
	}
	return rules.FaviconHash(b)
}

// banner 读取 TCP 服务的欢迎信息；probe 非空时先发送探测内容。
func (p *prober) banner(ctx context.Context, ip string, port int, probe string) string {
	c, err := (&net.Dialer{Timeout: p.timeout}).DialContext(ctx, "tcp4", net.JoinHostPort(ip, itoa(port)))
	if err != nil {
		return ""
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(1500 * time.Millisecond))
	if probe != "" {
		probe = strings.NewReplacer(`\r`, "\r", `\n`, "\n", "{ip}", ip, "{port}", itoa(port)).Replace(probe)
		_, _ = c.Write([]byte(probe))
	}
	buf := make([]byte, 512)
	n, _ := io.ReadAtLeast(c, buf, 1)
	return sanitize(buf[:n])
}

func sanitize(b []byte) string {
	b = bytes.Map(func(r rune) rune {
		if r == '\r' || r == '\n' || r == '\t' {
			return ' '
		}
		if r < 32 || r == utf8.RuneError {
			return -1
		}
		return r
	}, b)
	return strings.TrimSpace(string(b))
}

// schemes 根据端口上声明的规则决定探测顺序。
func (p *prober) schemes(port int) (order []string, tcpOnly bool, probe string) {
	rs := p.matcher.RulesForPort(port)
	hasWeb, hasTCP := false, false
	for _, r := range rs {
		switch r.Proto {
		case "https":
			hasWeb = true
			order = appendUnique(order, "https")
		case "http", "web":
			hasWeb = true
			order = appendUnique(order, "http")
		case "tcp":
			hasTCP = true
			if r.Probe != "" && probe == "" {
				probe = r.Probe
			}
		}
	}
	if !hasWeb && hasTCP {
		return nil, true, probe
	}
	// 未声明或声明为 web：先 http 再 https
	order = appendUnique(order, "http")
	order = appendUnique(order, "https")
	return order, false, probe
}

func appendUnique(s []string, v string) []string {
	for _, x := range s {
		if x == v {
			return s
		}
	}
	return append(s, v)
}

// identify 探测一个开放端口并识别服务。
func (p *prober) identify(ctx context.Context, ip string, port int, needFavicon bool) (Service, rules.Rule, bool) {
	resp := p.probe(ctx, ip, port, needFavicon)
	return p.toService(ip, port, resp)
}

// probe 按端口上声明的规则选择 http / https / tcp 进行探测。
func (p *prober) probe(ctx context.Context, ip string, port int, needFavicon bool) *rules.Response {
	order, tcpOnly, probe := p.schemes(port)
	var resp, fallback *rules.Response
	if !tcpOnly {
		for _, sch := range order {
			r, err := p.fetchHTTP(ctx, sch, ip, port)
			if err == nil {
				resp = r
				break
			}
			if r != nil && fallback == nil {
				fallback = r // 例如 HTTP 打到 HTTPS 端口返回的 400，HTTPS 也失败时仍可使用
			}
		}
		if resp == nil {
			resp = fallback
		}
	}
	if resp != nil && needFavicon {
		resp.Favicon = p.favicon(ctx, resp.Scheme, ip, port)
	}
	if resp == nil {
		resp = &rules.Response{Port: port, Scheme: "tcp", Header: http.Header{}, Banner: p.banner(ctx, ip, port, probe)}
	}
	return resp
}

func (p *prober) toService(ip string, port int, resp *rules.Response) (Service, rules.Rule, bool) {
	rule, ok := p.matcher.Identify(resp)
	sv := Service{Port: port, Proto: "tcp", Scheme: resp.Scheme, Source: "port", Title: resp.Title, Server: resp.Header.Get("Server")}
	if resp.Scheme == "tcp" {
		sv.Banner = resp.Banner
		if len(sv.Banner) > 120 {
			sv.Banner = sv.Banner[:120]
		}
	}
	if ok {
		sv.Name, sv.RuleID, sv.Category, sv.Icon, sv.Color = rule.Name, rule.ID, rule.Category, rule.Icon, rule.Color
		sv.URL = rules.ServiceURL(rule, resp.Scheme, ip, port)
	}
	if !ok || rule.ID == "web-generic" {
		realm := firstGroup(realmRe, resp.Header.Get("WWW-Authenticate"))
		switch {
		case resp.Scheme != "tcp" && resp.Status == 401 && realm != "":
			sv.Name = realm
		case resp.Scheme != "tcp" && resp.Status == 401:
			sv.Name = "网页服务（需登录）"
		case resp.Scheme != "tcp" && resp.Title != "":
			sv.Name = resp.Title
		case resp.Scheme != "tcp":
			sv.Name = "网页服务"
		default:
			sv.Name = "TCP " + itoa(port)
		}
		if resp.Scheme != "tcp" {
			sv.URL = rules.ServiceURL(rules.Rule{}, resp.Scheme, ip, port)
			if sv.Icon == "" {
				sv.Icon, sv.Color = "globe", "blue"
			}
		}
	}
	return sv, rule, ok
}

// TestResult 是规则测试的单端口结果。
type TestResult struct {
	Port     int    `json:"port"`
	Open     bool   `json:"open"`
	Scheme   string `json:"scheme,omitempty"`
	Status   int    `json:"status,omitempty"`
	Title    string `json:"title,omitempty"`
	Server   string `json:"server,omitempty"`
	Banner   string `json:"banner,omitempty"`
	Favicon  string `json:"favicon,omitempty"`
	Matched  bool   `json:"matched"`
	RuleID   string `json:"ruleId,omitempty"`
	RuleName string `json:"ruleName,omitempty"`
	URL      string `json:"url,omitempty"`
}

// TestPorts 用给定规则集探测单个主机的若干端口，返回原始响应与识别结果。
func TestPorts(ctx context.Context, ip string, ports []int, ruleList []rules.Rule, timeout time.Duration) []TestResult {
	m := rules.NewMatcher(ruleList)
	p := newProber(m, timeout)
	out := make([]TestResult, len(ports))
	var wg sync.WaitGroup
	for i, port := range ports {
		wg.Add(1)
		go func(i, port int) {
			defer wg.Done()
			res := TestResult{Port: port}
			c, err := (&net.Dialer{Timeout: timeout}).DialContext(ctx, "tcp4", net.JoinHostPort(ip, itoa(port)))
			if err != nil {
				out[i] = res
				return
			}
			c.Close()
			res.Open = true
			resp := p.probe(ctx, ip, port, true)
			sv, rule, ok := p.toService(ip, port, resp)
			res.Scheme, res.Status, res.Title, res.Server, res.Banner, res.Favicon = resp.Scheme, resp.Status, resp.Title, resp.Header.Get("Server"), resp.Banner, resp.Favicon
			res.Matched, res.URL = ok && rule.ID != "web-generic", sv.URL
			if ok {
				res.RuleID, res.RuleName = rule.ID, rule.Name
			}
			out[i] = res
		}(i, port)
	}
	wg.Wait()
	return out
}
