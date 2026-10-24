// Package proxy runs local TCP listeners that forward to Cloud SQL
// instances over cloudsqlconn, using the caller's IAM credentials
// (equivalent to cloud-sql-proxy --auto-iam-authn).
package proxy

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"sort"
	"strconv"

	"example.invalid/go/cloudsqlconn"

	"example.invalid/sre/dbc/internal/auth"
	"example.invalid/sre/dbc/internal/config"
	"example.invalid/sre/dbc/internal/gatewayclient"
)

// Serve opens one listener per instance and blocks until ctx is cancelled
// (e.g. Ctrl-C) or a listener fails to start.
//
// Before listening, it checks (once per distinct GCP project among
// instances) that the caller can actually write the query-audit log — see
// queryaudit.go's package doc for why that matters. An instance whose
// project fails this check is refused rather than served without an audit
// trail; if every instance fails, Serve refuses to start at all.
func Serve(ctx context.Context, instances []config.Instance, address, clientVersion string) error {
	if len(instances) == 0 {
		return fmt.Errorf("沒有要掛載的實例")
	}

	// Display order only (listening doesn't care about order): group by
	// project like `dbc list` does, so the two commands read consistently.
	sorted := make([]config.Instance, len(instances))
	copy(sorted, instances)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Project != sorted[j].Project {
			return sorted[i].Project < sorted[j].Project
		}
		return sorted[i].Port < sorted[j].Port
	})

	account := ""
	if hasLocallyAuditedInstance(sorted) {
		var err error
		account, err = auth.Account(ctx)
		if err != nil {
			return err
		}
	}

	servable, skipped := partitionByAuditPermission(ctx, sorted)
	for _, inst := range skipped {
		fmt.Printf("❌ %s 缺少稽核紀錄寫入權限（roles/logging.logWriter），無法啟動：%s\n", inst.Project, inst.ConnName())
	}
	if len(skipped) > 0 {
		fmt.Println("   請聯絡管理者確認是否已加入 db-prod-user/db-prod-admin 群組。")
	}
	if len(servable) == 0 {
		return fmt.Errorf("沒有任何實例通過稽核紀錄權限檢查，dbc start 無法啟動")
	}

	var dialer *cloudsqlconn.Dialer
	if hasCloudSQLInstance(servable) {
		var err error
		dialer, err = cloudsqlconn.NewDialer(ctx, cloudsqlconn.WithIAMAuthN())
		if err != nil {
			return fmt.Errorf("建立 Cloud SQL dialer 失敗: %w", err)
		}
		defer dialer.Close()
	}

	var listeners []net.Listener
	defer func() {
		for _, ln := range listeners {
			ln.Close()
		}
	}()

	currentProject := ""
	for _, inst := range servable {
		addr := listenAddress(address, inst.Port)
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			return fmt.Errorf("位址 %s 無法監聽（可能 port 已被其他程式佔用，執行 `dbc doctor` 檢查）: %w", addr, err)
		}
		listeners = append(listeners, ln)

		if inst.Project != currentProject {
			if currentProject != "" {
				fmt.Println()
			}
			fmt.Printf("%s (%s)\n", inst.Project, inst.Region)
			fmt.Printf("  %-55s %-8s %s\n", "CONN_NAME", "ROLE", "PORT")
			currentProject = inst.Project
		}
		fmt.Printf("  %-55s %-8s %d\n", inst.ConnName(), inst.Role, inst.Port)
		go acceptLoop(ctx, dialer, ln, inst, account, clientVersion)
	}

	<-ctx.Done()
	return nil
}

func listenAddress(address string, port int) string {
	return net.JoinHostPort(address, strconv.Itoa(port))
}

// partitionByAuditPermission runs CheckAuditLogging once per distinct
// project among instances (order-preserving within each bucket).
func partitionByAuditPermission(ctx context.Context, instances []config.Instance) (servable, skipped []config.Instance) {
	checked := map[string]error{}
	for _, inst := range instances {
		// Gateway-side logging is authoritative; the local user does not need
		// logWriter and cannot suppress gateway audit records.
		if inst.UsesGateway() {
			servable = append(servable, inst)
			continue
		}
		err, ok := checked[inst.Project]
		if !ok {
			err = CheckAuditLogging(ctx, inst.Project)
			checked[inst.Project] = err
		}
		if err != nil {
			skipped = append(skipped, inst)
		} else {
			servable = append(servable, inst)
		}
	}
	return servable, skipped
}

func hasLocallyAuditedInstance(instances []config.Instance) bool {
	for _, inst := range instances {
		if !inst.UsesGateway() {
			return true
		}
	}
	return false
}

func hasCloudSQLInstance(instances []config.Instance) bool {
	for _, inst := range instances {
		if !inst.IsBareMetal() {
			return true
		}
	}
	return false
}

