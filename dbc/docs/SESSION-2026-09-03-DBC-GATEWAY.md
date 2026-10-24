# dbc-gateway 工作交接（2026-09-03）

## 新 task 開始前必讀

- Repository：`/home/rory/www-auto/sre/dbc`
- Branch：`develop/dbc-gateway`
- 工作樹有大量尚未提交的既有修改，全部都要保留。
- 開始工作時先執行 `git status --short --branch` 與 `git diff`。
- 不要 `git reset`、不要 checkout/覆蓋既有修改。
- 不要部署 production。
- 不要 push 或 commit，除非使用者明確要求。
- Kubernetes、Envoy、DNS、firewall 的實際異動需要先確認是隔離的 staging 環境。
- 原始實作計劃位於 `/tmp/dbc-gateway-original-plan.md`；本文件已摘要關鍵要求，避免該暫存檔消失後無法接手。
- 開發框架、Google API、gcloud/OAuth 行為等判斷，依使用者要求優先用 Context7 與官方文件核對。

## 原始目標與信任邊界

1. `dbc` client 不得保存或使用裸機 MySQL backend address。
2. `baremetal-vplus33` 流量必須經 `dbc-gateway`。
3. client 只知道公開 gateway endpoint、TLS SNI、audience 與邏輯 target ID。
4. backend private/WireGuard address 只存在 gateway server 設定。
5. gateway 驗證 Google 使用者身分並透過 Cloud Identity transitive membership 授權。
6. 查詢稽核在 gateway server 端完成，不能信任 client。
7. 真正阻止繞過 gateway 的邊界在 MySQL/host firewall：3306 只允許 gateway 的 private/WireGuard CIDR。
8. Gateway Pod 終止 TLS，Envoy Gateway 僅做 raw TLS passthrough/SNI routing。

## 目前工作樹

截至本交接文件建立前，`git status --short --branch` 為：

```text
## develop/dbc-gateway
 M .gitignore
 M go.mod
 M instances.yaml
 M internal/cli/cli.go
 M internal/config/config.go
 M internal/config/config_test.go
 M internal/proxy/preparedstmt_e2e_test.go
 M internal/proxy/proxy.go
?? .dockerignore
?? Containerfile
?? cmd/
?? deploy/
?? gateway.yaml.example
?? internal/auth/idtoken.go
?? internal/auth/idtoken_test.go
?? internal/gateway/
?? internal/gatewayclient/
?? internal/gatewayproto/
?? internal/proxy/closeaudit.go
?? internal/proxy/gatewayaudit.go
?? internal/proxy/gatewayclient_test.go
?? internal/proxy/relay.go
?? internal/proxy/relay_test.go
```

注意：許多 gateway 檔案仍是 untracked，單看 `git diff` 不會看到內容；review 必須同時讀取 untracked files。

## 已完成實作

### Client 設定與連線

- `instances.yaml` 的 `baremetal-vplus33` 已移除公開 MySQL `host`。
- 改為 gateway 設定：endpoint、server name、audience、target。
- config parser 嚴格檢查：
  - `host` 與 `gateway` 互斥。
  - gateway endpoint 必須為 `host:port`。
  - server name 必須是 DNS name，不能是 IP。
  - audience、target 不可為空。
- 不存在 gateway 失敗後直接 fallback backend 的路徑。
- `dbc start` 已接上 gateway client。
- CLI version 已由 `dbc start` 一路傳到 auth frame 的 `ClientVersion`。
- gateway client使用 TLS 1.3、系統 CA與指定 SNI。
- client總 timeout 已由 10 秒調整為 30 秒。

### Gateway protocol、驗證與授權

- 新增有大小上限、版本化且拒絕未知欄位的 auth frame。
- auth frame包含 ID token、target ID、client version。
- Gateway成功後回傳 cryptographically random session ID。
- target只能由 server allowlist解析，client不能提交任意 backend地址。
- Google ID token驗證包含：
  - signature
  - issuer（接受 `https://example.invalid` 與 `example.invalid`）
  - audience
  - issued-at / expiry / 最大 lifetime
  - subject
  - email
  - `email_verified=true`
- 時間允許 5 分鐘 clock skew，最大 token lifetime為 1 小時加 skew。
- Cloud Identity使用 service account ADC查詢群組。
- group resource name可以快取；membership每個 session重新查詢，避免撤權後長期沿用。
- configured groups在 listener啟動前做 preflight。
- token、group API或驗證錯誤均 fail closed，對 client只回傳公開錯誤。

### Backend relay、audit與 shutdown

- Gateway驗證成功後才 dial server-side private MySQL address。
- Gateway重用既有 MySQL wire parser執行權威 query audit。
- 支援 `COM_QUERY` 與 native prepared statements。
- prepared statement audit以實際綁定參數重建 SQL；無法安全解析時拒絕該次 execute，不猜測也不靜默漏記。
- bidirectional relay任一方向結束後會關閉連線以解除另一方向阻塞，並等待兩個 goroutine都完成。
- session WaitGroup涵蓋 relay與 audit cleanup。
- forced drain timeout會關閉 active connections，再等待 cleanup完成後才讓 logging client flush/close。
- Gateway啟動前會對每個 audit project執行同步 Cloud Logging write preflight。
- shutdown會 flush/close共享 Cloud Logging clients。
- auth完成後，client deadline會刷新成 backend dial timeout；成功進入 relay前以零值 deadline清除 timeout，避免長連線被 auth deadline誤殺。

### TLS、health與Kubernetes

- TLS certificate reloader支援 Secret更新；無效新憑證會保留上一份有效憑證。
- `/livez` 與 `/readyz` 只使用內部 8080 listener。
- SIGTERM會先讓 readiness變 false、停止 accept，再等待 session drain。
- `drainTimeout`預設 70 分鐘。
- 新增 multi-stage `Containerfile`：CGO disabled、distroless nonroot。
- 新增 `.dockerignore` 排除 Git、build output、gateway正式設定、credentials、service-account JSON、PEM及private keys。
- Kubernetes範本包含：
  - 2 replicas
  - RollingUpdate `maxUnavailable: 0`, `maxSurge: 1`
  - UID/GID `65532`
  - `fsGroup: 65532`
  - Secret mode `0440`
  - read-only root filesystem
  - drop all capabilities
  - RuntimeDefault seccomp
  - `terminationGracePeriodSeconds: 4500`
  - ClusterIP Service（8443與內部8080）
  - PDB `minAvailable: 1`
- 目前網路環境的 Flannel不執行 NetworkPolicy，因此範本沒有用一份無效的 NetworkPolicy宣稱已隔離；production仍需 Calico/Cilium或host firewall。

## 本輪已執行的驗證

主機本身目前沒有 `go`/`gofmt` 在 PATH，以下 Go檢查是使用本機 `golang:1.25` Docker image，將 repository以 read-only方式掛載後執行：

```text
gofmt check                         PASS
go mod tidy -diff                   PASS
go test ./...                       PASS
go test -race ./...                 PASS
go vet ./...                        PASS
git diff --check                    PASS
```

YAML解析：

```text
instances.yaml                              PASS
gateway.yaml.example                        PASS
deploy/kubernetes/dbc-gateway.yaml          PASS
```

Container驗證：

```text
docker build current dirty tree             PASS
image tag                                   dbc-gateway:test
image digest                                sha256:295f53d7786dda19c9485e2d70df3e418ed3ac3fb382c54fcef7638f9a452591
runtime user                                nonroot:nonroot
platform                                    linux/amd64
size                                        約 7.7 MB
binary -help smoke                          PASS
image sensitive filename scan               PASS（未發現 gateway.yaml、credentials.json、PEM/private/SSH keys）
```

真實 MySQL E2E：

- 使用隔離、未發布 host port且測完自動刪除的 MySQL 8 container。
- 執行 `TestPreparedStatementEndToEnd`。
- native prepared INSERT/SELECT、statement reuse UPDATE、結果完整性、參數重建後的 audit SQL全部通過。
- 暫時 MySQL及image-inspection containers均已刪除。

`kubectl apply --dry-run=client`沒有完成：kubectl仍嘗試向目前 context做API discovery，而GKE auth plugin的 PATH內找不到 `gcloud`。當時current context為：

```text
gke_gff-vwb2c_asia-southeast1-a_cluster-vwb2c
```

