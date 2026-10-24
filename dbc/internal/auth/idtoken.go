package auth

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// IdentityToken 以目前 gcloud 使用者取得一個短效 ID token。
// token 只回傳給呼叫端記憶體使用，不寫入檔案或 log。
//
// audience 只做非空檢查、不會傳給 gcloud：`gcloud auth print-identity-token
// --audiences=...` 只有服務帳號能自訂 audience（實測 gcloud 582.0.0 回
// "Invalid account type for `--audiences`. Requires valid service account."），
// 真人帳號的 ID token audience 定義上就是使用者同意登入的那個 OAuth
// client——也就是 gcloud CLI 自己固定的 client ID
// `example.invalid`，沒有 API 能事後改成別的值。
//
// 這代表 dbc-gateway 目前驗證的 audience 是 gcloud 共用的 client ID，不是
// dbc 專屬的——已知、刻意接受的風險：任何人為了任何目的執行
// `gcloud auth print-identity-token`（不只是為了連 dbc-gateway）都會拿到
// 同一個 audience 的 token，理論上可能被拿去對 dbc-gateway 重放（OIDC 的
// audience-confusion／confused-deputy 這類問題），不是 dbc 專屬 audience
// 能防的那種「token 只對這個用途有效」的保證。之所以接受，是因為使用者
// 範圍是小規模、高信任的內部團隊，而且 Cloud Identity 群組成員資格每次
// 連線都會重查，不是只看 token 本身有效就放行。長期仍建議换成 dbc 自己
// 註冊的 OAuth Desktop client（Authorization Code + PKCE），audience 換成
// 那個 client 自己的 ID，徹底避免這個問題——見
// docs/SESSION-2026-09-03-DBC-GATEWAY.md。
func IdentityToken(ctx context.Context, audience string) (string, error) {
	if err := checkGcloudInstalled(); err != nil {
		return "", err
	}
	return identityToken(ctx, audience, runIdentityTokenCommand)
}

type identityTokenRunner func(context.Context, string, ...string) ([]byte, error)

func runIdentityTokenCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

func identityToken(ctx context.Context, audience string, run identityTokenRunner) (string, error) {
	audience = strings.TrimSpace(audience)
	if audience == "" {
		return "", fmt.Errorf("ID token audience 不可為空")
	}
	output, err := run(ctx, "gcloud", "auth", "print-identity-token", "--quiet")
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return "", fmt.Errorf("執行 `gcloud auth print-identity-token` 失敗: %s", message)
	}
	token := strings.TrimSpace(string(output))
	if token == "" {
		return "", fmt.Errorf("gcloud 回傳空的 ID token")
	}
	return token, nil
}
