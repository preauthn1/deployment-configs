package gateway

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"example.invalid/sre/dbc/internal/gatewayproto"
)

var (
	// ErrUnauthenticated 對外統一表示 token 無效，不洩漏驗證細節。
	ErrUnauthenticated = errors.New("身分驗證失敗")
	// ErrPermissionDenied 表示身分有效，但沒有 target 的群組權限。
	ErrPermissionDenied = errors.New("沒有目標資料庫的存取權限")
)

// Identity 是通過 ID token 驗證後可供授權與稽核使用的身分。
type Identity struct {
	Subject string
	Email   string
}

// TokenVerifier 驗證 token 的簽章、issuer、audience 與時間 claims。
type TokenVerifier interface {
	Verify(ctx context.Context, rawToken, audience string) (Identity, error)
}

// GroupChecker 使用 gateway 自己的服務身分查詢 Cloud Identity 群組資格。
type GroupChecker interface {
	IsMember(ctx context.Context, email, group string) (bool, error)
}

// Authorization 是成功連線後 server 所需的可信任資訊。
type Authorization struct {
	Identity Identity
	TargetID string
	Target   TargetConfig
	// Role is the first TargetRole (in Target.Roles order) whose Groups
	// the caller was found to be a member of — its MySQLUser/MySQLPassword
	// are what the gateway authenticates to the backend with, see
	// mysqlauth.go.
	Role TargetRole
}

// Authorizer 組合 token 驗證、target allowlist 與群組檢查。
type Authorizer struct {
	Config        *Config
	TokenVerifier TokenVerifier
	GroupChecker  GroupChecker
}

// Authorize 驗證 client 認證 frame。群組服務錯誤採 fail-closed。
func (a *Authorizer) Authorize(ctx context.Context, request gatewayproto.ClientAuth) (*Authorization, error) {
	if a == nil || a.Config == nil || a.TokenVerifier == nil || a.GroupChecker == nil {
		return nil, fmt.Errorf("gateway authorizer 未完整初始化")
	}
	if request.Version != gatewayproto.Version || strings.TrimSpace(request.IDToken) == "" {
		return nil, ErrUnauthenticated
	}
	target, ok := a.Config.Targets[request.Target]
	if !ok {
		return nil, ErrPermissionDenied
	}

	identity, err := a.TokenVerifier.Verify(ctx, request.IDToken, a.Config.Audience)
	if err != nil || identity.Subject == "" || identity.Email == "" {
		return nil, ErrUnauthenticated
	}
	identity.Email = strings.ToLower(strings.TrimSpace(identity.Email))

	// Roles is an ordered list: the first role whose groups the caller
	// belongs to wins, so a role's position in the config is its priority —
	// see TargetRole's doc.
	for _, role := range target.Roles {
		for _, group := range role.Groups {
			member, err := a.GroupChecker.IsMember(ctx, identity.Email, group)
			if err != nil {
				return nil, fmt.Errorf("查詢群組資格失敗: %w", err)
			}
			if member {
				return &Authorization{Identity: identity, TargetID: request.Target, Target: target, Role: role}, nil
			}
		}
	}
	return nil, ErrPermissionDenied
}