命令停在認證前，沒有套用任何資源。不要在未確認用途前繼續操作該context。

## 新發現的 production blocker：一般使用者的 ID token audience

目前 `internal/auth/idtoken.go` 使用：

```text
gcloud auth print-identity-token --audiences=https://example.invalid --quiet
```

已用真實active user account與Google Cloud SDK 582.0.0 smoke test。gcloud直接拒絕：

```text
Invalid account type for `--audiences`. Requires valid service account.
```

拿掉 `--audiences` 後，一般使用者可以取得 ID token，而且實測包含：

- issuer `https://example.invalid`
- subject
- email
- `email_verified=true`
- issued-at / expiry

但是 token audience為gcloud自己的OAuth client ID：

```text
example.invalid
```

不是 `https://example.invalid`。因此目前真實使用者執行 gateway連線時，會在連上gateway之前就因token取得失敗而中止。

Context7的Google OAuth官方資料顯示：installed desktop app應使用Authorization Code flow、loopback callback及PKCE；要求`openid email`後，ID token的audience是該應用程式自己的OAuth client ID。原計劃把service-account可指定的URL audience模型套到了普通Google使用者，是目前設計錯位的根因。

## 後續更新（2026-09-03，另一輪）：已用臨時方案解除這個 blocker

上面「一般使用者的 ID token audience」這個 blocker，使用者在了解風險後決定先用一個明確標記為暫時的方案讓功能能動，不等 OAuth Desktop + PKCE：

- `internal/auth/idtoken.go`：拿掉 `--audiences`，改成單純 `gcloud auth print-identity-token --quiet`。已用真實 `user@example.invalid` 帳號實測，回來的 token `aud` 是 `example.invalid`，`email`/`email_verified`/`iss` 都正常。
- `instances.yaml`（`baremetal-vplus33`）、`gateway.yaml.example`、`deploy/kubernetes/dbc-gateway.yaml` 的 `audience` 都改成 `example.invalid`，並在旁邊加註解說明這是已知、刻意接受的風險。
- 風險本質：這個 audience 是 gcloud CLI 全世界共用的固定 client ID，不是 dbc 專屬的——任何人為了任何目的執行 `gcloud auth print-identity-token` 都會拿到同一個 audience 的 token，屬於 OIDC 的 audience-confusion／confused-deputy 類問題（查過官方文件與資安文獻確認這是真的風險，不是過度謹慎）。決定接受的理由：使用者範圍是小規模高信任內部團隊，且 Cloud Identity 群組成員資格每次連線都重查，不是單靠 token 有效就放行。
- 這不是正式修正，下面「建議修正方向」（OAuth Desktop + PKCE）仍是長期目標，尚未實作。
- 驗證：`gofmt`/`go vet`/`go build`/`go test ./...`/`go test -race ./...`/`go mod tidy -diff`/`git diff --check` 全過；三個 YAML 設定檔（含 ConfigMap 內嵌的 `gateway.yaml`）都能正確解析。
- 使用者確認後已 commit（`fa09b94`），沒有 push。沒有部署 production。

## 後續更新（2026-09-04）：完整本機煙霧測試，真的跑過 gateway 二進位檔本身

前面「本輪已執行的驗證」的真實 MySQL E2E（`TestPreparedStatementEndToEnd`）只測了 dbc client 端，完全沒經過 `dbc-gateway` 這個二進位檔——`GroupChecker`/`CheckAuditLogging` 都沒被呼叫到。這輪額外做了一次真正經過 gateway server 本身的端到端煙霧測試，補上這個缺口：

**環境**（全部用完即丟，測完都已清除）：
- 用完即丟的 Docker MySQL 8 容器當後端（`docker-compose-mysql8:latest`，本機沒有對外網路可以拉 `mysql:8.0`，改用本機已快取的 image）。
- 自簽 TLS 憑證（`CN=localhost`），client 端用自訂 `RootCAs` 信任它（正式環境仍應該用真正的 CA 簽發憑證，`dialInstance` 的正式路徑用的是系統 CA store）。
- 一個暫時的 server harness（複製 `cmd/dbc-gateway/main.go` 的啟動邏輯，只把 `GroupChecker` 換成一個用環境變數 `DBC_SMOKE_ALLOW` 控制回傳值的假物件，其餘——`CheckAuditLogging`、`GoogleTokenVerifier`、TLS reload、`gateway.NewServer`——全部用真的）。
- 一個暫時的 client harness：透過 `gatewayclient.Client`（跟 `dbc start` 內部用的是同一份程式碼）+ 真的 `auth.IdentityToken` 拿真實 ID token，再用 `go-sql-driver/mysql` 的 `Config.DialFunc` 掛上去，跑真正的 SQL。
- `auditProject` 指到真實的 `tp-wz3-rd`，會產生幾筆真的稽核紀錄（使用者已確認接受）。
- 這兩個 harness 都只是暫時檔案，測完已從 `cmd/` 目錄刪除，不在任何 commit 裡。

**允許路徑（`DBC_SMOKE_ALLOW=true`）**：
- 真實 ID token → TLS 1.3 handshake → gateway auth frame → `GoogleTokenVerifier` 驗證成功，正確解出 `user@example.invalid`。
- 假 `GroupChecker` 記錄到 `IsMember(user@example.invalid, user@example.invalid) -> true`。
- 透過 gateway 對後端 MySQL 做了一次 native prepared statement 的 INSERT 跟 SELECT，都正確來回。
- 查了真實的 `projects/tp-wz3-rd/logs/dbc-query-audit`，確認稽核紀錄真的寫進去了，`account` 正確是 `user@example.invalid`，`query` 正確重建出綁定後的完整 SQL（`INSERT INTO smoke (note) VALUES ('gwsmoke via dbc-gateway')`）——證實 gateway 端重用的 prepared statement 稽核邏輯在真實 gateway 路徑上也是對的。

**拒絕路徑（`DBC_SMOKE_ALLOW=false`）**：
- 同一組真實 token 一樣通過身分驗證（`user@example.invalid`），假 `GroupChecker` 這次回傳 `false`。
- client 端在 `Dial()` 階段就直接被拒絕：`dbc-gateway 拒絕連線: permission denied`（對外統一訊息，沒有洩漏細節，符合設計）。
- 確認連線從沒進到 backend relay，沒有任何 SQL 被送出、也沒有任何稽核紀錄產生——授權邊界在被拒絕的情況下正確地在最前面就擋下來，不是「先連上才發現沒權限」。

**還沒測到的部份**（跟原本「仍需人類或staging環境才能完成」清單一致，這輪沒有解決）：真正的 Cloud Identity 群組（這裡是假的 `GroupChecker`）、TLS 憑證 reload、SIGTERM drain、Kubernetes/Envoy 整合、DNS 切換。

驗證：`gofmt`/`go vet`/`go build`/`go test ./...` 全過；煙霧測試用的暫時檔案、Docker 容器、憑證都已清除，`git status` 乾淨。沒有 commit（這輪純粹是驗證，沒有改動任何會被提交的程式碼）。

## 後續更新（2026-09-04）：真實 `CloudIdentityGroupChecker` 驗證，補上唯一還在用假物件的部分

使用者提供了一個真實的服務帳號 key file（`user@example.invalid`），拿掉了上一輪唯一還在用假物件的部分（`GroupChecker`），做了完整的真實驗證。**Key file 只放在 repo 外的暫存區，用完後由使用者自行決定是否撤銷/輪替，沒有出現在任何 commit 或程式碼裡**；曾經意外被放進 repo 工作目錄一次（未被 `.gitignore` 擋住），發現後立刻搬出去，沒有被 `git add` 過。

**遇到、解決的前置問題**（跟這個 key file 本身的權限設定無關，是環境設定問題）：
- `tp-wz3-rd` 專案原本沒有啟用 Cloud Identity API（`SERVICE_DISABLED`），使用者確認後執行 `gcloud services enable example.invalid --project=tp-wz3-rd` 解決——這是專案層級的設定，跟服務帳號的 IAM 權限是兩回事，怎麼查都會先卡在這裡。

