// Package rules 定义端口与服务指纹规则：内置规则来自 builtin.yaml，
// 用户可以覆盖内置规则或新增自定义规则，二者合并后用于扫描与识别。
package rules

import (
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

//go:embed builtin.yaml
var builtinYAML []byte

type Cond struct {
	Field string `yaml:"field" json:"field"`
	Op    string `yaml:"op,omitempty" json:"op,omitempty"`
	Value string `yaml:"value,omitempty" json:"value,omitempty"`
}

type Rule struct {
	ID       string `yaml:"id" json:"id"`
	Name     string `yaml:"name" json:"name"`
	Category string `yaml:"category" json:"category"`
	Ports    []int  `yaml:"ports,flow" json:"ports"`
	Proto    string `yaml:"proto" json:"proto"` // web | http | https | tcp
	Match    []Cond `yaml:"match,omitempty,flow" json:"match,omitempty"`
	All      bool   `yaml:"all,omitempty" json:"all,omitempty"`
	Probe    string `yaml:"probe,omitempty" json:"probe,omitempty"`
	Device   string `yaml:"device,omitempty" json:"device,omitempty"`
	System   bool   `yaml:"system,omitempty" json:"system,omitempty"`
	Icon     string `yaml:"icon,omitempty" json:"icon,omitempty"`
	Color    string `yaml:"color,omitempty" json:"color,omitempty"`
	URL      string `yaml:"url,omitempty" json:"url,omitempty"`
	Disabled bool   `yaml:"disabled,omitempty" json:"disabled,omitempty"`

	// 以下字段仅用于接口输出
	Builtin  bool `yaml:"-" json:"builtin"`
	Modified bool `yaml:"-" json:"modified"`
}

var Categories = []string{"nas", "virt", "router", "media", "download", "ops", "smarthome", "printcam", "base", "custom"}
var DeviceTypes = []string{"", "router", "switch", "ap", "nas", "server", "pc", "phone", "tv", "printer", "camera", "iot"}

var (
	builtinOnce sync.Once
	builtin     []Rule
	builtinErr  error
)

// Builtin 返回内置规则（只读副本）。
func Builtin() ([]Rule, error) {
	builtinOnce.Do(func() {
		builtin, builtinErr = ParseYAML(builtinYAML)
		for i := range builtin {
			builtin[i].Builtin = true
		}
	})
	out := make([]Rule, len(builtin))
	copy(out, builtin)
	return out, builtinErr
}

func ParseYAML(b []byte) ([]Rule, error) {
	var list []Rule
	if err := yaml.Unmarshal(b, &list); err != nil {
		return nil, fmt.Errorf("YAML 解析失败: %w", err)
	}
	seen := map[string]bool{}
	for i := range list {
		if err := Validate(&list[i]); err != nil {
			return nil, fmt.Errorf("第 %d 条规则（%s）: %w", i+1, list[i].ID, err)
		}
		if seen[list[i].ID] {
			return nil, fmt.Errorf("规则 ID 重复: %s", list[i].ID)
		}
		seen[list[i].ID] = true
	}
	return list, nil
}

func ExportYAML(list []Rule) ([]byte, error) {
	return yaml.Marshal(list)
}

var idRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,47}$`)

func oneOf(v string, set ...string) bool {
	for _, s := range set {
		if v == s {
			return true
		}
	}
	return false
}

// Validate 校验并规范化规则。
func Validate(r *Rule) error {
	r.ID = strings.TrimSpace(strings.ToLower(r.ID))
	r.Name = strings.TrimSpace(r.Name)
	if !idRe.MatchString(r.ID) {
		return errors.New("ID 只能包含小写字母、数字和 -，且不超过 48 个字符")
	}
	if r.Name == "" {
		return errors.New("名称不能为空")
	}
	if r.Category == "" {
		r.Category = "custom"
	}
	if !oneOf(r.Category, Categories...) {
		return fmt.Errorf("未知分类: %s", r.Category)
	}
	if r.Proto == "" {
		r.Proto = "web"
	}
	if !oneOf(r.Proto, "web", "http", "https", "tcp") {
		return fmt.Errorf("未知协议: %s", r.Proto)
	}
	if len(r.Ports) == 0 && len(r.Match) == 0 {
		return errors.New("端口和匹配条件至少填写一项")
	}
	for _, p := range r.Ports {
		if p < 1 || p > 65535 {
			return fmt.Errorf("端口超出范围: %d", p)
		}
	}
	if !oneOf(r.Device, DeviceTypes...) {
		return fmt.Errorf("未知设备类型: %s", r.Device)
	}
	for i := range r.Match {
		c := &r.Match[i]
		if c.Op == "" {
			c.Op = "contains"
		}
		if !oneOf(c.Op, "contains", "equals", "prefix", "regex", "exists") {
			return fmt.Errorf("未知匹配方式: %s", c.Op)
		}
		f := c.Field
		if !oneOf(f, "title", "body", "server", "banner", "favicon", "status") && !strings.HasPrefix(f, "header.") {
			return fmt.Errorf("未知匹配字段: %s", f)
		}
		if r.Proto == "tcp" && f != "banner" {
			return fmt.Errorf("TCP 规则只能匹配 banner，不能匹配 %s", f)
		}
		if c.Op == "regex" {
			if _, err := regexp.Compile("(?i)" + c.Value); err != nil {
				return fmt.Errorf("正则表达式错误: %w", err)
			}
		}
		if c.Op != "exists" && c.Value == "" {
			return fmt.Errorf("匹配条件 %s 的值不能为空", f)
		}
	}
	return nil
}

// Merge 合并内置规则与用户规则：同 ID 的用户规则覆盖内置规则，其余作为自定义规则追加。
func Merge(user []Rule) []Rule {
	base, _ := Builtin()
	idx := map[string]int{}
	for i, r := range base {
		idx[r.ID] = i
	}
	for _, u := range user {
		if i, ok := idx[u.ID]; ok {
			u.Builtin, u.Modified = true, true
			base[i] = u
		} else {
			u.Builtin, u.Modified = false, false
			base = append(base, u)
		}
	}
	return base
}

// Ports 返回启用规则涉及的全部端口（升序、去重）。
func Ports(list []Rule) []int {
	set := map[int]bool{}
	for _, r := range list {
		if r.Disabled {
			continue
		}
		for _, p := range r.Ports {
			set[p] = true
		}
	}
	out := make([]int, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.Ints(out)
	return out
}

// ---- 匹配 ----

// Response 是一次探测的结果，交给规则匹配。
type Response struct {
	Port    int
	Scheme  string // http | https | tcp
	Status  int
	Title   string
	Body    string
	Header  http.Header
	Banner  string
	Favicon string // mmh3 哈希（十进制字符串），需要时才计算
}

type compiled struct {
	Rule
	res []*regexp.Regexp
}

// Matcher 预编译规则，供扫描时高效匹配。
type Matcher struct {
	rules  []compiled
	byPort map[int][]int
}

func NewMatcher(list []Rule) *Matcher {
	m := &Matcher{byPort: map[int][]int{}}
	for _, r := range list {
		if r.Disabled {
			continue
		}
		c := compiled{Rule: r}
		for _, cond := range r.Match {
			var re *regexp.Regexp
			if cond.Op == "regex" {
				re, _ = regexp.Compile("(?i)" + cond.Value)
			}
			c.res = append(c.res, re)
		}
		m.rules = append(m.rules, c)
		for _, p := range r.Ports {
			m.byPort[p] = append(m.byPort[p], len(m.rules)-1)
		}
	}
	return m
}

// RulesForPort 返回在该端口上声明的规则，用于决定探测方式（http/https/tcp、probe）。
func (m *Matcher) RulesForPort(port int) []Rule {
	var out []Rule
	for _, i := range m.byPort[port] {
		out = append(out, m.rules[i].Rule)
	}
	return out
}

// NeedFavicon 判断是否有 Web 规则使用 favicon 匹配（有才去抓取 favicon）。
func (m *Matcher) NeedFavicon() bool {
	for _, r := range m.rules {
		for _, c := range r.Match {
			if c.Field == "favicon" {
				return true
			}
		}
	}
	return false
}

func (c *compiled) test(r *Response) bool {
	if len(c.Match) == 0 {
		return false
	}
	for i, cond := range c.Match {
		ok := evalCond(cond, c.res[i], r)
		if ok && !c.All {
			return true
		}
		if !ok && c.All {
			return false
		}
	}
	return c.All
}

func fieldValue(field string, r *Response) (string, bool) {
	switch field {
	case "title":
		return r.Title, true
	case "body":
		return r.Body, true
	case "server":
		return r.Header.Get("Server"), r.Header.Get("Server") != ""
	case "banner":
		return r.Banner, r.Banner != ""
	case "favicon":
		return r.Favicon, r.Favicon != ""
	case "status":
		return strconv.Itoa(r.Status), r.Status != 0
	}
	if name, ok := strings.CutPrefix(field, "header."); ok {
		vals, exists := r.Header[http.CanonicalHeaderKey(name)]
		return strings.Join(vals, ", "), exists
	}
	return "", false
}

func evalCond(c Cond, re *regexp.Regexp, r *Response) bool {
	v, exists := fieldValue(c.Field, r)
	switch c.Op {
	case "exists":
		return exists
	case "equals":
		return strings.EqualFold(strings.TrimSpace(v), c.Value)
	case "prefix":
		return strings.HasPrefix(strings.ToLower(v), strings.ToLower(c.Value))
	case "regex":
		return re != nil && re.MatchString(v)
	default:
		return strings.Contains(strings.ToLower(v), strings.ToLower(c.Value))
	}
}

// Identify 识别一次探测结果，返回命中的规则。
// 优先级：本端口声明且条件命中 > 其他端口的条件命中（应用映射到非标准端口）> 本端口的纯端口规则。
func (m *Matcher) Identify(r *Response) (Rule, bool) {
	isWeb := r.Scheme == "http" || r.Scheme == "https"
	onPort := map[int]bool{}
	for _, i := range m.byPort[r.Port] {
		onPort[i] = true
		c := &m.rules[i]
		if c.proto() == isWeb && c.test(r) {
			return c.Rule, true
		}
	}
	if isWeb {
		for i := range m.rules {
			c := &m.rules[i]
			if !onPort[i] && c.proto() && c.test(r) {
				return c.Rule, true
			}
		}
	}
	for _, i := range m.byPort[r.Port] {
		c := &m.rules[i]
		if len(c.Match) == 0 && c.proto() == isWeb {
			return c.Rule, true
		}
	}
	return Rule{}, false
}

// proto 返回规则是否为 Web 类规则。
func (c *compiled) proto() bool { return c.Proto != "tcp" }

// ---- favicon 哈希（与 Shodan / FOFA 的 icon_hash 一致）----

// FaviconHash 计算 mmh3(base64(favicon))，base64 按 76 字符换行，与 Python base64.encodebytes 一致。
func FaviconHash(data []byte) string {
	enc := base64.StdEncoding.EncodeToString(data)
	var b strings.Builder
	for i := 0; i < len(enc); i += 76 {
		end := min(i+76, len(enc))
		b.WriteString(enc[i:end])
		b.WriteByte('\n')
	}
	return strconv.Itoa(int(int32(murmur3([]byte(b.String()), 0))))
}

func murmur3(data []byte, seed uint32) uint32 {
	const c1, c2 = 0xcc9e2d51, 0x1b873593
	h := seed
	n := len(data) / 4
	for i := 0; i < n; i++ {
		k := uint32(data[i*4]) | uint32(data[i*4+1])<<8 | uint32(data[i*4+2])<<16 | uint32(data[i*4+3])<<24
		k *= c1
		k = k<<15 | k>>17
		k *= c2
		h ^= k
		h = h<<13 | h>>19
		h = h*5 + 0xe6546b64
	}
	tail := data[n*4:]
	var k uint32
	switch len(tail) {
	case 3:
		k ^= uint32(tail[2]) << 16
		fallthrough
	case 2:
		k ^= uint32(tail[1]) << 8
		fallthrough
	case 1:
		k ^= uint32(tail[0])
		k *= c1
		k = k<<15 | k>>17
		k *= c2
		h ^= k
	}
	h ^= uint32(len(data))
	h ^= h >> 16
	h *= 0x85ebca6b
	h ^= h >> 13
	h *= 0xc2b2ae35
	h ^= h >> 16
	return h
}

// ServiceURL 生成服务地址。
func ServiceURL(r Rule, scheme, ip string, port int) string {
	tpl := r.URL
	if tpl == "" {
		if scheme != "http" && scheme != "https" {
			return ""
		}
		tpl = "{scheme}://{ip}:{port}"
		if (scheme == "http" && port == 80) || (scheme == "https" && port == 443) {
			tpl = "{scheme}://{ip}"
		}
	}
	if strings.Contains(ip, ":") { // IPv6
		ip = "[" + ip + "]"
	}
	return strings.NewReplacer("{scheme}", scheme, "{ip}", ip, "{port}", strconv.Itoa(port)).Replace(tpl)
}
