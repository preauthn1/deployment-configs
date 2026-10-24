package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"

	"example.invalid/sre/dbc/internal/gateway"
	"example.invalid/sre/dbc/internal/proxy"
)

func main() {
	configPath := flag.String("config", "gateway.yaml", "dbc-gateway YAML 設定檔")
	flag.Parse()
	if err := run(*configPath); err != nil {
		log.Printf("dbc-gateway 啟動失敗: %v", err)
		os.Exit(1)
	}
}

func run(configPath string) error {
	file, err := os.Open(configPath)
	if err != nil {
		return fmt.Errorf("開啟設定檔失敗: %w", err)
	}
	cfg, err := gateway.ParseConfig(file)
	_ = file.Close()
	if err != nil {
		return err
	}

	rootCtx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	// 稽核是安全邊界：任一 target 的 log project 無法同步寫入時，不開放 listener。
	for _, project := range auditProjects(cfg) {
		if err := proxy.CheckAuditLogging(rootCtx, project); err != nil {
			_ = proxy.CloseAuditLogging()
			return fmt.Errorf("Cloud Logging 稽核預檢失敗（project %s）: %w", project, err)
		}
	}
	defer func() {
		if err := proxy.CloseAuditLogging(); err != nil {
			log.Printf("Cloud Logging 關閉異常: %v", err)
		}
	}()

	groupChecker, err := gateway.NewCloudIdentityGroupChecker(rootCtx)
	if err != nil {
		return err
	}
	if err := groupChecker.Preflight(rootCtx, allowedGroups(cfg)); err != nil {
		return fmt.Errorf("Cloud Identity 群組預檢失敗: %w", err)
	}
	authorizer := &gateway.Authorizer{
		Config: cfg, TokenVerifier: gateway.NewGoogleTokenVerifier(), GroupChecker: groupChecker,
	}
	certificates, err := gateway.NewCertificateReloader(cfg.TLS.CertificateFile, cfg.TLS.PrivateKeyFile)
	if err != nil {
		return err
	}
	go certificates.Watch(rootCtx, time.Minute, func(err error) {
		log.Printf("TLS 憑證 reload 失敗，繼續使用上一份有效憑證: %v", err)
	})

	rawListener, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		return fmt.Errorf("監聽 gateway %s 失敗: %w", cfg.Listen, err)
	}
	tlsListener := tls.NewListener(rawListener, certificates.TLSConfig())

	healthListener, err := net.Listen("tcp", cfg.HealthListen)
	if err != nil {
		_ = tlsListener.Close()
		return fmt.Errorf("監聽 health %s 失敗: %w", cfg.HealthListen, err)
	}
	server := gateway.NewServer(cfg, authorizer)
	healthServer := &http.Server{Handler: server.HealthHandler(), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := healthServer.Serve(healthListener); err != nil && err != http.ErrServerClosed {
			log.Printf("health server 異常停止: %v", err)
		}
	}()

	log.Printf("dbc-gateway listening on %s; health on %s", cfg.Listen, cfg.HealthListen)
	serveErr := server.Serve(rootCtx, tlsListener)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := healthServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("health server 關閉失敗: %v", err)
	}
	return serveErr
}

func auditProjects(cfg *gateway.Config) []string {
	set := make(map[string]struct{})
	for _, target := range cfg.Targets {
		set[target.AuditProject] = struct{}{}
	}
	projects := make([]string, 0, len(set))
	for project := range set {
		projects = append(projects, project)
	}
	sort.Strings(projects)
	return projects
}

func allowedGroups(cfg *gateway.Config) []string {
	set := make(map[string]struct{})
	for _, target := range cfg.Targets {
		for _, role := range target.Roles {
			for _, group := range role.Groups {
				set[group] = struct{}{}
			}
		}
	}
	groups := make([]string, 0, len(set))
	for group := range set {
		groups = append(groups, group)
	}
	sort.Strings(groups)
	return groups
}