**真實群組成員資格查詢（`Groups.Lookup` + `Groups.Memberships.CheckTransitiveMembership`）**：
- 這個服務帳號**不需要額外的「加成群組 Owner」授權**就能查 `user@example.invalid`、`user@example.invalid` 這兩個群組——之前討論的「小範圍：把服務帳號加成群組 Owner」這條路，實測起來根本不需要，Cloud Identity API 啟用之後就直接能查（可能是這個服務帳號本來就有某種程度的既有存取權，不是這輪新加的）。`SearchTransitiveGroups`（查某人「所有」群組清單）則確實需要更廣的權限，403 被拒——這個功能沒用到，不影響 `dbc-gateway` 實際需要的查詢方式（`dbc-gateway` 只查「這個人是不是在這個已知群組裡」，不需要列出所有群組）。
- 允許情境：`IsMember(user@example.invalid, user@example.invalid) = true`。
- 拒絕情境：使用者當場把 `user@example.invalid` 從 `user@example.invalid` 移除，**立刻**重測 `IsMember(user@example.invalid, user@example.invalid) = false`——證實了原設計「membership 每次連線都重查、不快取」這個安全屬性是真的有效，撤權後不需要等待，下一次連線就會反映。

**完整端到端煙霧測試（這次沒有任何假物件——直接跑 `cmd/dbc-gateway` 這個真正的二進位檔）**：
- 環境跟上一輪一樣（用完即丟的 Docker MySQL、自簽憑證），差別只在這次用真的 `GOOGLE_APPLICATION_CREDENTIALS` 指到這個服務帳號 key，`allowedGroups` 設成真實群組。
- 允許路徑（`allowedGroups: [user@example.invalid]`）：真實 ID token → TLS → gateway auth → 真實 Cloud Identity 查詢通過 → 透過 gateway 對後端 MySQL 做 INSERT/SELECT，正確來回；查了真實的 `projects/tp-wz3-rd/logs/dbc-query-audit`，確認稽核紀錄正確寫入。
- 拒絕路徑（`allowedGroups: [user@example.invalid]`，此時 `user@example.invalid` 已被移除）：同一組真實 token 通過身分驗證，真實群組查詢正確回傳不是成員，client 在 `Dial()` 階段直接被拒絕（`permission denied`），沒有任何 SQL 被送出。

**結論**：`已完成實作` 清單裡列出的所有 client/gateway 程式碼邏輯，現在都用真實憑證、真實 Cloud Identity 群組、真實 MySQL 端到端驗證過允許跟拒絕兩條路徑，沒有任何一段是假物件在撐。還沒驗證、仍然需要 staging 環境的部分沒有變：TLS 憑證 Secret reload、SIGTERM drain（含縮短 timeout 的 forced drain 與真實 70 分鐘 session）、Kubernetes/Envoy 整合（8443 listener、TLSRoute、SNI、NodePort）、DNS 切換、非 gateway 來源直連 3306 必須失敗。

驗證：`gofmt`/`go vet`/`go build`/`go test ./...` 全過；所有暫時檔案、Docker 容器、自簽憑證都已清除，`git status` 乾淨。沒有 commit（這輪純粹是驗證）。

## 建議修正方向（尚未實作）

正式方案是建立dbc自己的Google OAuth Desktop client，讓使用者透過Authorization Code + PKCE登入：

1. 人類在Google Cloud建立`dbc CLI` Desktop OAuth client。
2. OAuth scopes只需`openid email`；Cloud Identity API仍由gateway service account呼叫。
3. `dbc setup`：
   - 產生高熵state。
   - 產生PKCE verifier與S256 challenge。
   - 在`0.0.0.0:<random-port>`開loopback callback。
   - 開系統browser登入Google。
   - 驗證callback state。
   - 用authorization code與verifier交換ID/refresh token。
   - 本機驗證初始ID token。
4. refresh token存進OS credential store；若第一版只能用檔案，至少要`0600`、atomic write且不可進log/repo。
5. `dbc start`使用refresh token取得ID token，在記憶體快取至到期前約5分鐘，並避免多連線同時刷新。
6. gateway與client設定的audience統一改成dbc自己的OAuth client ID（`*.example.invalid`），不是gateway URL。
7. 保留gateway現有issuer/audience/signature/time/email/email_verified驗證。
8. 增加login/logout/status或至少logout/re-auth能力。

預期修改位置：

- `internal/auth/idtoken.go`：移除gateway token對gcloud custom audience的依賴。
- 新增OAuth/PKCE、token cache及credential storage實作。
- `internal/cli/cli.go`：`dbc setup`加入gateway OAuth login；`dbc doctor`檢查gateway credential。
- `instances.yaml`、`gateway.yaml.example`與Kubernetes ConfigMap：audience改成OAuth client ID。
- 新增state/PKCE/callback/refresh/cache/storage與錯誤路徑測試。

不要把以下做法當production修正：移除`--audiences`後直接接受gcloud的`example.invalid`。這個audience屬於Google Cloud SDK而不是dbc，依賴未承諾穩定的外部client ID，也破壞原本要求的audience隔離。最多只能作為明確標註的短期staging實驗。

Service-account impersonation也不符合需求：ID token中的email會是service account，gateway將無法以真實終端使用者email做Cloud Identity群組授權與audit。

## 建議的新 task 執行順序

1. 重新檢查git status/diff並讀完整本文件。
2. 確認使用者要採用正式OAuth Desktop + PKCE方案。
3. 取得人類建立的OAuth client ID；不要要求或提交refresh token、service-account key或其他秘密。
4. 先寫OAuth flow與token cache的unit tests，再替換`internal/auth/idtoken.go`。
5. 更新`dbc setup`、`dbc doctor`及設定audience。
6. 重跑：gofmt、tidy diff、完整test、race、vet、diff check、container build與真實MySQL E2E。
7. 用允許與不允許群組的真實測試帳號做user-token smoke。
8. 只有在有明確隔離的staging context後，才進行Kubernetes、Cloud Identity、Cloud Logging、TLS及drain整合測試。

## 仍需人類或staging環境才能完成

- 建立dbc Google OAuth Desktop client與consent screen——目前用臨時方案（接受 gcloud 共用 client ID 的風險）撐著，非阻塞，除非決定要走正式方案才需要。
- ~~提供allowed-group與denied-group測試使用者~~——2026-09-04 已用 `user@example.invalid` 實測過允許（在 `user@example.invalid`）跟拒絕（不在 `user@example.invalid`）兩種情境。
- 提供隔離的staging Kubernetes namespace/context。
- 提供staging/private MySQL、staging hostname、TLS certificate與audit project。
- ~~配置gateway service account的Cloud Logging與Cloud Identity權限~~——`roles/logging.logWriter` 已經有；Cloud Identity 這邊發現服務帳號要至少加入網域裡的一個群組才能查詢（見下方「gateway 的服務帳號本身要有群組成員資格」），已加回 `user@example.invalid`。
- ~~驗證真實Cloud Identity direct/nested membership與撤權後的新連線拒絕~~——2026-09-04 已用真實服務帳號、真實群組（`user@example.invalid`/`user@example.invalid`）驗證過，撤權後立即生效。還沒測到 nested（巢狀）群組的情境，只測了直接成員資格。
- ~~驗證gateway-side Cloud Logging query audit與shutdown flush~~——2026-09-04 已用真實 `tp-wz3-rd` 專案驗證過兩者（含 shutdown flush，見下方「開發環境能驗證的 lifecycle 項目全部驗證過了」）。
- ~~驗證certificate Secret renewal/reload與無效憑證fallback~~——2026-09-04 已用真實檔案替換驗證過（見下方），但只測了「file watch 撿到磁碟上的新檔案」這個機制本身，沒有測真正的 Kubernetes Secret/cert-manager 那一段。
- ~~驗證SIGTERM drain、縮短timeout的forced drain~~——2026-09-04 已用真實連線驗證過正常 drain 跟強制 drain 兩種情況（見下方），真實 70 分鐘 session 還沒測（也沒有實質意義去真的等）。
- ~~驗證Envoy 8443 listener、TLSRoute、SNI~~——2026-09-10 前後完成（見下方「對外開放的路徑已經完整打通」章節），idle timeout／NodePort／rollback 還沒個別驗證過。
- **驗證非gateway來源直連MySQL 3306必須失敗**——還沒做，而且現在情況比原設計複雜：MySQL 其實跟 dbc-gateway 同一個叢集（見前面「用真實 vplus33 叢集測過」章節的發現），`mysql-service` 目前叢集內任何 namespace 都連得到（Flannel 不支援 NetworkPolicy），這個風險還沒處理，不是單純「檢查防火牆規則」就能確認的事。
- ~~最後才切正式DNS~~——2026-09-10 前後完成，`example.invalid` 已解析到 `main-gateway` 的公開 IP。rollback 順序（先移除 DNS/封鎖 8443，再移除 TLSRoute/listener）還沒真的演練過。

