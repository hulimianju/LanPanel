package discovery

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"lanpanel/internal/rules"
)

// 真实网络扫描：LANPANEL_LIVE=1 go test ./internal/discovery -run TestLiveScan -v
func TestLiveScan(t *testing.T) {
	if os.Getenv("LANPANEL_LIVE") == "" {
		t.Skip("设置 LANPANEL_LIVE=1 才会扫描真实网络")
	}
	st, err := OpenDeviceStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig(false)
	sc := NewScanner(st, func() Config { return cfg }, func() []rules.Rule { r, _ := rules.Builtin(); return r }, Hooks{})
	if err := sc.Start("full", "manual"); err != nil {
		t.Fatal(err)
	}
	last := 0
	for {
		time.Sleep(500 * time.Millisecond)
		for _, l := range sc.Logs(last) {
			last = l.Seq
			if l.Kind != "ARP" {
				fmt.Printf("%s %-5s %-22s %s\n", l.Time.Format("15:04:05"), l.Kind, l.Target, l.Msg)
			}
		}
		if !sc.Status().Running {
			break
		}
	}
	fmt.Println("\n==== 设备 ====")
	for _, d := range st.List() {
		var svs []string
		for _, s := range d.Services {
			svs = append(svs, fmt.Sprintf("%s:%d", s.Name, s.Port))
		}
		fmt.Printf("%-15s %-17s %-8s %-22.22s %-24.24s %-12.12s %v\n  服务: %s\n", d.IP, d.MAC, d.Type, d.Hostname, d.Vendor, d.OS, d.Sources, strings.Join(svs, ", "))
	}
}
