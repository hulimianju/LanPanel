// Package auth 提供密码哈希、无状态签名会话与登录限流。
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const CookieName = "lp_session"

func HashPassword(pw string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	return string(h), err
}

func CheckPassword(hash, pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}

// Sign 生成 "用户名|过期时间|密码哈希指纹" 的 HMAC 签名令牌。
// 指纹来自密码哈希，修改密码后旧会话全部失效。
func Sign(secret, username, pwHash string, ttl time.Duration) string {
	exp := strconv.FormatInt(time.Now().Add(ttl).Unix(), 10)
	payload := username + "|" + exp + "|" + fingerprint(pwHash)
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + mac(secret, payload)
}

// Verify 校验令牌，返回用户名；lookup 用于取当前密码哈希。
func Verify(secret, token string, lookup func(username string) (string, bool)) (string, error) {
	b64, sig, ok := strings.Cut(token, ".")
	if !ok {
		return "", errors.New("格式错误")
	}
	raw, err := base64.RawURLEncoding.DecodeString(b64)
	if err != nil {
		return "", err
	}
	payload := string(raw)
	if !hmac.Equal([]byte(sig), []byte(mac(secret, payload))) {
		return "", errors.New("签名无效")
	}
	parts := strings.Split(payload, "|")
	if len(parts) != 3 {
		return "", errors.New("格式错误")
	}
	exp, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || time.Now().Unix() > exp {
		return "", errors.New("会话已过期")
	}
	hash, ok := lookup(parts[0])
	if !ok || fingerprint(hash) != parts[2] {
		return "", errors.New("会话已失效")
	}
	return parts[0], nil
}

func mac(secret, payload string) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

func fingerprint(hash string) string {
	sum := sha256.Sum256([]byte(hash))
	return base64.RawURLEncoding.EncodeToString(sum[:6])
}

func SetCookie(w http.ResponseWriter, r *http.Request, token string, ttl time.Duration, persistent bool) {
	c := &http.Cookie{
		Name: CookieName, Value: token, Path: "/", HttpOnly: true,
		SameSite: http.SameSiteLaxMode, Secure: r.TLS != nil,
	}
	if persistent {
		c.MaxAge = int(ttl.Seconds())
	}
	http.SetCookie(w, c)
}

func ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: CookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true})
}

// Limiter 按来源 IP 限制登录失败次数：窗口内失败 max 次后锁定 window。
type Limiter struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	fails  map[string][]time.Time
}

func NewLimiter(max int, window time.Duration) *Limiter {
	return &Limiter{max: max, window: window, fails: map[string][]time.Time{}}
}

func (l *Limiter) prune(key string, now time.Time) []time.Time {
	list := l.fails[key]
	kept := list[:0]
	for _, t := range list {
		if now.Sub(t) < l.window {
			kept = append(kept, t)
		}
	}
	if len(kept) == 0 {
		delete(l.fails, key)
		return nil
	}
	l.fails[key] = kept
	return kept
}

func (l *Limiter) Blocked(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.prune(key, time.Now())) >= l.max
}

func (l *Limiter) Fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	l.fails[key] = append(l.prune(key, now), now)
}

func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.fails, key)
}
