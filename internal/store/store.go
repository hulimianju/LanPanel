// Package store 实现基于单个 JSON 文件的持久化：全部数据常驻内存，
// 每次修改后原子写回磁盘（先写临时文件再 rename），并保留上一版 .bak。
package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

const currentVersion = 1

type Store struct {
	mu   sync.RWMutex
	path string
	data Data
}

func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	s := &Store{path: filepath.Join(dir, "data.json")}
	raw, err := os.ReadFile(s.path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		s.data = Data{Version: currentVersion, Settings: DefaultSettings()}
	case err != nil:
		return nil, err
	default:
		s.data.Settings = DefaultSettings() // 旧文件缺少的字段使用默认值
		if err := json.Unmarshal(raw, &s.data); err != nil {
			return nil, fmt.Errorf("解析 %s 失败: %w", s.path, err)
		}
	}
	if s.data.Secret == "" {
		s.data.Secret = RandomHex(32)
	}
	s.data.Version = currentVersion
	return s, s.saveLocked()
}

// View 以只读方式访问数据；回调内不得保留指针。
func (s *Store) View(fn func(d *Data)) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	fn(&s.data)
}

// Update 修改数据并持久化；回调返回错误时不写盘也不保留修改。
func (s *Store) Update(fn func(d *Data) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	backup, err := json.Marshal(s.data)
	if err != nil {
		return err
	}
	if err := fn(&s.data); err != nil {
		_ = json.Unmarshal(backup, &s.data)
		return err
	}
	normalize(&s.data)
	return s.saveLocked()
}

func (s *Store) saveLocked() error {
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(raw); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	f.Close()
	if _, err := os.Stat(s.path); err == nil {
		_ = os.Rename(s.path, s.path+".bak")
	}
	return os.Rename(tmp, s.path)
}

// normalize 让分组与卡片的 Order 保持 0..n-1 连续，并移除孤立卡片。
func normalize(d *Data) {
	sort.SliceStable(d.Groups, func(i, j int) bool { return d.Groups[i].Order < d.Groups[j].Order })
	valid := make(map[string]bool, len(d.Groups))
	for i := range d.Groups {
		d.Groups[i].Order = i
		valid[d.Groups[i].ID] = true
	}
	items := d.Items[:0]
	for _, it := range d.Items {
		if valid[it.GroupID] {
			items = append(items, it)
		}
	}
	d.Items = items
	sort.SliceStable(d.Items, func(i, j int) bool {
		if d.Items[i].GroupID != d.Items[j].GroupID {
			return d.Items[i].GroupID < d.Items[j].GroupID
		}
		return d.Items[i].Order < d.Items[j].Order
	})
	counter := map[string]int{}
	for i := range d.Items {
		d.Items[i].Order = counter[d.Items[i].GroupID]
		counter[d.Items[i].GroupID]++
	}
}

func RandomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func NewID() string { return RandomHex(8) }
