package main

import (
	"context"
	"embed"
	"errors"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata"

	"z-mihomo-health-check/internal/app"
	"z-mihomo-health-check/internal/httpapi"
)

//go:embed sites.txt
var poolFS embed.FS

func environment(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
func main() {
	address := environment("LISTEN_ADDR", "127.0.0.1:8080")
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		_, port, err := net.SplitHostPort(address)
		if err != nil {
			os.Exit(1)
		}
		client := http.Client{Timeout: 2 * time.Second}
		resp, err := client.Get("http://127.0.0.1:" + port + "/healthz")
		if err != nil {
			os.Exit(1)
		}
		resp.Body.Close()
		if resp.StatusCode != 200 {
			os.Exit(1)
		}
		return
	}
	dataDir := environment("DATA_DIR", "./data")
	data, _ := poolFS.ReadFile("sites.txt")
	sites := []string{}
	seen := map[string]bool{}
	for _, line := range strings.Fields(string(data)) {
		u, e := url.Parse(line)
		if e != nil || u.Scheme != "https" || u.Host == "" {
			log.Fatal("站点池包含无效 HTTPS URL")
		}
		if !seen[line] {
			sites = append(sites, line)
			seen[line] = true
		}
	}
	if len(sites) == 0 {
		log.Fatal("站点池为空")
	}
	manager, err := app.NewInstanceManager(dataDir, sites)
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	manager.Start()
	srv := &http.Server{Addr: address, Handler: httpapi.NewInstancesServer(manager), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 * 1024}
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-ctx.Done()
		manager.Close()
		shutdownCtx, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		srv.Shutdown(shutdownCtx)
	}()
	log.Printf("Mihomo Health 面板监听 %s，站点池 %d 个", address, len(sites))
	if err = srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		cancel()
		<-shutdownDone
		log.Fatal("面板 HTTP 服务启动失败，请检查监听地址和端口")
	}
	<-shutdownDone
}