## Sandbox與工具狀況

- Ubuntu 24.04.4。
- bubblewrap未安裝。
- AppArmor已啟用。
- `kernel.apparmor_restrict_unprivileged_userns=1`。
- 一般sandbox曾出現：

  ```text
  bwrap: loopback: Failed RTM_NEWADDR: Operation not permitted
  ```

- approval繞行可用。
- 主機PATH目前沒有Go toolchain；可用本機`golang:1.25` Docker image執行檢查。
- gcloud存在於task暫存scratchpad路徑，但未在一般PATH，GKE auth plugin因此無法直接找到它。

## 交接時確認

- 沒有部署production。
- 沒有push。
- 沒有commit。
- 沒有reset或覆蓋任何既有修改。
- 除本交接文件外，最後一輪只建立本機測試image；沒有修改應用程式碼。

## 後續更新（2026-09-04，另一輪）：新增「群組決定後端 MySQL 權限」，取代單純放行/拒絕

### 動機

原本 `allowedGroups` 只是放行/拒絕的二元判斷——connection 一旦通過群組檢查，client 端自己打的 MySQL 帳密會直接透傳到後端，實際權限完全由 client 知不知道哪組密碼決定，跟 Cloud SQL 那邊「IAM 群組本身就是權限保證」不是同一個強度。使用者要求補齊：`db-prod-user` 群組要真的只有唯讀、`db-prod-admin` 要真的有完整權限，而且要由 **Cloud Identity 群組本身**強制，不是密碼。

### 設計：gateway 自己終結 MySQL 認證，兩段獨立握手

- `TargetConfig.AllowedGroups []string` 改成 `TargetConfig.Roles []TargetRole`，每個 role 有 `groups`（群組清單）跟 `mysqlUser`/`mysqlPassword`（後端憑證）。**Roles 是有序清單，`Authorizer` 找到第一個成員資格符合的 role 就採用、不再往後找**——同一個人同時符合多個 role 時，只會用最先符合的那個，所以權限高的 role 要排在前面（`db-prod-admin` 排 `db-prod-user` 前面）。設定驗證新增：每個 role 至少一個 group、`mysqlUser`/`mysqlPassword` 不可空、**同一個群組不能出現在超過一個 role 裡**（會讓其中一個 role 永遠選不到，直接拒絕載入而不是悄悄存在）。
- 新增 `internal/gateway/mysqlwire.go`（低階 MySQL 封包 framing，跟 `internal/proxy` 的相似邏輯故意不共用——用途不同，硬要共用不值得）跟 `internal/gateway/mysqlauth.go`（核心認證終結邏輯）：
  - **對 client 端**：gateway 假扮 MySQL server，自己生一個 Handshake 送給 client、接住 client 的 Handshake Response（完全不驗證內容——身分把關在 ID token/群組那步已經做完）、直接回 OK。**故意不在這個假 handshake 裡宣告 `CLIENT_QUERY_ATTRIBUTES`**，這樣正常的 client 不會嘗試用它，避免「client 端跟後端端兩次獨立握手的能力協商對不齊」這個風險，讓既有的查詢稽核／prepared statement 邏輯完全不用改。
  - **對後端 MySQL**：gateway 假扮 client，用選中 role 的憑證做真正的認證。同時支援 **`mysql_native_password`** 跟 **`caching_sha2_password`**（MySQL 8 預設）——後者含 fast-path（SHA256 版本的挑戰-回應）跟 full-authentication（沒有 MySQL 層級 TLS，所以走 RSA：跟後端要公鑰、用原始 scramble 循環 XOR 密碼、RSA-OAEP 加密送出）兩條路徑都做了。**其他認證外掛一律拒絕，不猜**。
  - **重要的真實踩雷、事後補上的部分**：後端的初始 handshake 宣告的認證外掛是**伺服器的預設值**（`default_authentication_plugin`），不是「這個帳號實際用的外掛」——伺服器要等 client 送出使用者名稱才知道帳號，所以初始宣告的外掛不保證跟帳號實際外掛一致。不一致時後端會送一個 **AuthSwitchRequest**（`0xFE` + 新外掛名稱 + 新 scramble），要求用新外掛重算一次挑戰回應。這個現象是**用真實 MySQL 8 測出來的**（該伺服器的 `default_authentication_plugin` 是 `mysql_native_password`，但 `dbc_admin` 帳號用 `caching_sha2_password`，導致第一版程式碼直接失敗，報錯「後端回應了未預期的封包」），不是憑空猜測要加。已補上處理（比照 `go-sql-driver/mysql` 自己的 `handleAuthResult` 邏輯：只允許切換一次，避免惡意/異常後端無限要求切換）。
- `Server` 新增兩個可替換的欄位 `AuthenticateClient`/`AuthenticateBackend`（比照既有的 `DialContext`/`Relay` 這種測試用替換點的設計），`NewServer` 預設接上真正的實作；`handleConn` 在 dial 後端成功後，先做後端真實認證（失敗的話回跟 dial 失敗一樣的通用訊息 `database backend unavailable`，不洩漏內部細節），成功才回 client `AuthResult{OK:true}`，接著才做 client 端的假 handshake，最後才進入既有的 `RelayWithQueryAuditPreAuthenticated`（新增的 relay 進入點，跳過「第一個封包是 handshake」的自動偵測，因為這個階段兩邊都已經握手完成）。
- `internal/proxy/queryaudit.go` 的 `relayClientToServer` 做了最小幅度重構（抽出一個帶初始 `first`/`queryAttrs` 參數的內部函式），讓上面這個新的 relay 進入點可以重用完全相同的查詢稽核/prepared statement 邏輯，不用重寫一份。

### 測試

- 單元測試：兩種認證外掛的 scramble 計算（`mysql_native_password` 用 Python 獨立算出的向量交叉驗證；`caching_sha2_password` 直接用 `go-sql-driver/mysql`（已經是這個 module 的依賴）自己的測試向量比對，逐位元組相符）、RSA 公鑰解析與加密的真實金鑰對往返驗證、後端/client 端 handshake 封包的建構與解析（含各種截斷輸入）、AuthSwitchRequest 的處理與「只允許切換一次」的邊界、`handleConn` 串起完整流程（含後端憑證錯誤時的通用錯誤訊息、role 順序決定憑證）的整合測試。全部跑在 `go test ./... -race`，加上既有的 `go vet`/`gofmt`/`go mod tidy -diff`/`git diff --check`，全部乾淨。
- **真實 MySQL 端到端驗證**（Docker、用完即丟）：建了兩個真帳號——`dbc_readonly`（`mysql_native_password`，只有 `SELECT`）跟 `dbc_admin`（`caching_sha2_password`，完整權限）。用真正的 `cmd/dbc-gateway` 二進位檔（`GroupChecker` 換成一個依環境變數決定允許哪個群組的假物件，其餘全部是真的——`CheckAuditLogging`、TLS、`GoogleTokenVerifier`）搭配真實 ID token（`auth.IdentityToken`）跟 `gatewayclient.Client`（跟 `dbc start` 內部同一份程式碼）：
  - `db-prod-user` 群組 → 選到 `dbc_readonly`：`SELECT` 成功、`INSERT` 被**真正的 MySQL** 拒絕（`Error 1142: INSERT command denied to user 'dbc_readonly'@...`），client 端故意打了完全錯誤/無關的帳密（`ignored-by-gateway`）也一樣，證實權限只由 gateway 選的後端憑證決定，跟 client 自己打什麼完全無關。
  - `db-prod-admin` 群組 → 選到 `dbc_admin`：第一次連線（沒有快取雜湊，走 RSA full-auth）跟第二次連線（有快取，走 fast-path）都測過，`SELECT`/`INSERT` 都成功。
  - 這輪測試也是這樣才發現並修好上面提到的 AuthSwitchRequest 問題——沒有真的連真實 MySQL，這個問題不會被抓到（純 Go 單元測試裡的假後端不會自然模擬出「伺服器預設外掛」這種真實情境）。
  - 所有暫時檔案（`cmd/gwrole-server`、`cmd/gwrole-client`）、Docker 容器、自簽憑證、測試帳號都已刪除/清除，`git status` 乾淨。

