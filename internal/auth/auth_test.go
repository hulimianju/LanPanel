package auth

import (
	"testing"
	"time"
)

func TestSignVerify(t *testing.T) {
	hash, _ := HashPassword("secret123")
	lookup := func(string) (string, bool) { return hash, true }
	tok := Sign("k", "admin", hash, time.Hour)
	if u, err := Verify("k", tok, lookup); err != nil || u != "admin" {
		t.Fatalf("有效令牌校验失败: %v", err)
	}
	if _, err := Verify("other", tok, lookup); err == nil {
		t.Fatal("密钥不同应校验失败")
	}
	if _, err := Verify("k", Sign("k", "admin", hash, -time.Second), lookup); err == nil {
		t.Fatal("过期令牌应校验失败")
	}
	newHash, _ := HashPassword("changed")
	if _, err := Verify("k", tok, func(string) (string, bool) { return newHash, true }); err == nil {
		t.Fatal("修改密码后旧令牌应失效")
	}
}

func TestLimiter(t *testing.T) {
	l := NewLimiter(3, time.Minute)
	for i := 0; i < 3; i++ {
		if l.Blocked("ip") {
			t.Fatal("未达上限不应锁定")
		}
		l.Fail("ip")
	}
	if !l.Blocked("ip") {
		t.Fatal("达到上限应锁定")
	}
	l.Reset("ip")
	if l.Blocked("ip") {
		t.Fatal("重置后应解锁")
	}
}
