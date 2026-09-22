// Package sysinfo 采集本机的 CPU / 内存 / 网络速率，用于面板状态卡片。
// 采样由调用方触发（两次调用之间计算速率），不启动后台协程。
package sysinfo

import (
	"os"
	"runtime"
	"sync"
	"time"
)

type Info struct {
	Hostname   string  `json:"hostname"`
	OS         string  `json:"os"`
	Arch       string  `json:"arch"`
	Uptime     int64   `json:"uptime"`
	CPUPercent float64 `json:"cpuPercent"` // -1 表示不可用
	MemTotal   uint64  `json:"memTotal"`
	MemUsed    uint64  `json:"memUsed"`
	NetRx      uint64  `json:"netRx"` // 字节/秒
	NetTx      uint64  `json:"netTx"`
	SelfMem    uint64  `json:"selfMem"` // 本程序占用的内存
}

type sample struct {
	at           time.Time
	cpuTotal     uint64
	cpuIdle      uint64
	netRx, netTx uint64
}

type Sampler struct {
	mu   sync.Mutex
	prev *sample
	last Info
}

func NewSampler() *Sampler { return &Sampler{} }

// Get 返回最新信息；1 秒内的重复调用直接返回缓存。
func (s *Sampler) Get() Info {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if s.prev != nil && now.Sub(s.prev.at) < time.Second {
		return s.last
	}
	host, _ := os.Hostname()
	info := Info{Hostname: host, OS: runtime.GOOS, Arch: runtime.GOARCH, CPUPercent: -1}
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	info.SelfMem = ms.Sys - ms.HeapReleased

	cur, ok := readPlatform(&info)
	if ok {
		cur.at = now
		if p := s.prev; p != nil {
			if dt := cur.cpuTotal - p.cpuTotal; dt > 0 && cur.cpuTotal >= p.cpuTotal {
				info.CPUPercent = float64(dt-(cur.cpuIdle-p.cpuIdle)) * 100 / float64(dt)
			}
			if sec := now.Sub(p.at).Seconds(); sec > 0 && cur.netRx >= p.netRx && cur.netTx >= p.netTx {
				info.NetRx = uint64(float64(cur.netRx-p.netRx) / sec)
				info.NetTx = uint64(float64(cur.netTx-p.netTx) / sec)
			}
		}
		s.prev = cur
	}
	s.last = info
	return info
}