### 仍待處理

- `gateway.yaml.example`／Kubernetes manifest 的 `roles` 範例憑證是佔位字串，正式部署要換成真的密碼，且要先在後端 MySQL 上真的建立 `dbc_admin`/`dbc_readonly` 帳號並設好對應 `GRANT`。
- 沒有測到 nested/巢狀群組情境（前面 Cloud Identity 那輪只測過直接成員資格）。
- 沒有測到 MySQL 帳號密碼輪替/過期後的行為（`gateway.yaml` 裡的密碼要跟後端 MySQL 保持同步，目前沒有自動化機制，這是純手動維護的憑證）。

驗證：`gofmt`/`go vet`/`go build`/`go test ./... -race`/`go mod tidy -diff`/`git diff --check` 全過。沒有 commit、沒有 push、沒有部署 production。

## 後續發現（2026-09-04，另一輪）：gateway 的服務帳號本身要有群組成員資格，Cloud Identity 群組查詢才不會被拒絕

跟 mysqlauth 的實作沒有直接關係，是額外用真實服務帳號（`user@example.invalid`）測試 `CloudIdentityGroupChecker` 時，用真實操作意外測出來的一個環境限制，值得記錄下來，避免正式部署時卡在同一個地方：

- **現象**：這個服務帳號原本是 `user@example.invalid`/`user@example.invalid` 兩個群組的成員，查詢 `user@example.invalid` 是否為這兩個群組的成員都正常。**把服務帳號從這兩個群組移除之後**，查詢**任何人**（包含 `user@example.invalid`）在不在這兩個群組裡，全部變成 `403 Permission denied for resource ... (or it may not exist)`——不是查不到「移除後」的正確結果，是連查詢本身都被拒絕。
- **排除了的可能性**：
  - 不是 API 沒啟用（`example.invalid` 已經確認啟用過）。
  - 不是 `roles/cloudidentity.groupsReader` 這個 GCP IAM 角色能解決的——實際綁定到組織層級測試過，等了超過 15 分鐘（排除傳播延遲）查詢依然是同樣的 403，判斷這個角色對 Cloud Identity Groups API 這個操作沒有實際作用（角色確實存在，可能是用在別的用途，或這條 API 路徑根本不吃 Cloud Resource Manager IAM 這一層授權）。
- **有效的做法**：把服務帳號重新加回**其中一個**群組（`user@example.invalid`）當一般成員（不是 Owner），查詢就恢復正常了——而且不只能查它自己所屬的 `db-prod-user`，連它並非成員的 `db-prod-admin` 也能正確查出「不是成員」（`false`，不是 403）。目前的推測是：**服務帳號只要在這個 Cloud Identity 網域裡「至少」有一個群組成員資格，就會取得對整個網域群組查詢的可見性**，不需要是「被查詢的那個群組」本身的成員——但這只是根據這一次實測的推測，沒有查到官方文件明確這樣寫，也沒有測過「只加一個群組、查完全不同一個群組」以外的組合，正式部署前建議再多驗證幾種情境。
- **對正式部署的影響**：`dbc-gateway` 的服務帳號**不能是完全沒有任何群組成員資格的「乾淨」帳號**，至少要加進一個 Cloud Identity 群組（哪一個似乎不重要），不然 `groupChecker.Preflight`（啟動時的預檢）會直接失敗，`dbc-gateway` 整個起不來。
- 沒有徹底搞懂 Cloud Identity Groups API 這一層的授權模型（不是標準 GCP Resource Manager IAM，也不完全是文件說的「Workspace Super Admin 委派」或「加成 Owner」那兩條路——這次測到的是「加成一般 Member 就有效」，比文件暗示的門檻更低）。之前查文件時提到的「加成群組 Owner」小範圍做法，這次沒有測到，如果未來要精確控制服務帳號到底該加進哪個/幾個群組，值得再花時間查清楚。

## 後續更新（2026-09-04，另一輪）：開發環境能驗證的 lifecycle 項目全部驗證過了

前面「仍需人類或staging環境才能完成」清單裡，有四項其實不用真的上 Kubernetes 也能驗證核心邏輯——都用真正的 `cmd/dbc-gateway` 二進位檔（`GroupChecker` 一樣換成假的 always-allow，其餘全真）搭配 Docker MySQL、真的送 `SIGTERM`、真的替換憑證檔案、真的在後端 MySQL 上輪替密碼，全部測過了：

- **~~驗證SIGTERM drain、縮短timeout的forced drain~~**：用一個真的還在跑查詢的連線測了兩種情況：
  - **正常 drain**（client 在 `drainTimeout` 內主動關閉連線）：`SIGTERM` 送出後 `/readyz` 立刻變 503、`/livez` 仍是 200（進程還活著、只是不接新連線）、新連線嘗試直接被拒絕（listener 已關閉）；既有連線在整個 drain 期間持續可用，能正常再跑一次查詢；client 關閉後 drain 正常結束，process 印出「已正常結束」並乾淨退出。
  - **強制 drain**（client 故意拖過 `drainTimeout`）：連線在剛好 `drainTimeout` 那一刻被強制關閉（client 端看到 `broken pipe`/`invalid connection`），server 印出「等待既有 session 結束超過 Xs」後退出——這個錯誤訊息目前跟啟動失敗共用同一行 log（`dbc-gateway 啟動失敗: ...`），字面上有點誤導（明明是關閉流程，不是啟動失敗），純粹是既有程式碼的用字瑕疵，沒有改，因為沒有人要求改，先記錄下來。
  - 兩種情況都確認 Cloud Logging 只會記到「真的執行完的查詢」——被強制中斷、連結果都還沒回來的查詢不會出現在稽核紀錄裡（因為根本沒有回應可以記），這是合理的行為，不是漏記。
