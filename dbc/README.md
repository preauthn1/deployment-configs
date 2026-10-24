# dbc

> 本目录原始 README 见文末，本节为快照补充信息。

原始路径：`dbc`　|　文件 **63**　|　主语言：Go 53, YAML 4, Shell 2

## 目录构成

```
internal/gateway/  — 19
internal/proxy/  — 12
internal/admin/  — 5
internal/auth/  — 3
deploy/kubernetes/  — 3
docs/  — 2
internal/upgrade/  — 2
internal/gatewayclient/  — 2
internal/config/  — 2
internal/gatewayproto/  — 2
internal/cli/  — 2
internal/doctor/  — 1
```

## 关键文件

- CI/构建：`.gitlab-ci.yml`

---

## 原始说明

# dbc

用個人 Google 帳號的 IAM 驗證，連線到公司在 GCP Cloud SQL (MySQL) 上的正式資料庫，不用共用帳密、不用另外安裝 `cloud-sql-proxy`。

## 目錄

- [安裝](#安裝)
- [快速開始](#快速開始)
- [指令](#指令)
- [`dbc start` 的行為](#dbc-start-的行為)
- [Port 對照表](#port-對照表)
- [已知限制](#已知限制)
- [自助排查](#自助排查)
- [管理指令（`dbc admin`）](#管理指令dbc-admin)
- [開發](#開發)

## 安裝

發布二進位檔案是透過 GitLab Pages（`http://example.invalid:1688`）——這是公開的，不需要登入或 GitLab token；repo 原始碼本身仍是 private，兩者是分開的設定。

**macOS / Linux：** 用 `install.sh`：

```bash
curl -fsSL http://example.invalid:1688/install.sh | bash
```

裝到 `~/.dbc/`，並在 `~/.local/bin/dbc` 建捷徑（記得把 `~/.local/bin` 加進 `PATH`，腳本會提醒）。安裝指定版本：`... | bash -s -- v0.0.2`。

macOS 上透過 `curl` 下載的檔案不會被標記 `com.apple.quarantine`（那是 Safari／Finder 這類 GUI 下載器才會加的），所以**不會跳出 Gatekeeper 警告，也不需要手動 `xattr`**。Linux 沒有這個問題。

<details>
<summary>備案：手動下載（如果 install.sh 用不了）</summary>

到 [Releases](../../releases) 下載對應平台的壓縮檔，解壓後把 `dbc` 連同 `instances.yaml` 放在同一個資料夾。**macOS** 因為是透過瀏覽器下載，第一次執行前要先解除 Gatekeeper 的隔離標記（binary 未經簽章）；**Linux** 沒有這個問題，解壓後直接執行即可：

```bash
xattr -d com.apple.quarantine ./dbc
```

新版 macOS（Ventura / Sequoia 之後）光拿掉這個標記可能還不夠，Gatekeeper 會另外跳出「Apple 無法驗證『dbc』是否為惡意軟體」的視窗。這種情況要另外手動放行：

1. 開啟**系統設定 → 隱私權與安全性**
2. 往下捲，會看到一行 `dbc` 被封鎖的提示
3. 點旁邊的**仍要打開**，輸入密碼或用 Touch ID 確認

（根治需要 Apple Developer 帳號做 codesign + notarize，目前先用 `install.sh` / 上面兩步驟繞過。）

</details>

**Windows：** 用 `install.ps1`，不需要 Git Bash 或 WSL：

```powershell
iwr -useb http://example.invalid:1688/install.ps1 | iex
```

裝到 `%LOCALAPPDATA%\dbc\`，並自動加進使用者 `PATH`（新開一個終端機視窗才會生效）。安裝指定版本：先 `$env:DBC_VERSION = "v0.0.2"` 再執行上面的指令。

下載下來的檔案 Windows 會標記成「來自網際網路」，第一次執行可能被 SmartScreen 擋一次；腳本裝的時候會自動 `Unblock-File` 處理掉這個標記。

<details>
<summary>備案：手動下載（如果 install.ps1 用不了）</summary>

到 [Releases](../../releases) 下載對應壓縮檔，解壓後把資料夾放到任意路徑，用 PowerShell 或 cmd 執行 `dbc.exe` 即可。

</details>

### 前置需求

- [gcloud CLI](https://example.invalid/sdk/docs/install)
- `mysql` client（`dbc connect` 會呼叫它）

## 快速開始

```bash
dbc setup      # 第一次使用：登入 gcloud、設定 ADC
dbc doctor     # 檢查環境是否就緒
dbc list       # 查看 port 對照表
```

開一個終端機視窗掛代理（保持執行，不要關閉）：

```bash
dbc start                # 掛所有從庫（日常查詢）+ 沒有從庫的主庫（如 chat db）
```

另開一個終端機視窗連線：

```bash
dbc connect 3402         # 自動帶入帳號，直接進 mysql
```

`dbc connect` 會用 `gcloud config get-value account` 取得目前登入的帳號，並自動取 `@` 前面那段當作 MySQL 使用者名稱（例如 `user@example.invalid` → `gffteam88`），不需要自己輸入。

## 指令

```
dbc setup              # 檢查依賴、引導 gcloud 登入
dbc doctor             # 自助排查
dbc start              # 掛從庫（預設，前景執行，Ctrl-C 結束）+ 沒有從庫的主庫
dbc start --primary    # 掛所有主庫
dbc start --all        # 掛所有實例
dbc start --port 3402  # 只掛單一實例
dbc start --address 0.0.0.0  # 監聽所有網路介面，供容器連線
dbc connect 3402       # 自動帶入帳號，進 mysql
dbc list               # 顯示 port 對照表
dbc upgrade            # 更新到最新版本（取代目前的執行檔）
```

管理者專用指令（`dbc admin ...`）見文件最後的[管理指令](#管理指令dbc-admin)章節。

`dbc upgrade` 會從 GitLab Pages 下載最新版本，直接取代目前正在執行的這個執行檔（跟 `instances.yaml`），裝過一次之後不用再重跑 `install.sh`/`install.ps1`。`dbc doctor`／`dbc start` 執行時也會順便檢查一次有沒有新版本，有的話會印一行提醒。

## `dbc start` 的行為

`dbc start` 會前景執行、佔住終端機視窗；按 `Ctrl-C` 結束代理。

代理預設只監聽 `0.0.0.0`。若要讓 Docker 等容器連入，可使用 `dbc start --address 0.0.0.0`（可與 `--primary`、`--all` 或 `--port` 併用）。這會將代理暴露給所有可到達主機的網路介面，請確認防火牆與網路存取範圍已正確限制。

**掛到「有從庫可用、卻連去主庫」的情況**時（例如用 `dbc start --primary` 或 `--all` 連到 xsgcasino 主庫）會印出提醒：

```
⚠️  主庫連線。日常查詢請用從庫（dbc start）。
    需要寫入權限請先加入 user@example.invalid
```

**例外：chat 系列資料庫目前只有主庫、沒有從庫**（`chatroom8`、`chatrrom-online` x3），所以 `dbc start` 預設模式也會把它們一起掛上去，不會印上面的警告——沒有替代的從庫可用，警告沒有意義。這種情況下 `dbc start` 會改印一行提示，說明是哪些庫、以及仍然是唯讀連線：

```
ℹ️  chatroom8、chatrrom-online 目前沒有從庫，以下連線是唯一的存取方式（非提權操作，仍是唯讀）。
```

`dbc list` 的「備註」欄也會標出這些實例。

**另一種例外：主庫有從庫可用，但仍想讓 `dbc start` 預設一起掛上**（`instances.yaml` 該筆實例設 `always_start: true`）。跟上面的 chat db 例外不同，這種情況**仍然會印「主庫連線」的警告**——從庫是真的存在、日常查詢還是該走從庫，這個旗標只是讓該主庫預設也保持連線，不代表不需要留意寫入風險。

### 查詢稽核

`dbc start` 會攔截通過它的每一條查詢，記錄到 Cloud Logging（該實例所在 GCP 專案，log name `dbc-query-audit`）：

- 查詢成功記 `INFO`，失敗（含被 dbc 拒絕的操作）記 `WARNING`，內容包含呼叫者的 IAM 帳號（`account`）、連線的 `conn_name`（`connName`）、完整查詢文字（`query`）。
- 另外附一個組好的 `message` 欄位（`帳號 @ conn_name [OK 或 FAILED: 錯誤訊息] 查詢片段`），查詢片段只截取前 120 字元，超過就加 `...`——這是給人看的一行式摘要，完整內容還是在 `query` 欄位。預設 Cloud Logging 的 Logs Explorer 不會自動用這個欄位當摘要顯示，要自己去 **Preferences → Manage summary fields** 加上 `jsonPayload.message`，之後看列表就會是這行摘要，不用整包 payload 攤開看。
- 這是為了補上 Cloud SQL 平台本身的限制：**Cloud SQL 只稽核主庫，複本完全不會產生稽核紀錄**，而 `dbc start` 預設就是把日常查詢導去複本，如果沒有這層，複本上的查詢會完全沒有稽核紀錄。

**啟動前會先檢查稽核紀錄寫入權限，沒有就直接拒絕啟動**：`dbc start` 啟動時會對每個涉及的 GCP 專案各做一次真實的寫入測試（`roles/logging.logWriter`），沒有這個權限的專案，該專案底下的實例會被拒絕啟動（其他有權限的專案不受影響），並印出清楚的錯誤訊息告訴你要找誰授權。啟動之後如果 Cloud Logging 中途出問題（網路瞬斷之類），會退回 best-effort（連線不受影響，只是那段時間的查詢可能沒記到，且只會提醒一次，不會洗畫面）。

**支援 prepared statement**（`COM_STMT_PREPARE`/`COM_STMT_EXECUTE`，多數資料庫 driver 參數化查詢的預設方式，包含 Laravel 等框架的預設行為）——dbc proxy 會把實際綁定的參數值代回 SQL 樣板，完整記進稽核紀錄，跟一般查詢一樣，不是只記樣板、也不是留一個稽核缺口。少數目前無法安全解出正確值的情況會擋掉那**一次**執行（回一個清楚的 MySQL 錯誤，連線本身不受影響，不是整個功能關掉）：

- 這個連線談成了 `CLIENT_QUERY_ATTRIBUTES`（MySQL 8.0.23+ 的協定能力，一般 driver 預設不會用到）
- 綁定了 dbc 尚未支援解碼的參數型別（整數、浮點、字串/BLOB、`DECIMAL`、日期時間都已支援，其餘少見型別如 `BIT`/`ENUM`/`SET`/`GEOMETRY` 目前不支援）
- 用 `COM_STMT_SEND_LONG_DATA` 傳大型參數（通常是大型 BLOB/TEXT）——這種情況查詢本身還是會被記錄，只是那個參數的內容會標成「未記錄」而不是靜默略過，因為記整包大型二進位資料意義不大，也可能超過 Cloud Logging 單筆紀錄的大小上限

**這不影響在 `mysql` CLI 互動輸入 `PREPARE`/`EXECUTE` 這種 SQL 語句**——那是文字協定（`COM_QUERY`），跟這裡處理的二進位協定指令是不同層次的東西。

儘管如此，dbc 仍然是給人用個人身分做 ad hoc 查詢的工具，不建議拿來當應用程式的正式連線方式——個人 IAM 身分、需要保持終端機視窗開著、沒有高可用性設計，都跟服務需要的穩定連線不同。應用程式（包含正式環境）需要連資料庫，應該用專屬的服務帳號直接連線（見 [`dbc admin service-accounts`](#dbc-admin-service-accounts)）。

### 裸機 MySQL（非 Cloud SQL）

`dbc start` 也能連到不在 GCP 底下的 MySQL（自架/裸機），在 `instances.yaml` 該筆實例加一個 `host` 欄位（`host:port` 格式）即可：

```yaml
- port: 3701
  role: primary
  instance: some-baremetal-db
  host: "0.0.0.0:3306"
```

`project`/`region` 還是要照填（不是拿來連線用，是決定查詢稽核要寫進哪個 GCP 專案的 Cloud Logging，跟一般 Cloud SQL 實例邏輯一致）。跟一般 Cloud SQL 實例的差異：

- **連線方式不同**：裸機直接純 TCP 轉發，不經過 `cloudsqlconn`/IAM 驗證那一層——因為裸機沒有 Cloud SQL IAM 資料庫驗證這種機制，實際登入還是要用該台 MySQL 真正的帳號密碼。
- **查詢稽核照樣運作**：`dbc start` 攔截查詢、記錄到 Cloud Logging 那套邏輯是純協定層的，不管背後是 Cloud SQL 還是裸機都一樣能用，`connName` 欄位會顯示實際的 `host:port`。
- **`dbc connect` 不會自動帶帳號**：Cloud SQL 那邊能自動帶入 IAM 對應的使用者名稱，是因為 Cloud SQL 有對應機制；裸機沒有，連線時 `mysql` 會照一般方式提示輸入帳號密碼。
- **`dbc admin grants`/`audit-rules`/`service-accounts` 不支援裸機實例**，會自動跳過並印一行說明——這幾個指令全部建立在 Cloud SQL Admin API／Cloud SQL 專屬的系統程序上，裸機 MySQL 沒有對應機制。

### 用 GUI 工具連線（MySQL Workbench、DBeaver、TablePlus…）

先跑 `dbc start` 掛好代理（保持該終端機視窗開著），GUI 工具當一般 MySQL 連線設定即可，不需要另外安裝任何東西：

| 欄位 | 填法 |
|---|---|
| Host | `0.0.0.0` |
| Port | `dbc list` 裡對應的 port |
| Username | Cloud SQL 實例：IAM 帳號 `@` 前面那段（例如 `user@example.invalid` → `gffteam88`）。裸機實例：該台伺服器真正的帳號，見上方「裸機 MySQL」 |
| Password | Cloud SQL 實例：留空——代理已經用 IAM 身分驗證過，MySQL 端不會再檢查密碼。裸機實例：該台伺服器真正的密碼 |
| SSL/TLS | 不用開啟——這段是代理到本機的明文連線，Cloud SQL 那一段的加密由 `dbc start` 自己處理，GUI 工具端不用管 |

GUI 工具預設用 server-side prepared statement 沒關係，dbc proxy 支援（見上方「查詢稽核」段落）；只有極少數情況（罕見參數型別、大型 BLOB 參數等）會被擋掉單次執行並回一個清楚的錯誤，不會整個連線失敗。

閒置兩分鐘會斷線（見下方「已知限制」），GUI 工具重新整理時看到重連是正常現象。

## Port 對照表

`34xx` = 從庫（replica），日常查詢用；`35xx` = 主庫（primary）。完整清單見 [`instances.yaml`](./instances.yaml)（同一個 instance 名稱可能出現在多個專案，查找請用 `dbc list`，不要只看名稱）。chat db 沒有從庫，只能透過 `35xx` 連線，見上方說明。

## 已知限制

- **閒置逾時：** 實例設定 `wait_timeout=120` / `interactive_timeout=120`，閒置兩分鐘會自動斷線。用 GUI 工具（如 DBeaver、TablePlus）連線經過 `dbc start` 的代理時，看到頻繁重連是正常現象。
- **只有 SELECT：** 個人帳號透過 `db-prod-user` 群組取得的是各業務資料庫的 `SELECT` 權限。需要寫入權限請走提權流程（加入 `user@example.invalid`）。
- **`dbc start` 不支援背景執行：** 需要保持一個終端機視窗開著。這是刻意的設計取捨，避免跨平台 daemon 管理的複雜度。

## 自助排查

先跑 `dbc doctor`。

`dbc doctor` 不會檢查你是不是在 `db-prod-user` 群組——一般帳號本來就沒有查詢 Cloud Identity 群組成員資格的權限，這項檢查對絕大多數人永遠只會回答「無法確認」，沒有實質意義，所以拿掉了。連線失敗、且已經排除下面其他常見原因的話，請聯繫管理者確認你是否已加入 `user@example.invalid`。

常見問題：

| 現象 | 可能原因 |
|---|---|
| `dbc start` 顯示 port 已被佔用 | 另一個終端機視窗已經在跑 `dbc start`，或有其他程式佔用該 port |
| `dbc connect` 找不到 mysql | 尚未安裝 MySQL client |
| 連線時系統一直斷線重連 | 正常現象，見上方「閒置逾時」 |
| 連線被拒絕 / 權限不足 | 確認帳號在 `db-prod-user` 群組，且該資料庫有授權 |
| `dbc start` 啟動時直接被拒絕，說缺少稽核紀錄寫入權限 | 該專案缺少 `roles/logging.logWriter`，見上方「查詢稽核」 |

## 管理指令（`dbc admin`）

以下指令只給有管理需求的人用（例如 `db-prod-admin` 成員），一般同仁不需要看這一節。全部都用呼叫者自己的 IAM 身分查詢/修正，不會內嵌任何額外憑證——沒有足夠權限就查不到/改不了，會顯示「⚠️ 無法確認」而不是整個指令失敗，不加 `--apply` 一律只模擬、不會真的改動。

```
dbc admin grants                                   # 列出每台實例的狀態摘要，並檢查是否符合預期
dbc admin grants --verbose                         # 額外印出每個群組完整的 GRANT 內容
dbc admin grants --apply                           # 確認後真的套用修正
dbc admin audit-rules                              # 檢查每台實例的 Cloud SQL 稽核規則
dbc admin audit-rules --verbose                    # 額外印出每台實例目前實際的規則清單
dbc admin audit-rules --apply                      # 確認後真的清空並重建成標準規則
dbc admin service-accounts                         # 依 service-accounts.yaml 檢查業務服務帳號
dbc admin service-accounts --apply                 # 確認後建立/同步帳號、密碼、cloudsqlsuperuser
dbc admin service-accounts --discard-old-password  # 停用每個帳號的舊密碼（密碼輪替後手動執行）
dbc admin service-accounts --port 3402             # 只針對單一實例執行（測試用）
```

### `dbc admin grants`

檢查/修正兩個 IAM 群組的權限是否符合預期：`db-prod-user` 對這台實例上每個業務資料庫（排除 `mysql`/`sys`/`information_schema`/`performance_schema`）都該有 `SELECT`，且不該有 `cloudsqlsuperuser`；`db-prod-admin` 該有 `cloudsqlsuperuser`。

- 預設輸出只有「需要注意」的異常清單跟每台實例一行的狀態摘要；要看每個群組完整的 GRANT 內容才加 `--verbose`。
- 不加 `--apply` 只會印出「會做什麼」，不會真的改動；加了 `--apply` 會先印出同樣的清單，確認後才真的執行。
- **只有主庫會被 `--apply` 修正，從庫只掃描、不動手**：從庫是 `read_only=ON`，直接下 `GRANT` 會失敗，而且沒必要——主庫的 GRANT 會透過複寫自動同步到從庫（`db-prod-user` 原本的 `SELECT` 授權就是這樣來的）。從庫如果跟預期有落差，一樣會被抓出來、顯示在「摘要」裡（標成 ℹ️ 僅供參考，不會出現在「需要注意」清單，也不算進 `--apply` 的套用項目數），通常代表對應的主庫還沒修正，或複寫還沒跟上；真正要處理的是主庫。

**特定實例例外：讓 `db-prod-user` 在某台實例上額外拿到 `cloudsqlsuperuser`**——這是整個群組（所有成員）都會拿到，不是只有你自己，套用前要想清楚。在 `instances.yaml` 該筆實例底下加：

```yaml
- port: 3502
  role: primary
  instance: mysql84-new
  # 為什麼要開這個例外、誰核准的，寫在這裡（YAML 註解），
  # 不要只留一個沒有說明的 true，之後的人會看不懂為什麼。
  db_prod_user_cloudsqlsuperuser: true
```

只影響 `cloudsqlsuperuser` 這一項判斷，`db-prod-user` 原本的 `SELECT` 補齊邏輯不受影響（是加法，不是取代）。`--verbose` 會把這個例外標成「← 此實例的例外設定，非異常」，跟一般的權限落差分開，避免被誤以為是工具漏報或帳號被入侵。

**⚠️ 主庫跟從庫要各自設定，不能只設主庫那筆**：`instances.yaml` 裡主庫跟從庫是兩筆完全獨立的設定（例如 `mysql84-new` 跟 `mysql84-new-replica`），彼此沒有程式碼層級的關聯。GRANT 雖然會透過複寫自動從主庫同步到從庫，但這個旗標本身不會——如果只在主庫那筆加，從庫那筆沒加：
- 從庫因複寫拿到 `cloudsqlsuperuser` 是正常的，但沒設旗標的那筆設定會把它當成異常顯示出來，容易誤判。
- **更嚴重的是**：如果這台從庫之後被升級成主庫（改了 `role`），沒有旗標的話 `--apply` 會真的去撤銷這個角色——不是顯示問題，是會真的執行錯誤的撤銷。

兩筆設定務必都加上同樣的旗標。

### `dbc admin audit-rules`

檢查/修正每台實例的 Cloud SQL 稽核規則，baseline 有兩條：

1. 全員的 DML/DDL/DCL 都要記錄（`op_result=B`）。
2. 排除 `mysql.heartbeat` 這張表的 DML（`op_result=E`）——這是 Cloud SQL 自己的 HA/複寫健康檢查每幾秒寫一次的心跳，不是業務寫入，實測確認過會用 `root` 帳號高頻寫入。這條排除**只精準指定 `mysql.heartbeat` 這一張表，不是整個 `mysql` schema**——`mysql.user`/`mysql.db` 這些真正敏感、放帳號權限的表還是完整記錄，不會被連帶排除掉。

其他重點：

- **不含 DQL**——Cloud SQL 只會稽核主庫，複本永遠不會產生稽核紀錄（官方文件與實測都確認），DQL 規則放在複本上沒有意義，日常查詢（走複本）的稽核改由 `dbc start` 自己的 proxy 負責（見[查詢稽核](#查詢稽核)）。
- 跟 `grants` 一樣，只有主庫會被 `--apply` 重置，從庫只掃描顯示。
- **現況跟 baseline 只要有任何差異就會整個清空重建**（不是逐條修補），因為稽核規則的比對本來就該是全有或全無，而不是猜哪裡不一樣。

### `dbc admin service-accounts`

給業務系統用的服務帳號（BUILT_IN、帳密登入，不是 IAM）管理工具，設定檔是 `service-accounts.yaml`（**內含密碼明文，已加進 `.gitignore`，絕對不要 commit 進 git**），放在跟 `dbc` 執行檔同目錄。範本見 [`service-accounts.yaml.example`](./service-accounts.yaml.example)（這份是佔位密碼，可以放心 commit），複製一份改名成 `service-accounts.yaml` 再填入實際密碼即可。格式：

```yaml
accounts:
  - name: gff_api_frontend
    password: [REDACTED]"
  - name: gff_old_worker
    state: absent   # 標記要刪除的帳號；不是「檔案裡沒寫」就代表刪除
```

權限統一是 `cloudsqlsuperuser`，不用另外設定。每個帳號在每台實例上：帳號不存在就建立；`cloudsqlsuperuser` 沒掛就補上；**密碼每次執行都會強制覆蓋**（Cloud SQL 沒辦法讀出目前密碼，沒辦法比對現況，只能每次都重新套用）。

**密碼輪替用 MySQL 的雙密碼機制**（`ALTER USER ... RETAIN CURRENT PASSWORD`，已實測確認 Cloud SQL 支援）：換成新密碼的同時，舊密碼還是繼續有效，讓服務逐步切換不會斷線。等確認所有服務都切換完新密碼後，另外執行：

```bash
dbc admin service-accounts --discard-old-password
```

停用舊密碼——這是獨立的手動步驟，dbc 沒辦法知道什麼時候可以安全丟棄舊密碼，只有你自己知道。

**刪除帳號是破壞性操作**，`--apply` 時會先把即將刪除的帳號單獨列在「⚠️ 即將刪除的帳號」區塊，並且要另外多按一次確認才會真的執行，不會跟一般的建立/密碼同步動作混在一起。跟 `grants`/`audit-rules` 一樣，只有主庫會被 `--apply` 執行，從庫只掃描顯示（帳號/密碼/角色都會透過複寫同步過去）。

`--port 3402` 可以只鎖定單一實例執行，測試密碼輪替/刪除時不用動到整個機群。

## 開發

```bash
go build ./...
go vet ./...
go test ./...
```

發版由 `.gitlab-ci.yml` 觸發 `goreleaser`（打上 `vX.Y.Z` tag 即會自動建置各平台 binary 並發佈）。

