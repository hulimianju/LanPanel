//go:build !linux

package discovery

import (
	"context"
	"errors"
	"net"
	"time"
)

func rawARPScan(ctx context.Context, ifi localIface, targets []net.IP, wait time.Duration) (map[string]string, error) {
	return nil, errors.New("当前系统不支持原始 ARP 扫描")
}
