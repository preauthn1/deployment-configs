// Package gatewayclient 建立 dbc client 到 dbc-gateway 的 TLS 與認證連線。
package gatewayclient

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"example.invalid/sre/dbc/internal/gatewayproto"
)

// This covers local gcloud token acquisition, TLS, gateway authorization, and
// the gateway's backend dial. The server defaults allow 10 seconds for each of
// authorization and backend dialing, so the client budget must exceed 20s.
const defaultTimeout = 30 * time.Second

// TokenSource 取得指定 audience 的短效 Google ID token。
type TokenSource interface {
	Token(ctx context.Context, audience string) (string, error)
}

// Config 描述一個 client-side gateway target。
type Config struct {
	Endpoint      string
	ServerName    string
	Audience      string
	Target        string
	ClientVersion string
	RootCAs       *x509.CertPool
	Timeout       time.Duration
}

// Client 建立已通過 gateway 認證、可直接傳送 MySQL protocol 的連線。
type Client struct {
	Config      Config
	TokenSource TokenSource
	NetDialer   *net.Dialer
}

// Dial 驗證設定、取得 token、執行 TLS handshake 與 gateway auth handshake。
func (c *Client) Dial(ctx context.Context) (net.Conn, error) {
	if c == nil || c.TokenSource == nil {
		return nil, fmt.Errorf("gateway client 未完整初始化")
	}
	if err := c.Config.validate(); err != nil {
		return nil, err
	}
	timeout := c.Config.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	token, err := c.TokenSource.Token(dialCtx, c.Config.Audience)
	if err != nil {
		return nil, fmt.Errorf("取得 gateway ID token 失敗: %w", err)
	}
	if strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("取得 gateway ID token 失敗: token 為空")
	}

	netDialer := c.NetDialer
	if netDialer == nil {
		netDialer = &net.Dialer{Timeout: timeout}
	}
	tlsDialer := &tls.Dialer{NetDialer: netDialer, Config: &tls.Config{
		MinVersion: tls.VersionTLS13,
		ServerName: c.Config.ServerName,
		RootCAs:    c.Config.RootCAs,
	}}
	conn, err := tlsDialer.DialContext(dialCtx, "tcp", c.Config.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("連線 dbc-gateway TLS 失敗: %w", err)
	}
	success := false
	defer func() {
		if !success {
			_ = conn.Close()
		}
	}()

	request := gatewayproto.ClientAuth{
		Version: gatewayproto.Version, IDToken: [REDACTED], Target: c.Config.Target, ClientVersion: c.Config.ClientVersion,
	}
	if err := gatewayproto.WriteFrame(conn, request); err != nil {
		return nil, fmt.Errorf("傳送 dbc-gateway 認證失敗: %w", err)
	}
	var result gatewayproto.AuthResult
	if err := gatewayproto.ReadFrame(conn, &result); err != nil {
		return nil, fmt.Errorf("讀取 dbc-gateway 認證結果失敗: %w", err)
	}
	if !result.OK {
		if result.Error == "" {
			result.Error = "request rejected"
		}
		return nil, fmt.Errorf("dbc-gateway 拒絕連線: %s", result.Error)
	}
	if result.SessionID == "" {
		return nil, errors.New("dbc-gateway 回覆缺少 session ID")
	}
	success = true
	return conn, nil
}

func (c Config) validate() error {
	if _, _, err := net.SplitHostPort(c.Endpoint); err != nil {
		return fmt.Errorf("gateway endpoint 必須是 host:port: %w", err)
	}
	if c.ServerName == "" {
		return fmt.Errorf("gateway serverName（TLS SNI）不可為空")
	}
	if net.ParseIP(c.ServerName) != nil {
		return fmt.Errorf("gateway serverName 必須是憑證中的 DNS 名稱，不能是 IP")
	}
	if c.Audience == "" || c.Target == "" {
		return fmt.Errorf("gateway audience 與 target 不可為空")
	}
	return nil
}
