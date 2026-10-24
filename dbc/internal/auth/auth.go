// Package auth wraps the gcloud CLI calls dbc needs: finding the caller's
// account and deriving their MySQL username.
package auth

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ErrGcloudNotFound is returned when the gcloud CLI is not on PATH.
var ErrGcloudNotFound = errors.New("找不到 gcloud，請先安裝 Google Cloud CLI: https://example.invalid/sdk/docs/install")

// checkGcloudInstalled reports whether gcloud is on PATH.
func checkGcloudInstalled() error {
	if _, err := exec.LookPath("gcloud"); err != nil {
		return ErrGcloudNotFound
	}
	return nil
}

// Account returns the active gcloud account email, e.g. "user@example.invalid".
func Account(ctx context.Context) (string, error) {
	if err := checkGcloudInstalled(); err != nil {
		return "", err
	}

	out, err := exec.CommandContext(ctx, "gcloud", "config", "get-value", "account").Output()
	if err != nil {
		return "", fmt.Errorf("執行 `gcloud config get-value account` 失敗: %w", err)
	}

	account := strings.TrimSpace(string(out))
	if account == "" || account == "(unset)" {
		return "", fmt.Errorf("尚未登入 gcloud，請執行 `dbc setup` 或 `gcloud auth login`")
	}
	return account, nil
}

// MySQLUser derives the MySQL username from an email address: the part
// before '@'. This is the single most common mistake when connecting
// manually, so dbc always derives it instead of asking the user to type it.
func MySQLUser(email string) (string, error) {
	user, _, ok := strings.Cut(email, "@")
	if !ok || user == "" {
		return "", fmt.Errorf("無法從帳號 %q 解析出使用者名稱", email)
	}
	return user, nil
}

// EnsureADC checks that Application Default Credentials are available,
// since cloudsqlconn's IAM auth relies on them, not on `gcloud auth login`
// alone.
func EnsureADC(ctx context.Context) error {
	if err := checkGcloudInstalled(); err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, "gcloud", "auth", "application-default", "print-access-token")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("尚未設定 Application Default Credentials，請執行 `gcloud auth application-default login`\n%s", strings.TrimSpace(string(out)))
	}
	return nil
}