func acceptLoop(ctx context.Context, dialer *cloudsqlconn.Dialer, ln net.Listener, inst config.Instance, account, clientVersion string) {
	for {
		client, err := ln.Accept()
		if err != nil {
			// Listener was closed as part of shutdown; nothing to report.
			return
		}
		go handleConn(ctx, dialer, client, inst, account, clientVersion)
	}
}

func handleConn(ctx context.Context, dialer *cloudsqlconn.Dialer, client net.Conn, inst config.Instance, account, clientVersion string) {
	defer client.Close()
	if inst.UsesGateway() {
		if err := pipe(ctx, dialer, client, inst, nil, clientVersion); err != nil {
			log.Printf("⚠️  %d %s gateway 連線失敗: %v", inst.Port, inst.Name, err)
		}
		return
	}
	aud, err := newQueryAuditor(inst.Project, account, inst)
	if err != nil {
		log.Printf("⚠️  %d %s 稽核紀錄初始化失敗，本次連線不受影響但不會被記錄: %v", inst.Port, inst.Name, err)
		aud = nil
	}
	if err := pipe(ctx, dialer, client, inst, aud, clientVersion); err != nil {
		log.Printf("⚠️  %d %s 連線失敗: %v", inst.Port, inst.Name, err)
	}
}

// Listen opens a local TCP listener on an OS-assigned port, for one-shot
// use with ServeOnce.
func Listen() (net.Listener, error) {
	return net.Listen("tcp", "0.0.0.0:0")
}

// ServeOnce accepts exactly one connection on ln, proxies it to inst, and
// blocks until that connection closes. Unlike Serve, this doesn't loop or
// print anything — it's for one-shot, non-interactive uses like
// `dbc admin grants` running a single query rather than a long-running
// listener.
func ServeOnce(ctx context.Context, dialer *cloudsqlconn.Dialer, ln net.Listener, inst config.Instance) error {
	client, err := ln.Accept()
	if err != nil {
		return err
	}
	defer client.Close()
	// No query auditor here: this is `dbc admin`'s internal one-shot
	// queries (GRANT/CALL statements), which Cloud SQL's own DCL audit
	// rule already covers — logging them again through this path would be
	// redundant noise, not a real gap like the one queryaudit.go exists
	// to close.
	return pipe(ctx, dialer, client, inst, nil, "")
}

// dialInstance connects to inst: a plain TCP dial to Host for a bare-metal
// server, or through cloudsqlconn's IAM-auth tunnel for a Cloud SQL
// instance. Bare-metal servers have no equivalent to Cloud SQL's IAM
// database authentication, so there's nothing for dbc to broker there —
// the mysql client authenticates with the real server normally (see
// internal/connect, which skips its IAM-derived username auto-fill for
// these).
func dialInstance(ctx context.Context, dialer *cloudsqlconn.Dialer, inst config.Instance, clientVersion string) (net.Conn, error) {
	if inst.Gateway != nil {
		client := &gatewayclient.Client{
			Config:      gatewayClientConfig(inst, clientVersion),
			TokenSource: identityTokenSource{},
		}
		return client.Dial(ctx)
	}
	if inst.IsBareMetal() {
		var d net.Dialer
		return d.DialContext(ctx, "tcp", inst.Host)
	}
	if dialer == nil {
		return nil, fmt.Errorf("Cloud SQL dialer 未初始化")
	}
	return dialer.Dial(ctx, inst.ConnName())
}

func gatewayClientConfig(inst config.Instance, clientVersion string) gatewayclient.Config {
	return gatewayclient.Config{
		Endpoint: inst.Gateway.Endpoint, ServerName: inst.Gateway.ServerName,
		Audience: inst.Gateway.Audience, Target: inst.Gateway.Target, ClientVersion: clientVersion,
	}
}

type identityTokenSource struct{}

func (identityTokenSource) Token(ctx context.Context, audience string) (string, error) {
	return auth.IdentityToken(ctx, audience)
}

// pipe forwards bytes between client and server unmodified. If aud is
// non-nil, each direction is additionally parsed at the MySQL-packet level
// so aud can log queries as they pass through — see queryaudit.go.
func pipe(ctx context.Context, dialer *cloudsqlconn.Dialer, client net.Conn, inst config.Instance, aud *queryAuditor, clientVersion string) error {
	server, err := dialInstance(ctx, dialer, inst, clientVersion)
	if err != nil {
		return fmt.Errorf("連線失敗: %w", err)
	}
	defer server.Close()

	relayBidirectional(client, server, func() {
		if aud != nil {
			relayClientToServer(server, client, aud)
		} else {
			io.Copy(server, client)
		}
	}, func() {
		if aud != nil {
			relayServerToClient(client, server, aud)
		} else {
			io.Copy(client, server)
		}
	})
	return nil
}
