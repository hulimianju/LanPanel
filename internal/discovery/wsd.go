package discovery

import (
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"regexp"
	"strings"
	"sync"
	"time"
)

// WS-Discovery（UDP 3702）：ONVIF 摄像头、Windows 电脑、网络打印机会应答 Probe。

const wsdProbe = `<?xml version="1.0" encoding="utf-8"?>` +
	`<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope" xmlns:wsa="http://schemas.xmlsoap.org/ws/2004/08/addressing" xmlns:wsd="http://schemas.xmlsoap.org/ws/2005/04/discovery">` +
	`<soap:Header><wsa:To>urn:schemas-xmlsoap-org:ws:2005:04:discovery</wsa:To>` +
	`<wsa:Action>http://schemas.xmlsoap.org/ws/2005/04/discovery/Probe</wsa:Action>` +
	`<wsa:MessageID>urn:uuid:%s</wsa:MessageID></soap:Header><soap:Body><wsd:Probe/></soap:Body></soap:Envelope>`

var (
	wsdTypes  = regexp.MustCompile(`(?s)<[\w-]*:?Types[^>]*>(.*?)</[\w-]*:?Types>`)
	wsdXAddrs = regexp.MustCompile(`(?s)<[\w-]*:?XAddrs[^>]*>(.*?)</[\w-]*:?XAddrs>`)
	wsdScopes = regexp.MustCompile(`(?s)<[\w-]*:?Scopes[^>]*>(.*?)</[\w-]*:?Scopes>`)
)

func uuid4() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6], b[8] = b[6]&0x0f|0x40, b[8]&0x3f|0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func discoverWSD(ctx context.Context, ifaces []localIface, col *collector) {
	dst := &net.UDPAddr{IP: net.IPv4(239, 255, 255, 250), Port: 3702}
	var wg sync.WaitGroup
	var seenMu sync.Mutex
	seen := map[string]bool{} // 设备常对一次探测多次应答，只处理一次
	for _, ifi := range ifaces {
		c, err := net.ListenUDP("udp4", &net.UDPAddr{IP: ifi.IP})
		if err != nil {
			continue
		}
		wg.Add(1)
		go func(c *net.UDPConn) {
			defer wg.Done()
			defer c.Close()
			_, _ = c.WriteToUDP([]byte(fmt.Sprintf(wsdProbe, uuid4())), dst)
			deadline := time.Now().Add(2500 * time.Millisecond)
			if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
				deadline = d
			}
			_ = c.SetReadDeadline(deadline)
			buf := make([]byte, 16<<10)
			for {
				n, src, err := c.ReadFromUDP(buf)
				if err != nil {
					return
				}
				body := string(buf[:n])
				types, scopes, xaddrs := firstGroup(wsdTypes, body), firstGroup(wsdScopes, body), firstGroup(wsdXAddrs, body)
				if types == "" {
					continue
				}
				ip := src.IP.String()
				seenMu.Lock()
				dup := seen[ip]
				seen[ip] = true
				seenMu.Unlock()
				if dup {
					continue
				}
				col.with(ip, func(o *hostObs) { applyWSD(o, ip, types, scopes, xaddrs) })
				col.log("WSD", ip, "%s", strings.TrimSpace(types))
			}
		}(c)
	}
	wg.Wait()
}

func firstGroup(re *regexp.Regexp, s string) string {
	if m := re.FindStringSubmatch(s); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func applyWSD(o *hostObs, ip, types, scopes, xaddrs string) {
	o.Sources["WSD"] = true
	o.raw("WSD", strings.TrimSpace(types+"  "+xaddrs))
	lt := strings.ToLower(types)
	switch {
	case strings.Contains(lt, "networkvideotransmitter"):
		o.hint("camera", 9)
		// ONVIF 作用域里带有名称与硬件型号：onvif://www.onvif.org/name/xxx
		for _, sc := range strings.Fields(scopes) {
			if v, ok := strings.CutPrefix(sc, "onvif://www.onvif.org/hardware/"); ok && o.Model == "" {
				o.Model = unescapeURL(v)
			}
			if v, ok := strings.CutPrefix(sc, "onvif://www.onvif.org/name/"); ok && o.Hostnames["WSD"] == "" {
				o.Hostnames["WSD"] = unescapeURL(v)
			}
		}
	case strings.Contains(lt, "printdevice") || strings.Contains(lt, "printer"):
		o.hint("printer", 8)
	case strings.Contains(lt, "computer"):
		o.hint("pc", 6)
	}
}

func unescapeURL(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) {
			var v byte
			if _, err := fmt.Sscanf(s[i+1:i+3], "%02x", &v); err == nil {
				b.WriteByte(v)
				i += 2
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
