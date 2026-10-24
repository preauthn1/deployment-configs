package gateway

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"example.invalid/api/cloudidentity/v1"
	"example.invalid/api/option"
)

// CloudIdentityGroupChecker 使用 gateway 的 ADC 服務身分查詢直接或巢狀群組資格。
// group email 到不可變 resource name 的 lookup 結果會快取，membership 結果暫不快取，
// 避免撤權後仍可建立新連線。
type CloudIdentityGroupChecker struct {
	lookupGroup func(context.Context, string) (string, error)
	checkMember func(context.Context, string, string) (bool, error)

	mu         sync.RWMutex
	groupNames map[string]string
}

// NewCloudIdentityGroupChecker 使用 Application Default Credentials 建立 client。
func NewCloudIdentityGroupChecker(ctx context.Context) (*CloudIdentityGroupChecker, error) {
	service, err := cloudidentity.NewService(ctx, option.WithScopes(cloudidentity.CloudIdentityGroupsReadonlyScope))
	if err != nil {
		return nil, fmt.Errorf("建立 Cloud Identity client 失敗: %w", err)
	}
	return &CloudIdentityGroupChecker{
		groupNames: make(map[string]string),
		lookupGroup: func(callCtx context.Context, groupEmail string) (string, error) {
			response, err := service.Groups.Lookup().GroupKeyId(groupEmail).Context(callCtx).Do()
			if err != nil {
				return "", err
			}
			return response.Name, nil
		},
		checkMember: func(callCtx context.Context, groupName, memberEmail string) (bool, error) {
			query := fmt.Sprintf("member_key_id == '%s'", escapeCELString(memberEmail))
			response, err := service.Groups.Memberships.CheckTransitiveMembership(groupName).Query(query).Context(callCtx).Do()
			if err != nil {
				return false, err
			}
			return response.HasMembership, nil
		},
	}, nil
}

// IsMember 實作 GroupChecker。任何 lookup/check 錯誤都交由 Authorizer fail-closed。
func (c *CloudIdentityGroupChecker) IsMember(ctx context.Context, email, group string) (bool, error) {
	if c == nil || c.lookupGroup == nil || c.checkMember == nil {
		return false, fmt.Errorf("Cloud Identity group checker 未初始化")
	}
	email = strings.ToLower(strings.TrimSpace(email))
	group = strings.ToLower(strings.TrimSpace(group))
	if email == "" || group == "" {
		return false, fmt.Errorf("member email 與 group email 不可為空")
	}

	groupName, err := c.resolveGroup(ctx, group)
	if err != nil {
		return false, err
	}

	member, err := c.checkMember(ctx, groupName, email)
	if err != nil {
		return false, fmt.Errorf("檢查 %q 是否屬於 %q 失敗: %w", email, group, err)
	}
	return member, nil
}

// Preflight resolves every configured group before the gateway becomes ready.
// This catches nonexistent groups and missing lookup permission at startup;
// membership checks remain per-session so revocations take effect immediately.
func (c *CloudIdentityGroupChecker) Preflight(ctx context.Context, groups []string) error {
	seen := make(map[string]struct{}, len(groups))
	for _, group := range groups {
		group = strings.ToLower(strings.TrimSpace(group))
		if group == "" {
			return fmt.Errorf("group email 不可為空")
		}
		if _, ok := seen[group]; ok {
			continue
		}
		seen[group] = struct{}{}
		if _, err := c.resolveGroup(ctx, group); err != nil {
			return err
		}
	}
	return nil
}

func (c *CloudIdentityGroupChecker) resolveGroup(ctx context.Context, group string) (string, error) {
	c.mu.RLock()
	groupName := c.groupNames[group]
	c.mu.RUnlock()
	if groupName != "" {
		return groupName, nil
	}

	groupName, err := c.lookupGroup(ctx, group)
	if err != nil {
		return "", fmt.Errorf("查詢群組 %q resource name 失敗: %w", group, err)
	}
	if groupName == "" {
		return "", fmt.Errorf("查詢群組 %q 得到空 resource name", group)
	}
	c.mu.Lock()
	c.groupNames[group] = groupName
	c.mu.Unlock()
	return groupName, nil
}

// escapeCELString 防止 token claim 被拼接成另一段 Cloud Identity CEL query。
func escapeCELString(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	return strings.ReplaceAll(value, `'`, `\'`)
}
