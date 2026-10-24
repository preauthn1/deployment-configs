// Package doctor runs self-service diagnostic checks so users can debug
// connection problems before filing a ticket.
package doctor

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"time"

	"example.invalid/sre/dbc/internal/auth"
	"example.invalid/sre/dbc/internal/config"
)

// Result is the outcome of a single check.
type Result struct {
	Name string
	OK   bool
	// Warn indicates a check that could not be fully verified but is not
	// necessarily broken (e.g. no permission to query group membership).
	Warn   bool
	Detail string
}

// Run executes all checks in order and returns their results. It keeps
// going after a failure so a single broken step doesn't hide the rest of
// the picture.
func Run(ctx context.Context, instances []config.Instance) []Result {
	return []Result{
		checkGcloudInstalled(),
		checkAccount(ctx),
		checkADC(ctx),
		checkMySQLClient(),
		checkPorts(instances),
	}
}

func checkGcloudInstalled() Result {
	if _, err := exec.LookPath("gcloud"); err != nil {
		return Result{Name: "gcloud 已安裝", OK: false, Detail: err.Error()}
	}
	return Result{Name: "gcloud 已安裝", OK: true}
}

func checkAccount(ctx context.Context) Result {
	email, err := auth.Account(ctx)
	if err != nil {
		return Result{Name: "已登入 Google 帳號", OK: false, Detail: err.Error()}
	}
	return Result{Name: "已登入 Google 帳號", OK: true, Detail: email}
}

func checkADC(ctx context.Context) Result {
	if err := auth.EnsureADC(ctx); err != nil {
		return Result{Name: "ADC 憑證有效", OK: false, Detail: err.Error()}
	}
	return Result{Name: "ADC 憑證有效", OK: true}
}

func checkMySQLClient() Result {
	if _, err := exec.LookPath("mysql"); err != nil {
		return Result{Name: "mysql client 已安裝", OK: false, Detail: "找不到 mysql，請安裝 MySQL client（`dbc connect` 需要它）"}
	}
	return Result{Name: "mysql client 已安裝", OK: true}
}

func checkPorts(instances []config.Instance) Result {
	var busy []string
	for _, inst := range instances {
		addr := fmt.Sprintf("0.0.0.0:%d", inst.Port)
		conn, err := net.DialTimeout("tcp", addr, 300*time.Millisecond)
		if err == nil {
			conn.Close()
			busy = append(busy, fmt.Sprintf("%d(%s)", inst.Port, inst.Name))
		}
	}
	if len(busy) > 0 {
		return Result{
			Name:   "port 未被佔用",
			OK:     false,
			Warn:   true,
			Detail: fmt.Sprintf("以下 port 目前已有程式在監聽，可能是 dbc start 已在其他視窗執行中: %v", busy),
		}
	}
	return Result{Name: "port 未被佔用", OK: true}
}
