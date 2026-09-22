package discovery

import (
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

// SNMP v2c 查询（UDP 161）：读取 system 组的描述、厂商 OID、名称与服务层级。
// sysServices 按位标明设备工作的网络层级，是区分二层交换机与路由器的可靠依据。

var snmpOIDs = [][]int{
	{1, 3, 6, 1, 2, 1, 1, 1, 0}, // sysDescr
	{1, 3, 6, 1, 2, 1, 1, 2, 0}, // sysObjectID
	{1, 3, 6, 1, 2, 1, 1, 5, 0}, // sysName
	{1, 3, 6, 1, 2, 1, 1, 7, 0}, // sysServices
}

// 常见网络设备厂商的企业号（sysObjectID 前缀 1.3.6.1.4.1.<企业号>）
var snmpEnterprises = map[int]string{
	9: "Cisco", 11: "HP", 2011: "Huawei", 25506: "H3C", 4881: "Ruijie", 11863: "TP-Link", 4526: "Netgear",
	14988: "MikroTik", 41112: "Ubiquiti", 6574: "Synology", 24681: "QNAP", 8072: "Net-SNMP", 311: "Microsoft",
	12356: "Fortinet", 2636: "Juniper", 30065: "Arista", 674: "Dell", 171: "D-Link", 3902: "ZTE", 34592: "Maipu",
}

func berLen(n int) []byte {
	if n < 0x80 {
		return []byte{byte(n)}
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte(n)}, b...)
		n >>= 8
	}
	return append([]byte{0x80 | byte(len(b))}, b...)
}

func tlv(tag byte, content ...[]byte) []byte {
	var body []byte
	for _, c := range content {
		body = append(body, c...)
	}
	return append(append([]byte{tag}, berLen(len(body))...), body...)
}

func berInt(v int) []byte {
	b := []byte{byte(v)}
	for v > 0xff || v < -0x80 {
		v >>= 8
		b = append([]byte{byte(v)}, b...)
	}
	if b[0]&0x80 != 0 && v >= 0 {
		b = append([]byte{0}, b...)
	}
	return tlv(0x02, b)
}

func berOID(oid []int) []byte {
	b := []byte{byte(oid[0]*40 + oid[1])}
	for _, n := range oid[2:] {
		var enc []byte
		enc = append(enc, byte(n&0x7f))
		for n >>= 7; n > 0; n >>= 7 {
			enc = append([]byte{byte(0x80 | n&0x7f)}, enc...)
		}
		b = append(b, enc...)
	}
	return tlv(0x06, b)
}

func snmpGet(community string, reqID int) []byte {
	var vbs [][]byte
	for _, oid := range snmpOIDs {
		vbs = append(vbs, tlv(0x30, berOID(oid), []byte{0x05, 0x00}))
	}
	pdu := tlv(0xA0, berInt(reqID), berInt(0), berInt(0), tlv(0x30, vbs...))
	return tlv(0x30, berInt(1), tlv(0x04, []byte(community)), pdu)
}

// berRead 读取一个 TLV，返回标签、内容与剩余字节。
func berRead(b []byte) (byte, []byte, []byte, error) {
	if len(b) < 2 {
		return 0, nil, nil, errors.New("长度不足")
	}
	tag, l := b[0], int(b[1])
	off := 2
	if l&0x80 != 0 {
		n := l & 0x7f
		if n == 0 || n > 3 || len(b) < 2+n {
			return 0, nil, nil, errors.New("长度字段错误")
		}
		l = 0
		for i := 0; i < n; i++ {
			l = l<<8 | int(b[2+i])
		}
		off += n
	}
	if len(b) < off+l {
		return 0, nil, nil, errors.New("内容不完整")
	}
	return tag, b[off : off+l], b[off+l:], nil
}

func decodeOID(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	parts := []string{strconv.Itoa(int(b[0]) / 40), strconv.Itoa(int(b[0]) % 40)}
	v := 0
	for _, c := range b[1:] {
		v = v<<7 | int(c&0x7f)
		if c&0x80 == 0 {
			parts = append(parts, strconv.Itoa(v))
			v = 0
		}
	}
	return strings.Join(parts, ".")
}

type snmpInfo struct {
	Descr, ObjectID, Name string
	Services              int
}

