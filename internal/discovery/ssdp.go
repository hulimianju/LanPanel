package discovery

import (
	"bufio"
	"bytes"
	"context"
	"encoding/xml"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// SSDP / UPnP：M-SEARCH 广播后，按设备声明的 LOCATION 抓取描述 XML，
// 得到友好名称、厂商、型号，以及设备自己声明的管理页（presentationURL）。

type upnpDesc struct {
	URLBase string     `xml:"URLBase"`
	Device  upnpDevice `xml:"device"`
}

type upnpDevice struct {
	DeviceType       string       `xml:"deviceType"`
	FriendlyName     string       `xml:"friendlyName"`
	Manufacturer     string       `xml:"manufacturer"`
	ModelName        string       `xml:"modelName"`
	ModelNumber      string       `xml:"modelNumber"`
	ModelDescription string       `xml:"modelDescription"`
	PresentationURL  string       `xml:"presentationURL"`
	Children         []upnpDevice `xml:"deviceList>device"`
}

func discoverSSDP(ctx context.Context, ifaces []localIface, col *collector) {
	locations := map[string]string{} // LOCATION → 源 IP
	servers := map[string]string{}   // 源 IP → SERVER 头
	var mu sync.Mutex
	var wg sync.WaitGroup
	msg := "M-SEARCH * HTTP/1.1\r\nHOST: 239.255.255.250:1900\r\nMAN: \"ssdp:discover\"\r\nMX: 2\r\nST: ssdp:all\r\n\r\n"
	dst := &net.UDPAddr{IP: net.IPv4(239, 255, 255, 250), Port: 1900}
	for _, ifi := range ifaces {
		c, err := net.ListenUDP("udp4", &net.UDPAddr{IP: ifi.IP})
		if err != nil {
			continue
		}
		wg.Add(1)
		go func(c *net.UDPConn) {
			defer wg.Done()
			defer c.Close()
			for i := 0; i < 2; i++ { // UDP 可能丢包，发两次
				_, _ = c.WriteToUDP([]byte(msg), dst)
				time.Sleep(100 * time.Millisecond)
			}
			deadline := time.Now().Add(3 * time.Second)
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
				resp, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(buf[:n])), nil)
				if err != nil {
					continue
				}
				loc := resp.Header.Get("Location")
				mu.Lock()
				if loc != "" {
					locations[loc] = src.IP.String()
				}
				if sv := resp.Header.Get("Server"); sv != "" {
					servers[src.IP.String()] = sv
				}
				mu.Unlock()
			}
		}(c)
	}
	wg.Wait()
	if ctx.Err() != nil {
		return
	}

	for ip, sv := range servers {
		col.with(ip, func(o *hostObs) {
			o.Sources["SSDP"] = true
			o.raw("SSDP", "SERVER: "+sv)
		})
	}

	// 抓取描述文件（同一设备可能声明多个 LOCATION，逐个处理，限制并发）
	client := &http.Client{Timeout: 3 * time.Second}
	sem := make(chan struct{}, 8)
	var fw sync.WaitGroup
	for loc, ip := range locations {
		u, err := url.Parse(loc)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			continue
		}
		fw.Add(1)
		sem <- struct{}{}
		go func(loc, ip string, u *url.URL) {
			defer fw.Done()
			defer func() { <-sem }()
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, loc, nil)
			resp, err := client.Do(req)
			if err != nil {
				return
			}
			defer resp.Body.Close()
			raw, _ := io.ReadAll(io.LimitReader(resp.Body, 256<<10))
			var d upnpDesc
			if xml.Unmarshal(raw, &d) != nil {
				return
			}
			base := u
			if d.URLBase != "" {
				if b, err := url.Parse(d.URLBase); err == nil {
					base = b
				}
			}
			col.with(ip, func(o *hostObs) { applyUPnP(o, ip, base, &d.Device) })
			col.log("SSDP", ip, "%s  %s %s  %s", strings.TrimSpace(d.Device.FriendlyName), d.Device.Manufacturer, d.Device.ModelName, shortDeviceType(d.Device.DeviceType))
		}(loc, ip, u)
	}
	fw.Wait()
}

func shortDeviceType(t string) string {
	// urn:schemas-upnp-org:device:MediaRenderer:1 → MediaRenderer
	parts := strings.Split(t, ":")
	if len(parts) >= 2 {
		return parts[len(parts)-2]
	}
	return t
}

func applyUPnP(o *hostObs, ip string, base *url.URL, d *upnpDevice) {
	o.Sources["SSDP"] = true
	name := strings.TrimSpace(d.FriendlyName)
	if name != "" && o.Hostnames["SSDP"] == "" {
		o.Hostnames["SSDP"] = name
	}
	model := strings.TrimSpace(strings.TrimSpace(d.ModelName + " " + d.ModelNumber))
	if model != "" && o.Model == "" && len(model) < 64 {
		o.Model = model
	}
	if d.Manufacturer != "" {
		o.Vendor = d.Manufacturer
	}
	o.raw("SSDP", strings.TrimSpace(name+"  "+d.Manufacturer+"  "+model+"  "+shortDeviceType(d.DeviceType)))
	classifyUPnP(o, d)
	// 设备声明的管理页
	if p := strings.TrimSpace(d.PresentationURL); p != "" {
		if ref, err := url.Parse(p); err == nil {
			u := base.ResolveReference(ref)
			if (u.Scheme == "http" || u.Scheme == "https") && u.Hostname() == ip {
				port := 80
				if u.Scheme == "https" {
					port = 443
				}
				if pp := u.Port(); pp != "" {
					port = atoi(pp)
				}
				label := "管理页"
				if name != "" {
					label = name
				}
				o.addService(Service{Port: port, Proto: "tcp", Scheme: u.Scheme, Name: label, URL: u.String(), Icon: "globe", Color: "blue", Source: "ssdp"})
			}
		}
	}
	for i := range d.Children {
		classifyUPnP(o, &d.Children[i])
		for j := range d.Children[i].Children {
			classifyUPnP(o, &d.Children[i].Children[j])
		}
	}
}
