// Package oui 根据 MAC 地址前缀查询设备厂商。数据由 gen 从 Wireshark manuf 生成并嵌入。
package oui

import (
	"bufio"
	"bytes"
	"compress/gzip"
	_ "embed"
	"sort"
	"strconv"
	"strings"
	"sync"
)

//go:embed oui.tsv.gz
var data []byte

var (
	once     sync.Once
	prefixes []uint32 // 升序，二分查找
	names    []string
)

func load() {
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return
	}
	sc := bufio.NewScanner(zr)
	intern := map[string]string{} // 同一厂商有大量前缀，复用字符串节省内存
	for sc.Scan() {
		p, name, ok := strings.Cut(sc.Text(), "\t")
		if !ok {
			continue
		}
		v, err := strconv.ParseUint(p, 16, 32)
		if err != nil {
			continue
		}
		if s, ok := intern[name]; ok {
			name = s
		} else {
			intern[name] = name
		}
		prefixes = append(prefixes, uint32(v))
		names = append(names, name)
	}
}

// Lookup 返回 MAC 对应的厂商名；未知或随机 MAC 返回空串。
func Lookup(mac string) string {
	hexs := strings.NewReplacer(":", "", "-", "", ".", "").Replace(mac)
	if len(hexs) < 6 {
		return ""
	}
	v, err := strconv.ParseUint(hexs[:6], 16, 32)
	if err != nil {
		return ""
	}
	once.Do(load)
	i := sort.Search(len(prefixes), func(i int) bool { return prefixes[i] >= uint32(v) })
	if i < len(prefixes) && prefixes[i] == uint32(v) {
		return names[i]
	}
	return ""
}

// IsRandomized 判断是否为本地管理地址（手机、电脑的隐私随机 MAC）。
func IsRandomized(mac string) bool {
	hexs := strings.NewReplacer(":", "", "-", "").Replace(mac)
	if len(hexs) < 2 {
		return false
	}
	b, err := strconv.ParseUint(hexs[:2], 16, 8)
	return err == nil && b&0x02 != 0
}
