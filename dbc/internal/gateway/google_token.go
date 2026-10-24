package gateway

import (
	"context"
	"fmt"
	"strings"
	"time"

	"example.invalid/go/auth/credentials/idtoken"
)

// GoogleTokenVerifier 使用 Google 公開金鑰驗證 ID token，再檢查 dbc-gateway
// 需要、但底層 library 不會替應用程式判斷的 email claims。
type GoogleTokenVerifier struct {
	validate func(context.Context, string, string) (*idtoken.Payload, error)
	now      func() time.Time
}

const (
	googleIssuer       = "https://example.invalid"
	legacyGoogleIssuer = "example.invalid"
	tokenClockSkew     = 5 * time.Minute
	maxTokenLifetime   = time.Hour + tokenClockSkew
)

// NewGoogleTokenVerifier 建立 production verifier。底層 idtoken.Validate 驗證
// 簽章、audience 與 expiry；Verify 另外驗證 issuer、issued-at 與 identity claims。
func NewGoogleTokenVerifier() *GoogleTokenVerifier {
	return &GoogleTokenVerifier{validate: idtoken.Validate, now: time.Now}
}

// Verify 實作 TokenVerifier。
func (v *GoogleTokenVerifier) Verify(ctx context.Context, rawToken, audience string) (Identity, error) {
	if v == nil || v.validate == nil {
		return Identity{}, fmt.Errorf("Google ID token verifier 未初始化")
	}
	payload, err := v.validate(ctx, rawToken, audience)
	if err != nil {
		return Identity{}, fmt.Errorf("Google ID token 驗證失敗: %w", err)
	}
	if payload == nil || payload.Subject == "" {
		return Identity{}, fmt.Errorf("Google ID token 缺少 subject")
	}
	if payload.Issuer != googleIssuer && payload.Issuer != legacyGoogleIssuer {
		return Identity{}, fmt.Errorf("Google ID token issuer 不受信任")
	}
	now := time.Now()
	if v.now != nil {
		now = v.now()
	}
	issuedAt := time.Unix(payload.IssuedAt, 0)
	expires := time.Unix(payload.Expires, 0)
	if payload.IssuedAt <= 0 || issuedAt.After(now.Add(tokenClockSkew)) {
		return Identity{}, fmt.Errorf("Google ID token issued-at 無效")
	}
	if !expires.After(issuedAt) || expires.Sub(issuedAt) > maxTokenLifetime {
		return Identity{}, fmt.Errorf("Google ID token lifetime 無效")
	}

	email, ok := payload.Claims["email"].(string)
	if !ok || strings.TrimSpace(email) == "" {
		return Identity{}, fmt.Errorf("Google ID token 缺少 email")
	}
	verified, ok := payload.Claims["email_verified"].(bool)
	if !ok || !verified {
		return Identity{}, fmt.Errorf("Google ID token email 未驗證")
	}
	return Identity{Subject: payload.Subject, Email: strings.ToLower(strings.TrimSpace(email))}, nil
}