func parseSNMP(b []byte) (snmpInfo, bool) {
	var info snmpInfo
	_, msg, _, err := berRead(b) // SEQUENCE
	if err != nil {
		return info, false
	}
	_, _, rest, err := berRead(msg) // version
	if err != nil {
		return info, false
	}
	_, _, rest, err = berRead(rest) // community
	if err != nil {
		return info, false
	}
	tag, pdu, _, err := berRead(rest)
	if err != nil || tag != 0xA2 {
		return info, false
	}
	var errStatus []byte
	_, _, pdu, _ = berRead(pdu)         // request-id
	_, errStatus, pdu, _ = berRead(pdu) // error-status
	_, _, pdu, _ = berRead(pdu)         // error-index
	_, vbs, _, err := berRead(pdu)      // varbind list
	if err != nil || (len(errStatus) > 0 && errStatus[0] != 0) {
		return info, false
	}
	for len(vbs) > 0 {
		var vb []byte
		_, vb, vbs, err = berRead(vbs)
		if err != nil {
			break
		}
		_, oidRaw, val, err := berRead(vb)
		if err != nil {
			continue
		}
		vtag, v, _, err := berRead(val)
		if err != nil {
			continue
		}
		switch decodeOID(oidRaw) {
		case "1.3.6.1.2.1.1.1.0":
			if vtag == 0x04 {
				info.Descr = strings.TrimSpace(string(v))
			}
		case "1.3.6.1.2.1.1.2.0":
			if vtag == 0x06 {
				info.ObjectID = decodeOID(v)
			}
		case "1.3.6.1.2.1.1.5.0":
			if vtag == 0x04 {
				info.Name = strings.TrimSpace(string(v))
			}
		case "1.3.6.1.2.1.1.7.0":
			if vtag == 0x02 {
				for _, c := range v {
					info.Services = info.Services<<8 | int(c)
				}
			}
		}
	}
	return info, info.Descr != "" || info.ObjectID != ""
}

func discoverSNMP(ctx context.Context, ips []string, community string, col *collector) {
	c, err := net.ListenUDP("udp4", nil)
	if err != nil {
		return
	}
	defer c.Close()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		deadline := time.Now().Add(1500 * time.Millisecond)
		if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
			deadline = d
		}
		_ = c.SetReadDeadline(deadline)
		buf := make([]byte, 4096)
		for {
			n, src, err := c.ReadFromUDP(buf)
			if err != nil {
				return
			}
			info, ok := parseSNMP(buf[:n])
			if !ok {
				continue
			}
			ip := src.IP.String()
			col.with(ip, func(o *hostObs) { applySNMP(o, info) })
			col.log("SNMP", ip, "%s  %s", info.Name, firstLine(info.Descr, 80))
		}
	}()
	pkt := snmpGet(community, 0x4c50)
	for _, ip := range ips {
		_, _ = c.WriteToUDP(pkt, &net.UDPAddr{IP: net.ParseIP(ip), Port: 161})
	}
	wg.Wait()
}

func firstLine(s string, max int) string {
	s, _, _ = strings.Cut(s, "\n")
	s = strings.TrimSpace(s)
	if len([]rune(s)) > max {
		s = string([]rune(s)[:max]) + "…"
	}
	return s
}

func applySNMP(o *hostObs, info snmpInfo) {
	o.Sources["SNMP"] = true
	o.raw("SNMP", strings.TrimSpace(info.Name+"  "+firstLine(info.Descr, 160)+"  oid="+info.ObjectID+"  services="+strconv.Itoa(info.Services)))
	if info.Name != "" {
		o.Hostnames["SNMP"] = info.Name
	}
	if ent, ok := strings.CutPrefix(info.ObjectID, "1.3.6.1.4.1."); ok {
		num, _, _ := strings.Cut(ent, ".")
		if v, ok := snmpEnterprises[atoi(num)]; ok && v != "Net-SNMP" && o.Vendor == "" {
			o.Vendor = v
		}
	}
	d := strings.ToLower(info.Descr)
	if o.Model == "" && info.Descr != "" && !strings.HasPrefix(d, "linux") {
		o.Model = firstLine(info.Descr, 60)
	}
	switch {
	case strings.Contains(d, "switch") || strings.Contains(d, "交换机"):
		o.hint("switch", 11)
	case strings.Contains(d, "access point") || strings.Contains(d, "wireless ap") || strings.Contains(d, "无线"):
		o.hint("ap", 10)
	case strings.Contains(d, "router") || strings.Contains(d, "routeros") || strings.Contains(d, "路由"):
		o.hint("router", 9)
	case strings.Contains(d, "printer") || strings.Contains(d, "laserjet") || strings.Contains(d, "打印"):
		o.hint("printer", 9)
	}
	// sysServices：位 1=物理层 2=数据链路层 3=网络层 4=传输层 7=应用层
	l2, l3, l7 := info.Services&2 != 0, info.Services&4 != 0, info.Services&64 != 0
	switch {
	case l2 && !l3 && !l7:
		o.hint("switch", 8) // 纯二层设备
	case l2 && l3 && !l7:
		o.hint("switch", 6) // 三层交换机
	case l3 && !l2 && !l7:
		o.hint("router", 7)
	}
}
