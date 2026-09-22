package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSendFormats(t *testing.T) {
	var got map[string]any
	var ctype string
	reply := `{"errcode":0}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctype = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		got = nil
		_ = json.Unmarshal(b, &got)
		_, _ = io.WriteString(w, reply)
	}))
	defer srv.Close()
	m := Message{Title: "LanPanel 设备动态", Lines: []string{"新设备：iPhone（192.168.1.8）"}}

	c := Config{Type: "wecom", URL: srv.URL}
	if err := Send(context.Background(), c, m); err != nil {
		t.Fatal(err)
	}
	if got["msgtype"] != "text" || !strings.Contains(got["text"].(map[string]any)["content"].(string), "iPhone") {
		t.Fatalf("企业微信格式错误: %v", got)
	}

	c.Type = "feishu"
	reply = `{"code":0}`
	if err := Send(context.Background(), c, m); err != nil || got["msg_type"] != "text" {
		t.Fatalf("飞书格式错误: %v %v", err, got)
	}

	c.Type = "generic"
	if err := Send(context.Background(), c, m); err != nil || got["source"] != "lanpanel" {
		t.Fatalf("通用格式错误: %v %v", err, got)
	}

	c.Type = "serverchan"
	if err := Send(context.Background(), c, m); err != nil || !strings.HasPrefix(ctype, "application/x-www-form-urlencoded") {
		t.Fatalf("Server 酱格式错误: %v %s", err, ctype)
	}

	// 业务错误码
	c.Type, reply = "dingtalk", `{"errcode":310000,"errmsg":"keywords not in content"}`
	if err := Send(context.Background(), c, m); err == nil || !strings.Contains(err.Error(), "keywords") {
		t.Fatalf("应返回钉钉业务错误: %v", err)
	}
}
