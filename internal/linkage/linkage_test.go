package linkage

import "testing"

func TestReplaceHost(t *testing.T) {
	cases := []struct{ in, ip, want string }{
		{"http://192.168.1.23:5666", "192.168.1.105", "http://192.168.1.105:5666"},
		{"http://192.168.1.23:5666/web/#/home?x=1", "192.168.1.105", "http://192.168.1.105:5666/web/#/home?x=1"},
		{"https://192.168.1.23", "10.0.0.2", "https://10.0.0.2"},
		{"smb://192.168.1.23/share", "192.168.1.9", "smb://192.168.1.9/share"},
		{"ssh://root@192.168.1.23:22", "192.168.1.9", "ssh://root@192.168.1.9:22"},
		{"http://nas.lan:5666", "192.168.1.9", "http://nas.lan:5666"}, // 域名不改
		{"http://[fe80::1]:80", "192.168.1.9", "http://[fe80::1]:80"}, // IPv6 不改
	}
	for _, c := range cases {
		got, _ := ReplaceHost(c.in, c.ip)
		if got != c.want {
			t.Errorf("ReplaceHost(%q) = %q，期望 %q", c.in, got, c.want)
		}
	}
}

func TestHostIPAndPort(t *testing.T) {
	if ip := HostIP("http://admin:pw@192.168.1.23:8080/x"); ip.String() != "192.168.1.23" {
		t.Errorf("HostIP 错误: %v", ip)
	}
	if HostIP("http://nas.lan") != nil {
		t.Error("域名不应解析出 IP")
	}
	for raw, want := range map[string]int{
		"http://192.168.1.23:5666/a": 5666, "https://192.168.1.23": 443, "http://192.168.1.23": 80,
		"smb://192.168.1.23": 445, "myapp://192.168.1.23": 0,
	} {
		if got := Port(raw); got != want {
			t.Errorf("Port(%q) = %d，期望 %d", raw, got, want)
		}
	}
}
