package discovery

import (
	"net"
	"testing"
	"time"
)

func obsOf(ip, mac string) *hostObs {
	o := newObs(ip)
	o.MAC = mac
	return o
}

func TestMergeIPChangeAndOffline(t *testing.T) {
	st, err := OpenDeviceStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_, lan, _ := net.ParseCIDR("192.168.1.0/24")
	t0 := time.Now()

	// 第一次扫描：作为基准，不标记为新设备
	r := st.merge(map[string]*hostObs{
		"192.168.1.23": obsOf("192.168.1.23", "3C:7C:3F:A1:22:9E"),
		"192.168.1.60": obsOf("192.168.1.60", "a0:8c:fd:3f:2a:1b"),
	}, []*net.IPNet{lan}, true, t0)
	if r.New != 2 || st.Summary().New != 0 {
		t.Fatalf("首次扫描应作为基准：new=%d summary.new=%d", r.New, st.Summary().New)
	}

	// 第二次扫描：NAS 换了 IP，打印机离线，出现一台新手机
	r = st.merge(map[string]*hostObs{
		"192.168.1.105": obsOf("192.168.1.105", "3c:7c:3f:a1:22:9e"),
		"192.168.1.88":  obsOf("192.168.1.88", "d8:bb:c1:5a:03:e2"),
	}, []*net.IPNet{lan}, true, t0.Add(time.Hour))
	if r.IPChanged != 1 || len(r.Changes) != 1 || r.Changes[0].OldIP != "192.168.1.23" || r.Changes[0].NewIP != "192.168.1.105" {
		t.Fatalf("IP 变化检测错误: %+v", r)
	}
	nas, _ := st.FindByMAC("3C-7C-3F-A1-22-9E")
	if nas.IP != "192.168.1.105" || len(nas.IPHistory) != 2 || nas.IPHistory[0].To == nil || nas.IPHistory[1].To != nil {
		t.Fatalf("IP 历史错误: %+v", nas.IPHistory)
	}
	printer, _ := st.FindByMAC("a0:8c:fd:3f:2a:1b")
	if printer.Online {
		t.Fatal("未出现的设备应标记为离线")
	}
	if st.Summary().New != 1 {
		t.Fatalf("基准之后出现的设备应为新设备，实际 %d", st.Summary().New)
	}

	// 跨网段设备先按 IP 记录，拿到 MAC 后迁移并保留用户命名
	_, other, _ := net.ParseCIDR("10.0.0.0/24")
	st.merge(map[string]*hostObs{"10.0.0.5": obsOf("10.0.0.5", "")}, []*net.IPNet{other}, true, t0.Add(2*time.Hour))
	if _, err := st.Edit("ip:10.0.0.5", func(d *Device) { d.Name = "机房交换机" }); err != nil {
		t.Fatal(err)
	}
	st.merge(map[string]*hostObs{"10.0.0.5": obsOf("10.0.0.5", "00:11:22:33:44:55")}, []*net.IPNet{other}, true, t0.Add(3*time.Hour))
	sw, ok := st.FindByMAC("00:11:22:33:44:55")
	if !ok || sw.Name != "机房交换机" {
		t.Fatalf("按 IP 记录的设备未迁移到 MAC: %+v", sw)
	}
	if _, ok := st.Get("ip:10.0.0.5"); ok {
		t.Fatal("旧的 IP 键应被移除")
	}
}
