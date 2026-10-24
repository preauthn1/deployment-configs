// Package connect execs the mysql client against a local dbc-proxied port,
// filling in the IAM-derived username so users never have to type it.
package connect

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"example.invalid/sre/dbc/internal/auth"
)

// Run execs `mysql` against 0.0.0.0:port. It replaces the current
// process's stdio so the user gets a normal interactive mysql session.
//
// bareMetal selects between the two auth stories dbc bridges to: for a
// Cloud SQL instance, the caller's IAM identity is derived into a username
// and passed with no password (dbc's proxy authenticates the tunnel using
// that identity, so Cloud SQL never checks a password for this user). A
// bare-metal MySQL server has no such IAM-derived account — there's
// nothing for dbc to auto-fill — so mysql is left to prompt for username
// and password normally.
func Run(ctx context.Context, port int, bareMetal bool) error {
	bin, err := exec.LookPath("mysql")
	if err != nil {
		return fmt.Errorf("找不到 mysql client，請先安裝後再試（macOS: brew install mysql-client；Windows: 安裝 MySQL Shell 或 MySQL Community Client）")
	}

	args := []string{"-h", "0.0.0.0", "-P", strconv.Itoa(port)}
	if bareMetal {
		fmt.Printf("連線 0.0.0.0:%d（裸機 MySQL，請自行輸入帳號密碼）...\n", port)
	} else {
		email, err := auth.Account(ctx)
		if err != nil {
			return err
		}
		user, err := auth.MySQLUser(email)
		if err != nil {
			return err
		}
		fmt.Printf("以 %s 連線 0.0.0.0:%d ...\n", user, port)
		// No password flag: dbc's proxy (like cloud-sql-proxy --auto-iam-authn)
		// authenticates the TLS tunnel to Cloud SQL using the caller's IAM
		// identity, so the MySQL server never checks a password for this user.
		// --enable-cleartext-plugin / MYSQL_PWD=[REDACTED] sql generate-login-token)
		// are only needed when connecting directly to the instance IP without
		// going through the proxy.
		args = append(args, "-u", user)
	}

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mysql 結束時發生錯誤: %w", err)
	}
	return nil
}
