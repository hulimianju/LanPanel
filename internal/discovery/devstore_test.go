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

func eventTypes(evs []Event) map[string]int {
	m := map[string]int{}
	for _, e := range evs {
		m[e.Type]++
	}
	return m
}

func TestMergeIPChangeOfflineAndEvents(t *testing.T) {
	st, err := OpenDeviceStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_, lan, _ := net.ParseCIDR("192.168.1.0/24")
	nets := []*net.IPNet{lan}
	t0 := time.Now()
	const nas, printer, phone = "3c:7c:3f:a1:22:9e", "a0:8c:fd:3f:2a:1b", "d8:bb:c1:5a:03:e2"
	watched := func(mac string) bool { return mac == printer } // 打印机绑定了面板卡片

	// 第一次扫描：作为基准，不产生"新设备"动态
	r := st.merge(map[string]*hostObs{
		"192.168.1.23": obsOf("192.168.1.23", "3C:7C:3F:A1:22:9E"),
		"192.168.1.60": obsOf("192.168.1.60", printer),
	}, nets, true, t0, watched)
	if r.New != 2 || len(r.Events) != 0 || st.Summary().New != 0 {
		t.Fatalf("首次扫描应作为基准：new=%d events=%v", r.New, r.Events)
	}

	// 第二次：NAS 换 IP、打印机未出现（第一次缺席，仍算在线）、新手机
	r = st.merge(map[string]*hostObs{
		"192.168.1.105": obsOf("192.168.1.105", nas),
		"192.168.1.88":  obsOf("192.168.1.88", phone),
	}, nets, true, t0.Add(time.Hour), watched)
	if et := eventTypes(r.Events); et["ip_changed"] != 1 || et["new"] != 1 || et["offline"] != 0 {
		t.Fatalf("第二次扫描动态错误: %+v", r.Events)
	}
	for _, e := range r.Events {
		if e.Type == "ip_changed" && (e.OldIP != "192.168.1.23" || e.IP != "192.168.1.105") {
			t.Fatalf("IP 变化动态错误: %+v", e)
		}
	}
	if p, _ := st.FindByMAC(printer); !p.Online || p.Missed != 1 {
		t.Fatalf("缺席一次不应判为离线: %+v", p)
	}
	n, _ := st.FindByMAC(nas)
	if n.IP != "192.168.1.105" || len(n.IPHistory) != 2 || n.IPHistory[0].To == nil || n.IPHistory[1].To != nil {
		t.Fatalf("IP 历史错误: %+v", n.IPHistory)
	}

	// 第三次：打印机连续两次缺席 → 离线，且因被关注而产生动态；手机未被关注，缺席不产生动态
	r = st.merge(map[string]*hostObs{"192.168.1.105": obsOf("192.168.1.105", nas)}, nets, true, t0.Add(2*time.Hour), watched)
	if et := eventTypes(r.Events); et["offline"] != 1 || len(r.Events) != 1 {
		t.Fatalf("第三次扫描动态错误: %+v", r.Events)
	}
	if p, _ := st.FindByMAC(printer); p.Online {
		t.Fatal("连续两次缺席应判为离线")
	}

	// 第四次：打印机恢复 → online 动态
	r = st.merge(map[string]*hostObs{"192.168.1.60": obsOf("192.168.1.60", printer)}, nets, true, t0.Add(3*time.Hour), watched)
	if et := eventTypes(r.Events); et["online"] != 1 {
		t.Fatalf("恢复在线动态缺失: %+v", r.Events)
	}

	// 动态保存与查询
	st.addEvents(r.Events)
	if evs := st.Events(10); len(evs) != 1 || evs[0].ID == 0 {
		t.Fatalf("动态保存错误: %+v", evs)
	}
	if d, ok := st.ByIP("192.168.1.105"); !ok || d.MAC != nas {
		t.Fatal("按 IP 查找设备失败")
	}
}

func TestMergeMigratesIPKeyToMAC(t *testing.T) {
	st, _ := OpenDeviceStore(t.TempDir())
	_, other, _ := net.ParseCIDR("10.0.0.0/24")
	t0 := time.Now()
	st.merge(map[string]*hostObs{"10.0.0.5": obsOf("10.0.0.5", "")}, []*net.IPNet{other}, true, t0, nil)
	if _, err := st.Edit("ip:10.0.0.5", func(d *Device) { d.Name = "机房交换机" }); err != nil {
		t.Fatal(err)
	}
	st.merge(map[string]*hostObs{"10.0.0.5": obsOf("10.0.0.5", "00:11:22:33:44:55")}, []*net.IPNet{other}, true, t0.Add(time.Hour), nil)
	sw, ok := st.FindByMAC("00:11:22:33:44:55")
	if !ok || sw.Name != "机房交换机" {
		t.Fatalf("按 IP 记录的设备未迁移到 MAC: %+v", sw)
	}
	if _, ok := st.Get("ip:10.0.0.5"); ok {
		t.Fatal("旧的 IP 键应被移除")
	}
}
