package rules

import (
	"net/http"
	"testing"
)

func TestBuiltinParses(t *testing.T) {
	list, err := Builtin()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) < 80 {
		t.Fatalf("内置规则数量过少: %d", len(list))
	}
	t.Logf("内置规则 %d 条，端口 %d 个", len(list), len(Ports(list)))
}

func TestMurmur3(t *testing.T) {
	// 与 Python mmh3.hash 对照
	for s, want := range map[string]int32{"hello": 613153351, "foo": -156908512, "": 0} {
		if got := int32(murmur3([]byte(s), 0)); got != want {
			t.Errorf("mmh3(%q) = %d，期望 %d", s, got, want)
		}
	}
}

func TestIdentify(t *testing.T) {
	list, _ := Builtin()
	m := NewMatcher(list)
	cases := []struct {
		resp Response
		want string
	}{
		{Response{Port: 5666, Scheme: "http", Title: "飞牛私有云 fnOS", Header: http.Header{}}, "fnos"},
		// 映射到非标准端口也能靠标题识别
		{Response{Port: 18096, Scheme: "http", Title: "Jellyfin", Header: http.Header{}}, "jellyfin"},
		{Response{Port: 9091, Scheme: "http", Header: http.Header{"X-Transmission-Session-Id": {"x"}}}, "transmission"},
		{Response{Port: 22, Scheme: "tcp", Banner: "SSH-2.0-OpenSSH_9.2", Header: http.Header{}}, "ssh"},
		{Response{Port: 445, Scheme: "tcp", Header: http.Header{}}, "smb"},
		// 无特征的网页落到通用规则
		{Response{Port: 8080, Scheme: "http", Title: "Welcome", Header: http.Header{}}, "web-generic"},
	}
	for _, c := range cases {
		r, ok := m.Identify(&c.resp)
		if !ok || r.ID != c.want {
			t.Errorf("端口 %d 标题 %q：识别为 %q，期望 %q", c.resp.Port, c.resp.Title, r.ID, c.want)
		}
	}
	if _, ok := m.Identify(&Response{Port: 12345, Scheme: "tcp", Header: http.Header{}}); ok {
		t.Error("未知端口不应识别")
	}
}

func TestMergeOverride(t *testing.T) {
	list := Merge([]Rule{{ID: "ssh", Name: "SSH", Category: "base", Ports: []int{22}, Proto: "tcp", Disabled: true}, {ID: "my-app", Name: "我的应用", Ports: []int{7777}}})
	var sshDisabled, custom bool
	for _, r := range list {
		if r.ID == "ssh" {
			sshDisabled = r.Disabled && r.Modified && r.Builtin
		}
		if r.ID == "my-app" {
			custom = !r.Builtin
		}
	}
	if !sshDisabled || !custom {
		t.Fatal("合并结果错误")
	}
}