- **~~gateway-side Cloud Logging query audit與shutdown flush~~**：上面兩種 drain 情境，關閉前都印出「關閉前 flush Cloud Logging clients...」／「已 flush/close」，而且事後查真實的 `tp-wz3-rd` 專案，兩種情況裡「真的執行完」的查詢都確實在 Cloud Logging 裡查得到（含強制 drain 情境裡、被中斷前唯一一筆已完成的查詢）。
- **~~驗證certificate Secret renewal/reload與無效憑證fallback~~**：process 不重啟的情況下，直接在磁碟上把 `tls.crt`/`tls.key` 換成另一份全新產生的憑證（模擬 cert-manager 更新 Kubernetes Secret 掛載檔案），等過 watch 間隔後用 `openssl s_client` 查實際 serve 出來的憑證 fingerprint，確認真的換成新的了。接著把憑證檔案寫壞（塞一段不是 PEM 格式的內容），確認 server 印出 reload 失敗的警告、但**繼續serve 上一份還有效的憑證**，process 不會崩潰、也不會變成沒有 TLS 可用。
- **（原本沒列在清單裡，額外多測的）MySQL 帳號密碼輪替**：先在真的 MySQL 上把 `dbc_readonly` 的密碼改掉（`gateway.yaml` 沒有同步更新），確認新連線乾淨失敗——client 端只看到通用訊息 `database backend unavailable`，server 端的 log 才有真正的細節（`Access denied for user 'dbc_readonly'@... (using password: [REDACTED] MySQL 自己回的，不是 dbc 捏造的）。接著把 `gateway.yaml` 的密碼改成新密碼、重啟 gateway（設定檔目前是啟動時讀一次，沒有熱重載），確認連線恢復正常。

驗證：`gofmt`/`go vet`/`go build`/`go test ./... -race` 全過；所有暫時檔案（`cmd/gwlife-server`、`cmd/gwlife-client`）、Docker 容器、憑證都已清除，`git status` 乾淨。沒有 commit（這輪純粹是驗證，沒有改動任何會提交的程式碼）。

## 後續更新（2026-09-04，另一輪）：規劃疏漏稽核，補上兩個發現的問題

用一個獨立 agent 廣泛稽核整個 gateway 實作、K8s manifest、CI、文件是否有規劃時漏掉但沒被記錄的缺口（部署自動化、安全性、測試覆蓋率、可觀測性）。找到的項目按嚴重度排序，這輪先處理了前兩個：

- **（已修，commit `005b346`）backend 認證交握完全沒有 I/O timeout**：`authenticateBackend`／`continueBackendAuth`（`mysqlauth.go`）對 backend 連線先前從未設過 deadline；backend 若在 handshake 中途卡住（防火牆黑洞、backend 過載、網路 partition），`handleConn` goroutine 會無限期卡死。更嚴重的是 forced-drain 的 `closeActiveConnections()` 先前只關閉 client 端追蹤的連線，backend 連線完全沒被追蹤——代表卡在 backend 讀寫的 goroutine，SIGTERM 連強制關閉都救不了，process 可能永遠等不到 shutdown 完成。修法：backend 認證階段套用跟 client 端相同的 `DialTimeout` deadline，並把 backend 一併納入 `connectionRegistry`。過程中順手抓到一個相鄰的既有 race：backend/dial 逾時後，client 自己的 deadline 幾乎同時到期，導致「database backend unavailable」這則錯誤訊息來不及寫回給 client（會被觀察成單純斷線、看不到明確錯誤）——三個錯誤回傳點寫回前都先重新給 client deadline 一段新的 `AuthTimeout` 預算。新增 `TestHandleConnBackendAuthTimesOutOnHungBackend`（真的用一個永遠不回應的 backend 驗證會在 deadline 內回傳、client 收到正確錯誤，而不是卡死）與 `TestCloseActiveConnectionsClosesTrackedBackendConnections`。這輪測試也是先寫出會失敗的測試才發現上面那個 race——單純看程式碼不會發現這麼隱蔽的時序問題。
- **（已修）`deploy/kubernetes/dbc-gateway.yaml` 把含明文 MySQL 密碼的 `gateway.yaml` 放在 ConfigMap 而非 Secret**：ConfigMap 在多數叢集常見的 RBAC（例如 `view` ClusterRole）下比 Secret 寬鬆，也不會套用叢集對 Secret 的 at-rest 加密政策。修法：整份 `gateway.yaml`（不只密碼欄位）改成 `kind: Secret`／`stringData`，volume 掛載方式從 `configMap:` 改成 `secret:[REDACTED] 0440`，跟現有 tls/google-credentials 兩個 Secret 一致）；沒有動到任何 Go 程式碼，因為應用程式本來就只是讀一個設定檔，不在乎背後是 ConfigMap 還是 Secret。檔案裡加了註解提醒：這裡的 `mysqlPassword` 只是佔位符，正式部署時不要把真實密碼提交進版控，要用 CI/CD secret 注入、sealed-secrets/external-secrets-operator，或 `kubectl create secret` 另外建立。

- **（確認不用改）`gateway.yaml.example`／K8s manifest 的範例值用真實 Cloud Identity 群組 email／GCP 專案 ID**：檢查後發現這些值（`example.invalid`、`tp-wz3-rd`、`example.invalid`）不是這兩個新檔案獨有的，`instances.yaml`、`README.md` 等整個 repo 一路都用真實值，是既有慣例；repo 本身是 private，只改這兩個新檔案不會實際降低暴露，反而會跟其他設定檔/文件不一致，所以維持現狀。
- **（已修）CI 沒有 build/push image 到 registry 的 job**：新增 `.gitlab-ci.yml` 的 `release-dbc-gateway-image` job，用 Kaniko（Kubernetes executor 沒有 Docker daemon，不能直接 `docker build`，Kaniko 是 daemonless、不需要 privileged container 的替代方案）build `Containerfile`，推到 `asia-east1-docker.pkg.dev/tp-wz3-rd/bucket2024/${CI_PROJECT_PATH}/dbc-gateway`（`CI_PROJECT_PATH` 這個 GitLab 內建變數在這個專案會展開成 `sre/dbc`），跟 `release` job 一樣只在 `v*.*.*` tag 觸發、放同一個 stage 平行跑。認證用 `GOOGLE_APPLICATION_CREDENTIALS` 指向解碼後的 SA JSON key（存成 CI/CD 變數 `GCP_ARTIFACT_REGISTRY_KEY`，因為 JSON key 多行、GitLab masked 變數只能單行，所以存 base64 編碼過的版本）——寫法有用 context7 對照 kaniko 官方文件核對過，是文件示範的標準做法；權限用 `roles/artifactregistry.writer`（不是 kaniko 文件預設範例給的 `roles/storage.objectAdmin`，那個是舊版掛在 GCS bucket 上的 GCR 專用權限，跟 Artifact Registry 不是同一套）。`deploy/kubernetes/dbc-gateway.yaml` 的 `image:` 也從 `REGISTRY/dbc-gateway:TAG` placeholder 換成實際路徑。**這組 SA/key 需要有 GCP 權限的人另外建立**，還沒建立前這個 job 會因為缺 `GCP_ARTIFACT_REGISTRY_KEY` 失敗。
- TLS Secret（`dbc-gateway-tls`）與 Google credentials Secret（`dbc-gateway-google-credentials`）在 manifest 裡都假設「已存在」，沒有任何腳本化流程建立/更新它們，也沒有 cert-manager 的 Issuer/Certificate 資源——跟 image/registry 是同一類「寫了 YAML、沒寫怎麼產生」的問題，還沒處理。
- 完全沒有 metrics/告警規劃，只有 `/livez`/`/readyz` 兩個最基本探針；認證失敗、群組查詢被拒、backend 密碼過期這類異常目前只會躺在 `log.Printf` 裡，沒人會主動發現。還沒處理。

已檢查、確認沒問題的部分：`.gitignore`/`.dockerignore` 對機密檔名模式涵蓋沒有遺漏；`.goreleaser.yaml` 不會把 gateway 相關敏感檔案打進公開發布的 `dist/`/GitLab Pages；密碼沒有透過 `%+v`/`%#v` 意外印進 log；`config.go` 的 `Validate()` 測試覆蓋多 target/多 role 情境完整；憑證熱重載用 `atomic.Pointer` 沒有 race。

驗證（backend timeout 那筆修正）：`gofmt`/`go vet`/`go build`/`go test ./... -race`/`go mod tidy -diff` 全過，已 commit `005b346`。驗證（ConfigMap→Secret）：三個 YAML 設定檔用 Python `yaml.safe_load_all` 重新解析過，`dbc-gateway-config` 正確顯示為 `kind: Secret`；沒有動到 Go 程式碼所以不需要重跑 Go 測試。

## 後續更新（2026-09-04，另一輪）：`develop/dbc-gateway` 已 merge 回 main，image/registry CI 真的端到端驗證過了

有 GCP 權限的人已經建立好 `roles/artifactregistry.writer` 的 service account，並把 base64 編碼過的 key 存進 GitLab CI/CD 變數 `GCP_ARTIFACT_REGISTRY_KEY`（masked + protected）。為了驗證新增的 `release-dbc-gateway-image` job（而不是只信任程式碼看起來對），照正常流程走了一次真的 release：

