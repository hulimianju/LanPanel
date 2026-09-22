package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"lanpanel/internal/api"
	"lanpanel/internal/discovery"
	"lanpanel/internal/store"
	"lanpanel/web"
)

// Version 在构建时通过 -ldflags "-X main.Version=..." 注入。
var Version = "dev"

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	listen := flag.String("listen", env("LANPANEL_LISTEN", ":3080"), "监听地址（环境变量 LANPANEL_LISTEN）")
	dataDir := flag.String("data", env("LANPANEL_DATA", "./data"), "数据目录（环境变量 LANPANEL_DATA）")
	showVersion := flag.Bool("version", false, "显示版本")
	flag.Parse()
	if *showVersion {
		fmt.Println(Version)
		return
	}
	log.SetFlags(log.LstdFlags)

	st, err := store.Open(*dataDir)
	if err != nil {
		log.Fatalf("打开数据目录失败: %v", err)
	}
	devices, err := discovery.OpenDeviceStore(*dataDir)
	if err != nil {
		log.Fatalf("打开设备库失败: %v", err)
	}
	app := api.New(st, devices, *dataDir, web.Dist(), Version)
	ctx, stopScheduler := context.WithCancel(context.Background())
	defer stopScheduler()
	go app.Scanner().RunScheduler(ctx)

	srv := &http.Server{
		Addr:              *listen,
		Handler:           app.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	ln, err := net.Listen("tcp", *listen)
	if err != nil {
		log.Fatalf("监听 %s 失败: %v", *listen, err)
	}
	log.Printf("LanPanel %s 已启动，数据目录 %s", Version, *dataDir)
	for _, u := range localURLs(ln.Addr().(*net.TCPAddr).Port) {
		log.Printf("  访问地址 %s", u)
	}

	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	stopScheduler()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	log.Print("已退出")
}

func localURLs(port int) []string {
	urls := []string{fmt.Sprintf("http://127.0.0.1:%d", port)}
	addrs, _ := net.InterfaceAddrs()
	for _, a := range addrs {
		if ipn, ok := a.(*net.IPNet); ok && ipn.IP.To4() != nil && ipn.IP.IsPrivate() {
			urls = append(urls, fmt.Sprintf("http://%s:%d", ipn.IP, port))
		}
	}
	return urls
}
