package store

import (
	"time"

	"lanpanel/internal/discovery"
	"lanpanel/internal/notify"
	"lanpanel/internal/rules"
)

// Data 是持久化到 data.json 的全部内容。
type Data struct {
	Version  int      `json:"version"`
	Secret   string   `json:"secret"` // 会话签名密钥，首次启动自动生成
	Users    []User   `json:"users"`
	Settings Settings `json:"settings"`
	Groups   []Group  `json:"groups"`
	Items    []Item   `json:"items"`

	Discovery discovery.Config `json:"discovery"`
	// RuleOverrides 保存用户修改过的内置规则与自定义规则（同 ID 覆盖内置规则）
	RuleOverrides []rules.Rule `json:"ruleOverrides"`
	// Notify 含推送地址等敏感信息，只通过管理接口读取，不随站点设置下发
	Notify notify.Config `json:"notify"`
}

type User struct {
	Username     string    `json:"username"`
	PasswordHash string    `json:"passwordHash"`
	CreatedAt    time.Time `json:"createdAt"`
}

type Group struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Order int    `json:"order"`
}

// Icon 描述应用卡片的图标来源。
type Icon struct {
	Type  string `json:"type"`            // auto | lucide | image | text
	Value string `json:"value"`           // lucide 图标名 / 图片地址 / 文字
	Color string `json:"color,omitempty"` // 色调（lucide 与 text 使用）
}

type Item struct {
	ID       string `json:"id"`
	GroupID  string `json:"groupId"`
	Title    string `json:"title"`
	Desc     string `json:"desc,omitempty"`
	URLLan   string `json:"urlLan"`
	URLWan   string `json:"urlWan,omitempty"`
	Icon     Icon   `json:"icon"`
	OpenMode string `json:"openMode"` // new | self
	Order    int    `json:"order"`
	// 阶段 3：按 MAC 绑定设备，IP 变化时自动改写地址
	DeviceMAC  string    `json:"deviceMac,omitempty"`
	DevicePort int       `json:"devicePort,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type Wallpaper struct {
	Type  string `json:"type"`  // none | image | color
	Value string `json:"value"` // 图片地址或颜色
	Blur  int    `json:"blur"`  // 0-40 px
	Dim   int    `json:"dim"`   // 遮罩强度 0-80 %
	Tone  string `json:"tone"`  // auto | light | dark：面板表面色调
	// 前端采样得到的壁纸亮度 0-1，-1 表示未知；Tone=auto 时据此决定色调
	Luminance float64 `json:"luminance"`
}

type Settings struct {
	SiteTitle       string    `json:"siteTitle"`
	Theme           string    `json:"theme"` // auto | light | dark：界面主题
	Wallpaper       Wallpaper `json:"wallpaper"`
	SearchEngine    string    `json:"searchEngine"` // bing | baidu | google | duckduckgo | custom
	CustomSearchURL string    `json:"customSearchUrl,omitempty"`
	ShowClock       bool      `json:"showClock"`
	ShowSeconds     bool      `json:"showSeconds"`
	ShowWidgets     bool      `json:"showWidgets"`
	AddressMode     string    `json:"addressMode"` // auto | lan | wan：默认地址模式
	PublicPanel     bool      `json:"publicPanel"` // 允许未登录访客只读浏览面板
	CardSize        string    `json:"cardSize"`    // comfortable | compact
}

func DefaultSettings() Settings {
	return Settings{
		SiteTitle:    "LanPanel",
		Theme:        "auto",
		Wallpaper:    Wallpaper{Type: "none", Blur: 0, Dim: 30, Tone: "auto", Luminance: -1},
		SearchEngine: "bing",
		ShowClock:    true,
		ShowWidgets:  true,
		AddressMode:  "auto",
		PublicPanel:  false,
		CardSize:     "comfortable",
	}
}
