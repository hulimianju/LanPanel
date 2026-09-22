//go:build !linux

package sysinfo

// 非 Linux 平台（开发机）只提供主机名与本进程内存。
func readPlatform(info *Info) (*sample, bool) { return nil, false }