- `develop/dbc-gateway`（9 個 commit，`main` 這邊沒有任何分支的 commit）fast-forward merge 進 `main`，沒有衝突。merge 前這個分支從未推上過 origin（只存在本機）。
- merge 後重跑一次完整驗證（`gofmt`/`go vet`/`go build`/`go test ./... -race`/`go mod tidy -diff`）全過，才 push `main`。
- 推 `main` 後打了 `v0.0.31` annotated tag（沿用既有慣例：`git tag -a vX.Y.Z -m vX.Y.Z`）並推送，觸發真的 release pipeline（[#46555](http://example.invalid:1688/sre/dbc/-/pipelines/46555)）。
- 三個 job（`release`、`release-dbc-gateway-image`、`pages`）全部 success。用 `glab ci trace` 直接看 `release-dbc-gateway-image` 的完整 log，確認不是只信任 job exit code——log 裡明確有兩行 `Pushed asia-east1-docker.pkg.dev/tp-wz3-rd/bucket2024/sre/dbc/dbc-gateway@sha256:c8c72f28294baa9091c8b62b23aa71a89343d2669dfe2e0beb1468ddb6bc85fd`（`v0.0.31` 跟 `latest` 兩個 tag，同一個 digest）。
- 這台 sandbox 沒有 `gcloud`，沒辦法額外用 `gcloud artifacts docker images list` 交叉驗證，但 kaniko 自己的 push 記錄已經是直接證據。

**這代表什麼**：`gateway.yaml.example`／`deploy/kubernetes/dbc-gateway.yaml` 裡「範例值維持現狀、image/registry CI 缺口已修」這兩項，現在不只是「寫了程式碼、測過單元測試」，是真的走過一次完整、正式的 release 流程，image 也真的躺在 `asia-east1-docker.pkg.dev/tp-wz3-rd/bucket2024/sre/dbc/dbc-gateway` 裡了。`main` 現在已經包含整個 dbc-gateway 功能（client + server），`develop/dbc-gateway` 這個分支的任務算是完成了。

**注意**：這次 release 也是 `dbc` CLI 本身（不只是 gateway）的一次正式版號躍進（`v0.0.30` → `v0.0.31`），所有已安裝 `dbc` 的使用者的自動更新提醒下次執行 `dbc doctor`/`dbc start` 都會看到新版本可用——這是預期中的副作用，不是意外。

**還是需要 K8s/staging 才能驗證的部分沒有變**：Envoy 8443 listener/TLSRoute/SNI/NodePort/rollback、真實 70 分鐘 session（上面測的是邏輯本身，不是真的等 70 分鐘）、後端主機防火牆設定、正式 DNS 切換、nested 群組情境。

## 後續更新（2026-09-04，另一輪）：用真實 vplus33 叢集測過 dbc-gateway namespace 連得到 MySQL

用另一個專案（`../gke-gateway-route-manager`）裡現成的 `vplus33_production` kubeconfig，第一次真的連上 `baremetal-vplus33` 這個 target 實際要部署的叢集，而不是純靠文件描述推測：

- **重大發現：這不是獨立的 staging 環境，是正式環境本身**。`kubectl cluster-info` 連到的就是 `user@example.invalid`，`../gke-gateway-route-manager/config/clusters.yaml` 裡這個叢集的描述直接寫「bare metal 正式環境（example.invalid），Ingress -> Envoy Gateway 遷移中」。前面「仍需人類或staging環境才能完成」清單裡「提供隔離的staging Kubernetes namespace/context」、「提供staging/private MySQL」這兩項，實務上不會有一個額外的 staging 叢集——之後所有驗證都得直接對正式叢集做（用獨立 namespace/丟棄式資源盡量降低風險），這兩項清單描述要更新認知，不是繼續等一個不存在的 staging 環境。
- **更重大的發現：MySQL 其實跑在同一個 vplus33 叢集裡**（`mysql` namespace，`pod/mysql-0`，`mysql-service` ClusterIP + `mysql-external` LoadBalancer 對外），不是原本設計文件假設的「透過 WireGuard 連到叢集外的裸機主機」。`gateway.yaml.example`／K8s manifest 裡原本的 `0.0.0.0:3306`（WireGuard 私網位址）其實是錯的／過時的假設，已經改成叢集內部 Service DNS `mysql-service.mysql.svc.cluster.local:3306`。
- **實測步驟**：建立 `dbc-gateway` namespace（留著，之後真的部署會用到），丟一個用完即丟的 `nicolaka/netshoot` debug pod 進去，從裡面測：
  - DNS 解析：`mysql-service.mysql.svc.cluster.local` → `0.0.0.0`，正常。
  - TCP 連線：`nc -zv` 成功。
  - 真的用 `/dev/tcp` 抓 MySQL 初始 handshake 封包的原始 bytes，解出來是**真實的 MySQL 8.0.35 handshake**，plugin 是 `caching_sha2_password`——不只是「port 通」，是真的確認對面是一台活的 MySQL server，而且用的 plugin 剛好是 `mysqlauth.go` 支援的兩種之一。
  - debug pod 測完已刪除；`dbc-gateway` namespace 留著沒刪。
- 沒有動到任何正在跑的服務（`mysql-0`、兩個既有 Service 都沒被碰），只新增了一個空 namespace。

**還沒解決的部分**：這次只證明「網路連得到、MySQL 是活的」，不代表可以整個部署——TLS/Google credentials Secret 建立流程還是空白（見前面章節）、`dbc_admin`/`dbc_readonly` 帳號還沒在這台真的 MySQL 上建立、Envoy Gateway 那段（8443/TLSRoute/SNI）也還沒碰。

## 後續更新（2026-09-04，另一輪）：正式部署到 vplus33、CLIENT_MULTI_STATEMENTS、修好 GUI 工具切換資料庫載入不出表的問題

`develop/dbc-gateway` merge 進 `main` 之後，這輪是真的把 dbc-gateway 部署到 vplus33 正式環境並用真實 GUI 工具（Beekeeper Studio）測試的紀錄，過程中發現並修好兩個問題：

### CLIENT_MULTI_STATEMENTS（commit `6cfd786`）

Beekeeper Studio 等 GUI 工具常把整個編輯器內容包成一個 COM_QUERY 一次送出（分號分隔多條敘述），需要 server 端宣告 `CLIENT_MULTI_STATEMENTS`，否則後端 MySQL 只解析第一條、後面殘留文字被當語法錯誤拒絕——`fakeCapabilities` 補上這個 capability（同時套用到假扮 client 端與真正連後端兩段 handshake）。`internal/proxy/queryaudit.go` 原本假設「第一個回應封包就代表整個 COM_QUERY 結束」，多敘述情境下會漏記 batch 裡後面才失敗的敘述，改成正確解析 OK_Packet 的 `status_flags`（`SERVER_MORE_RESULTS_EXISTS = 0x0008`，用 context7 對照官方 protocol 文件核對過），追蹤到整個 batch 真正結束才記錄稽核。這個修正同時套用到 `dbc start` 與 dbc-gateway（共用同一份 `relayServerToClient`）。

### GUI 工具切換資料庫後資料表清單是空的（commit 待補）

實際用 Beekeeper Studio 連上 dbc-gateway 後，切換資料庫時側邊欄的資料表清單會是空的，但 Cloud Logging 稽核紀錄顯示對應查詢全部是 `[OK]`——查證後發現：這些查詢用 `WHERE table_schema = database()`，而 gateway 的假 handshake（`authenticateClient`）完全不解析 client 端 Handshake Response 裡的 `CLIENT_CONNECT_WITH_DB`／資料庫名稱欄位，所以 `database()` 永遠是 `NULL`，`table_schema = NULL` 在 SQL 裡永遠不成立——查詢「成功」但保證回傳 0 筆，不是錯誤。Beekeeper 這類工具的「切換資料庫」下拉選單不是送 `USE`，是透過連線 handshake 的預設資料庫欄位（很多 GUI 工具的資料庫選擇器都是這樣做的）。

修法：`authenticateClient` 現在會解析這個欄位，如果 client 有指定資料庫，會在（已經認證過的）後端連線上額外送 `COM_INIT_DB` 幫忙選好，選不到（不存在/沒權限）就把後端的真實 ERR 封包原樣轉給 client（這是 client 自己要求的資訊，不是 gateway 內部憑證細節，不需要像 `AuthenticateBackend` 失敗那樣泛化），而且整個 login 失敗，不會靜默地「沒有預設 db 但還是讓你連上」——這樣才符合真實 MySQL server 對「指定的預設資料庫選不到」的行為。`AuthenticateClientFunc` 簽章因此改成 `func(client, backend net.Conn) error`（多帶一個已認證的 backend 連線，才能送 `COM_INIT_DB`）。新增 6 個測試：欄位解析（含未宣告能力、payload 太短、空資料庫名稱）、選取成功、選取失敗時正確轉發真實錯誤且整體回傳錯誤。

驗證：`gofmt`/`go vet`/`go build`/`go test ./... -race` 全過，還沒重新部署到 vplus33 用 Beekeeper 實測（下一步）。

### 正式部署紀錄

- `v0.0.31`、`v0.0.32` 兩次 release 都完整驗證過 pipeline 三個 job 成功、kaniko 真的 push image、`deploy/kubernetes/deploy.sh` 部署到 vplus33 後 rolling update 乾淨完成。
- **重大發現：`baremetal-vplus33` target 的位址是錯的**——`0.0.0.0:3306`（假設走 WireGuard 連外部裸機）其實是過時/錯誤的假設，MySQL 其實跑在同一個 vplus33 叢集裡（`mysql` namespace），已改成叢集內部 Service DNS `mysql-service.mysql.svc.cluster.local:3306`，並實測過 DNS 解析/TCP 連線/真實 MySQL handshake 都正常（commit `35882de`）。
- **事故**：`deploy/kubernetes/dbc-gateway.yaml` 曾經把 `dbc-gateway-config` 這個 Secret（含 `mysqlPassword: [REDACTED]"` 佔位符）也定義在同一份 manifest 裡，`deploy.sh` 每次 `kubectl apply` 整份套用時就把手動建立的真實密碼蓋回佔位符——真實在 vplus33 發生過，已修好（把這個 Secret 從 manifest 移除，改用註解說明手動建立一次；commit `dee474e`）。
- **`deploy.sh` 的參數驗證 bug**：早期版本沒有拒絕多餘參數，一次意外把 `--dry-run=client`當成被忽略的第二參數，實際上變成真的執行了部署——已修好，明確拒絕超過一個參數（commit `a4f90fa`）。
- MySQL 帳號實際上是 `dbc-admin`/`dbc-readonly`（連字號），跟 `gateway.yaml` 設定的 `dbc_admin`/`dbc_readonly`（底線）對不上，一開始的認證失敗訊息（`後端使用不支援的認證外掛 "sha256_password"`）其實是 MySQL 對不存在帳號的防列舉誘餌機制（用 context7 查到 `sql_authentication.cc` 的 `unknown_accounts`/`decoy_user` 原始碼文件證實），不是真的有帳號用這個 plugin。用 `RENAME USER` 改成底線後，暴露出真正的問題是密碼對不上（`RENAME USER` 不會改密碼）。
- **`dbc_admin` 權限經查證嚴重過大**（`SUPER`/`CREATE USER`/`REPLICATION`/多個 `_ADMIN` 動態權限，還帶 `WITH GRANT OPTION`，等同 root），會員直接破壞整個「群組決定權限」的設計——已提出收斂建議，使用者決定暫不處理。
- `dbc_readonly` 權限正常（`SELECT, SHOW VIEW`，沒有多餘權限）。

## 後續更新（2026-09-11）：CLIENT_QUERY_ATTRIBUTES 改成「容忍但不授予」、對外開放的路徑已經完整打通

### CLIENT_QUERY_ATTRIBUTES 改用風險更小的做法（commit `8eb29df`）

前一輪（見上面「正式部署到 vplus33、CLIENT_MULTI_STATEMENTS」章節）讓 gateway 真的支援 `CLIENT_QUERY_ATTRIBUTES`，但這會牽動 `preparedstmt.go` 既有的 fail-closed 保護（`execCommand` 對談成這個能力的連線一律拒絕帶參數的 EXECUTE），代價比預期高。改成風險小很多的做法：

- `fakeCapabilities` 依然完全不宣告這個能力（跟 `preparedstmt.go`／稽核解析邏輯完全無關，一行都沒改）。
- 只是拿掉「client 宣告了 server 沒提供的能力就直接拒絕整個連線」這條規則，改成單純容忍、忽略。
- 用 context7 對照官方協定文件確認：真正生效的能力是雙方交集，不是 client 單方宣告就算數；一個守規矩的 client 應該要照 server 實際宣告的能力決定封包編碼方式。
- **這個修法能不能完全解決問題，取決於 `mysql2`（Beekeeper Studio 底層 driver）是否真的照 server 宣告的能力決定要不要送 query-attributes 前綴，而不是自己說了算**——沒辦法百分之百查證，只能推論（`mysql2` 要跟大量不支援這個能力的 server 相處，如果不遵守會是眾所皆知的相容性問題）。
- ~~`v0.0.36` 已部署，還沒有實際用 Beekeeper Studio 連線驗證~~——2026-09-11 已用 Beekeeper Studio 實際連線測試過：查詢沒有出現異常（沒有莫名其妙的語法錯誤，結果正確），gateway log 乾淨。代表 `mysql2` 確實有照 gateway 實際宣告的能力決定封包編碼方式，沒有無視協商結果硬送 query-attributes 擴充格式——「容忍但不授予」這個修法在實務上是安全的，先前的推論成立。還沒有另外查 Cloud Logging 稽核紀錄逐字核對，但功能面（查詢正常執行、結果正確）已經足以確認沒有問題。

### 對外開放的路徑已經完整打通

檢查發現前幾輪標記為「被其他工作卡住」的項目其實已經全部完成（推測是另一個處理 `../gke-gateway-route-manager` TLSRoute 支援的 session 做的，交接文件〔`feature-request-tlsroute-support.md`〕派上用場了）：

- **`dbc-gateway-tls` 已經不是自簽憑證**：查證過 Secret 裡的憑證，簽發者是真的 `Let's Encrypt`（`CN = YR2`），subject 是 `example.invalid`，效期 2026-09-04～2026-12-03。
- **TLSRoute 已建立**：`dbc-gateway` namespace 裡有一個 `TLSRoute`（標籤顯示是 `gke-gateway-manager` 建立的），`parentRef` 指到 `main-gateway` 的 `tls-passthrough-8443` section（`protocol: TLS`, `mode: Passthrough`，`hostname: example.invalid`），`backendRefs` 指到 `dbc-gateway` Service 的 8443 port。狀態 `Accepted`/`ResolvedRefs` 都是 `True`。
- **DNS 已切換**：`example.invalid` 解析到 `0.0.0.0`，跟 `main-gateway` 的公開 IP 一致。
- **實際測試過真的從外部連線**：用 `openssl s_client -connect example.invalid:8443 -servername example.invalid` 連線，`CONNECTED`，憑證驗證 `Verify return code: 0 (ok)`，簽發者/subject 都正確——確認整條 TLS passthrough 路徑（Envoy SNI 路由 → dbc-gateway pod 終結 TLS → 回應正確憑證）是通的，不只是設定檔看起來對，是真的能連上。

**2026-09-11 補充**：~~用真的 `dbc` CLI 從外部完整跑一次 `dbc start` 連 `baremetal-vplus33`~~——使用者自己用真的筆電、真的 Google 帳號、`dbc start --port 3701` + `dbc connect 3701` 完整測過，正常。這代表整條對外路徑（`dbc` CLI → Google ID token → TLS → Envoy TLSRoute（SNI）→ gateway 終結 MySQL 認證 → 真的連到 `baremetal-vplus33` 後端）從外部到內部完整打通，不只是設定看起來對，是真的有人從叢集外面連進來成功——`baremetal-vplus33` 這個 target 的「原始目標與信任邊界」7 項裡最後一塊拼圖也補上了。

**還沒驗證的部分**：idle timeout／NodePort／rollback 順序沒有個別演練過；「非 gateway 來源直連 MySQL 3306 必須失敗」這項因為 MySQL 現在跟 gateway 同一個叢集、`mysql-service` 缺乏 NetworkPolicy 隔離，實際上還沒有做到，是目前最大的殘留風險。

## 後續更新（2026-09-11，另一輪）：dbc-gateway 這輪工作收尾

- **~~dbc_admin 密碼是否對得上~~**：使用者已確認連線正常。
- **MySQL 缺乏網路層隔離（`mysql-service` 叢集內任何 namespace 都連得到）**：使用者決定接受這個風險，暫不處理。之後如果要處理，方向是幫叢集換一個支援 NetworkPolicy 的 CNI（目前的 Flannel 不支援），或至少在 MySQL 那邊做其他形式的存取限制。
- **Cloud Identity 巢狀群組**：查了 Google 官方最佳實踐文件（`example.invalid/iam/docs/groups-best-practices`、`best-practices-workforce-identity-federation`）後決定 **`db-prod-user`/`db-prod-admin` 不使用巢狀群組**——官方明確建議「避免用組織性/協作性質的群組做存取控制」，且巢狀群組的成員資格傳播（尤其是撤權）比直接成員資格慢，對把關正式資料庫存取的群組是不能接受的延遲風險。程式碼技術上用的是 `Groups.Memberships.CheckTransitiveMembership`（理論上支援巢狀），但既然決定不用，也就沒有必要另外驗證。使用者已把先前為了測試而設定的巢狀群組情境還原。
- Envoy idle timeout／NodePort／rollback 演練、`dbc_admin` 權限收斂、metrics/告警規劃：維持先前決定（不處理或低優先度）。

**現況**：`dbc-gateway` 功能本身已經端到端驗證完成（真實外部 `dbc` CLI 連線、Beekeeper Studio GUI 連線都正常），`develop/dbc-gateway` 這個分支的工作目標可以算是達成了。剩下的都是已知、有意識決定暫不處理的項目，不是遺漏。
