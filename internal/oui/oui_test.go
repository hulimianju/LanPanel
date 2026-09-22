package oui

import "testing"

func TestLookup(t *testing.T) {
	cases := map[string]string{
		"00:11:32:6B:4C:01": "Synology",
		"BC:AD:28:44:0C:9E": "Hikvision",
		"a0-8c-fd-3f-2a-1b": "Hewlett Packard",
	}
	for mac, want := range cases {
		got := Lookup(mac)
		if got == "" || !contains(got, want) {
			t.Errorf("Lookup(%s) = %q，期望包含 %q", mac, got, want)
		}
	}
	if !IsRandomized("DA:A1:19:00:00:01") || IsRandomized("00:11:32:00:00:01") {
		t.Error("随机 MAC 判断错误")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
