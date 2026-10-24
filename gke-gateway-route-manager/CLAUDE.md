# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

---

此文檔為 Claude Code 在此程式碼庫中工作時提供指引。所有文檔和程式碼使用**正體中文**。

## 專案概述

**GKE Gateway Route Manager** 是一個 Laravel CLI 工具，用於統一管理多集群 GKE Gateway、Cloudflare DNS 和 cert-manager 憑證。

**⚠️ 專案狀態：** 規劃/設計階段，文檔完整但 **Laravel 應用程式尚未建立**（無 composer.json, artisan 等）。

**核心部署理念**：「先建設後開放」
```
Gateway → Certificate → HTTPRoute → 檢查狀態 → DNS
```

所有內部基礎設施（Gateway、Certificate、HTTPRoute）必須先部署並驗證完成，才能配置 DNS 開放外部流量。

## 開發工作流程

**重要原則：**
- 在進行具體開發之前，**必須先分析需求**，而不是直接執行修改
- 使用 **context7 MCP** 查詢 Laravel、Kubernetes、PHP 最新文檔和最佳實踐
- 實作前參考 `docs/` 目錄中的完整架構文檔


## 開發環境

本專案使用 VS Code devcontainer 確保一致的開發環境。

**Laravel 專案初始化流程：**
1. 首次啟動 devcontainer
2. 在容器內執行：`composer create-project laravel/laravel .`
3. 修改 `.devcontainer/devcontainer.json`，取消註解 `postCreateCommand` 行
4. 重建容器（Dev Containers: Rebuild Container）
5. 之後每次容器啟動都會自動執行 `composer install`

**容器掛載的前置需求：**
- `~/.config/gcloud` - GCP 憑證
- `~/.kube` - kubectl 配置
- `~/.ssh` - SSH 密鑰（用於 Git SSH 認證和遠程服務器訪問）
- `~/.config/claude` - Claude Code 配置（支援 MCP 服務器）
- 環境變數：`CLOUDFLARE_API_TOKEN`

**環境變數設定方式：**

`CLOUDFLARE_API_TOKEN` 可透過以下方式設定（優先級由高到低）：

1. **主機環境變數**（推薦）
   ```bash
   # 在主機的 ~/.bashrc 或 ~/.zshrc 添加
   export CLOUDFLARE_API_TOKEN=[REDACTED]"

   # 重新載入配置
   source ~/.bashrc  # 或 source ~/.zshrc
   ```
   優點：透過 devcontainer.json 自動掛載到容器，所有場景都能使用

2. **容器內 .env 文件**（Laravel 專案）
   ```bash
   # 在容器內執行
   cp .env.example .env
   # 編輯 .env: CLOUDFLARE_API_TOKEN=[REDACTED]
   ```
   優點：符合 Laravel 慣例，僅在 Laravel 應用內有效

**驗證環境變數設定：**
```bash
# 在容器內執行
echo $CLOUDFLARE_API_TOKEN

# 或使用 PHP 檢查
php -r 'echo getenv("CLOUDFLARE_API_TOKEN") ?: "未設定";'
```

**可用的系統工具：**
- kubectl, gcloud CLI, Helm 3
- PHP 8.4, Composer
- DNS 工具：dig, dnsutils
- Node.js 20.x（支援 MCP 服務器）

## Claude Code 支援

本專案的 devcontainer 已配置完整支援 Claude Code，可在容器內直接執行 `claude` 命令：

**已安裝工具：**
- ✅ Claude Code CLI（`@anthropic-ai/claude-code`）
- ✅ Node.js 20.x（MCP 服務器運行環境）
- ✅ 自動掛載 `~/.config/claude` 目錄（共享主機 MCP 配置）
- ✅ context7 MCP 服務器（容器啟動時自動配置）

**使用方式：**
```bash
# 在容器內直接使用 Claude Code
claude "幫我分析這個專案"

# 使用 context7 查詢最新文檔
claude "查詢 Laravel 11 最新的路由寫法"
```

**驗證安裝：**
```bash
# 檢查 Claude CLI 版本
claude --version

# 檢查 Node.js 版本
node --version  # 應顯示 v20.x

# 檢查 MCP 服務器狀態
claude mcp list

# 查看 MCP 配置文件（主機）
cat ~/.config/claude/claude_desktop_config.json

# 在容器內查看
cat /root/.config/claude/claude_desktop_config.json
```

**自動配置說明：**
- context7 MCP 服務器在容器啟動時自動配置（透過 `postStartCommand`）
- 配置儲存於主機的 `~/.config/claude/` 目錄
- 容器內掛載到 `/root/.config/claude/`
- 如需重新配置，執行：`claude mcp add --transport stdio context7 -- npx -y @upstash/context7-mcp`

**故障排除：**
- **claude 命令找不到**：重建容器（`Dev Containers: Rebuild Container`）
- **context7 未自動配置**：手動執行 `claude mcp add --transport stdio context7 -- npx -y @upstash/context7-mcp`
- **MCP 服務器連接失敗**：檢查網絡連接，確保可以訪問 npm registry
- **權限問題**：確保容器用戶為 root（devcontainer.json 中已配置）
- **掛載問題**：檢查 ~/.config/claude 目錄是否存在且可讀

## 常用命令

### Laravel 專案初始化（尚未執行）
```bash
# 建立 Laravel 專案
composer create-project laravel/laravel .
# 或使用 Laravel CLI
laravel new .

# 配置環境變數
cp .env.example .env
# 編輯 .env 填入 CLOUDFLARE_API_TOKEN
```

### 核心部署工作流程
```bash
# 一鍵部署（日常使用推薦）
php artisan deploy:k8s <cluster> <env>
php artisan status:all <cluster> <env>
php artisan dns:apply <cluster> <env>

# 分步驟部署（更多控制）
php artisan deploy:gateway <cluster> <env>
php artisan generate:config <cluster> <env>
php artisan deploy:certificate <cluster> <env>
php artisan deploy:httproute <cluster> <env>
php artisan status:all <cluster> <env>
php artisan dns:apply <cluster> <env>
php artisan dns:verify <cluster> <env>
```

### 狀態查詢與驗證
```bash
php artisan status:all <cluster> <env>
php artisan status:gateway <cluster> <env>
php artisan status:certificate <cluster> <env>
php artisan status:httproute <cluster> <env>
php artisan dns:verify <cluster> <env>
```

### 配置管理
```bash
php artisan validate:config
php artisan generate:config <cluster> <env>
php artisan analyze:domains <cluster> <env>
php artisan cluster:list
```

**常用參數：**
- `<cluster>`: cluster-a, cluster-b, cluster-c（定義於 config/clusters.yaml）
- `<env>`: staging, production, dev（對應 Kubernetes namespace）

## 架構原則

### 1. 部署順序（重要）

**推薦順序：**
```
1. Gateway（等待 IP 分配：3-5 分鐘）
2. Certificate（DNS-01 驗證：1-5 分鐘）
3. HTTPRoute（內部路由配置）
4. 檢查狀態（必須驗證所有資源就緒）
5. DNS（開放外部流量）
```

**為什麼是這個順序？**
- Gateway 提供 IP 地址作為基礎
- Certificate 使用 DNS-01（不需要 DNS A 記錄）
- HTTPRoute 在流量到達前配置路由
- 狀態檢查防止過早配置 DNS
- DNS 僅在一切就緒後才開放流量

**絕對不要這樣做：**
- ❌ 在 HTTPRoute 之前配置 DNS（導致 404 錯誤）
- ❌ 在 Certificate 就緒前配置 DNS（導致憑證錯誤）
- ❌ 在 DNS 配置前跳過狀態驗證

### 2. DNS-01 ACME 挑戰

**關鍵優勢：** 憑證驗證不需要 DNS A 記錄或 Gateway 可訪問性。

**運作方式：**
1. cert-manager 創建 DNS TXT 記錄：`_acmeexample.invalid`
2. Let's Encrypt 查詢 TXT 記錄（不是 A 記錄）
3. 驗證在 1-5 分鐘內完成
4. 憑證儲存為 Kubernetes Secret
5. TXT 記錄自動清理

**優勢：**
- 在配置 DNS 前就可驗證憑證
- 支持萬用字元憑證（`*.example.invalid`）
- 驗證速度更快（1-5 分鐘 vs HTTP-01 的 15-60 分鐘）
- 支持內部服務

### 3. 安全閘門機制

`dns:apply` 命令在修改 DNS 前會自動檢查前置條件：
- ✅ Gateway 有 IP 且為 READY
- ✅ Certificate 為 READY: True
- ✅ HTTPRoute 已配置

如果任何檢查失敗，命令會終止以防止誤配置。

### 4. 多集群架構

**地理分佈：**
- cluster-a: 台灣 (asia-east1)
- cluster-b: 日本 (asia-northeast1)
- cluster-c: 美國 (us-central1)

使用 Cloudflare Load Balancer 進行 GeoDNS 路由。

## 專案結構

```
gke-gateway-route-manager/
├── app/Console/Commands/        # Laravel Artisan 命令（待實作）
│   ├── Deploy/                  # deploy:k8s, deploy:gateway 等
│   ├── Dns/                     # dns:apply, dns:verify
│   └── Status/                  # status:all, status:gateway 等
├── clusters/                    # 集群配置（待建立）
│   └── {cluster}/
│       └── {env}/
│           ├── gateways/        # Gateway YAML 文件
│           ├── certificates/    # Certificate YAML 文件
│           ├── httproutes/      # HTTPRoute YAML 文件
│           └── dns-config.yaml  # DNS 配置
├── config/
│   ├── clusters.yaml            # 集群定義（待建立）
│   └── cert-manager/            # cert-manager 配置
│       └── cluster-issuer.yaml  # Let's Encrypt ClusterIssuer
├── docs/                        # 完整文檔（正體中文）
│   ├── getting-started.md       # 設置指南
│   ├── architecture.md          # 設計理念和部署順序
│   ├── commands-reference.md    # 所有 CLI 命令
│   ├── deployment-guide.md      # 部署工作流程
│   ├── troubleshooting.md       # 常見問題
│   ├── faq.md                   # 常見問答
│   └── devcontainer-features.md # DevContainer Features 配置說明
└── .devcontainer/               # VS Code 開發容器
    ├── Dockerfile               # PHP 8.4 + kubectl + gcloud + Claude CLI
    └── devcontainer.json        # 容器配置
```

## 配置文件格式

### clusters.yaml (config/clusters.yaml)
```yaml
clusters:
  - name: cluster-a
    project: my-gcp-project
    region: asia-east1
    environments: [staging, production]
  - name: cluster-b
    project: my-gcp-project
    region: asia-northeast1
    environments: [staging, production]
```

### Gateway YAML (clusters/{cluster}/{env}/gateways/*.yaml)
標準 Kubernetes Gateway API v1 資源：
- `spec.listeners[].port`: 80, 443
- `spec.listeners[].protocol`: HTTP, HTTPS
- `spec.listeners[].tls.certificateRefs`: 指向 cert-manager 生成的 Secret

### Certificate YAML (clusters/{cluster}/{env}/certificates/*.yaml)
cert-manager v1 Certificate 資源：
- `spec.dnsNames`: 域名列表（支援萬用字元 `*.example.invalid`）
- `spec.issuerRef`: 指向 ClusterIssuer（Let's Encrypt）
- `spec.secretName`: 儲存憑證的 Secret 名稱
- **由 `generate:config` 自動生成**

### HTTPRoute YAML (clusters/{cluster}/{env}/httproutes/*.yaml)
Kubernetes Gateway API v1 HTTPRoute 資源：
- `spec.hostnames`: 此路由處理的域名
- `spec.parentRefs`: 綁定的 Gateway
- `spec.rules[].backendRefs`: 後端 Service 路由規則

### dns-config.yaml (clusters/{cluster}/{env}/dns-config.yaml)
```yaml
dns:
  - hostname: example.invalid
    gateway_ip: 0.0.0.0
  - hostname: example.invalid
    gateway_ip: 0.0.0.0
```
**由 `generate:config` 自動生成**

## 安全性考量

### 密鑰管理
- **Cloudflare API Token**：儲存為 Kubernetes Secret 於 cert-manager namespace，僅授予最小權限（Zone.DNS Edit + Zone.Zone Read）
- **GCP Service Account**：盡可能使用 Workload Identity
- **環境變數**：永遠不要提交 `.env` 文件（已加入 gitignore）

### 網絡安全
- TLS 終止於 Gateway 層
- 強制 HTTPS 重定向
- 考慮使用 NetworkPolicy 限制 Pod 間通信

### 訪問控制
- Kubernetes RBAC 採用最小權限原則
- 保護 main 分支，強制 Code Review
- 手動部署執行（CI 不自動部署）

## GitOps 工作流程

所有配置變更遵循以下流程：
1. 建立功能分支
2. 修改 YAML 配置
3. 執行 `php artisan validate:config`
4. 建立 GitLab Merge Request
5. Code Review 和批准
6. 合併至 main
7. 透過 devcontainer 手動部署

**GitLab CI 僅執行驗證，不執行部署。**

## Laravel 應用程式架構

### 命令組織結構
```
app/Console/Commands/
├── Deploy/
│   ├── DeployK8sCommand.php          # deploy:k8s（一鍵部署）
│   ├── DeployGatewayCommand.php      # deploy:gateway
│   ├── DeployCertificateCommand.php  # deploy:certificate
│   └── DeployHttpRouteCommand.php    # deploy:httproute
├── Dns/
│   ├── DnsApplyCommand.php           # dns:apply（含安全檢查）
│   └── DnsVerifyCommand.php          # dns:verify
├── Status/
│   ├── StatusAllCommand.php          # status:all
│   ├── StatusGatewayCommand.php      # status:gateway
│   ├── StatusCertificateCommand.php  # status:certificate
│   └── StatusHttpRouteCommand.php    # status:httproute
└── Config/
    ├── ValidateConfigCommand.php     # validate:config
    ├── GenerateConfigCommand.php     # generate:config
    ├── AnalyzeDomainsCommand.php     # analyze:domains
    └── ClusterListCommand.php        # cluster:list
```

### 核心服務層（建議）
```
app/Services/
├── KubernetesService.php    # kubectl 操作抽象層
├── CloudflareService.php    # Cloudflare API 客戶端
├── ConfigService.php        # YAML 配置讀寫
└── ValidationService.php    # 資源狀態驗證
```

## 關鍵實作要點

### 實作 Laravel 命令時

1. **始終先連接集群** 使用 config/clusters.yaml
2. **等待資源就緒** 設定逾時時間：
   - Gateway IP 分配：最多 5 分鐘
   - Certificate READY：最多 5 分鐘
3. **在 dns:apply 命令中實作安全檢查**
4. **提供清晰的狀態輸出** 使用表情符號指示器（✅ ❌ ⏳）
5. **優雅處理錯誤** 提供可操作的錯誤訊息
6. **使用 Process facade** 執行 kubectl、gcloud 命令
7. **實作進度條** 用於長時間等待操作（如等待 Gateway IP）

### 智能配置生成

`generate:config` 命令必須：
1. 讀取所有 HTTPRoute YAML 文件
2. 從 `spec.hostnames` 提取唯一域名
3. 查詢 Gateway 獲取 IP 地址
4. 生成包含所有 dnsNames 的 Certificate YAML
5. 使用 Gateway IP 生成 dns-config.yaml

### DNS 管理

`dns:apply` 命令必須：
1. 檢查前置條件（Gateway/Certificate/HTTPRoute 狀態）
2. 智能查詢每個域名的 Cloudflare Zone ID
3. 檢查 DNS 記錄是否已存在
4. 創建或更新 DNS A 記錄
5. 套用後驗證配置

## 文檔語言

所有詳細文檔均為 **正體中文**，位於 `docs/` 目錄。本 CLAUDE.md 文件和程式碼註解也使用**正體中文**。

## 建議的 Composer 套件

實作時考慮使用以下套件：
- `symfony/yaml` - YAML 解析（Laravel 內建）
- `symfony/process` - 執行系統命令（Laravel 內建）
- `guzzlehttp/guzzle` - HTTP 客戶端，用於 Cloudflare API（Laravel 內建）
- `spatie/yaml-front-matter` - 進階 YAML 處理（選用）

## 目前實作狀態

**最後更新：** 2024年11月
**當前階段：** 規劃/設計階段 - 文檔完成，等待實作開始

**已完成：**
- ✅ 完整文檔 (docs/)
- ✅ 開發環境 (devcontainer + 所有工具)
- ✅ 架構設計和工作流程
- ✅ CLAUDE.md 指引文檔
- ✅ .devcontainer/Dockerfile（包含所有必要工具）
- ✅ .devcontainer/devcontainer.json（完整配置）

**進行中：**
- 🔄 無

**待實作：**
- ❌ Laravel 專案初始化（`composer create-project laravel/laravel .`）
- ❌ Laravel Artisan 命令 (app/Console/Commands/)
- ❌ 配置文件結構 (clusters/, config/clusters.yaml)
- ❌ Kubernetes 資源模板和範例
- ❌ GitLab CI 驗證流程 (.gitlab-ci.yml)

**追蹤方式：**
查看 GitHub/GitLab Issues 或項目看板追蹤實作進度

## 下一步行動

1. 初始化 Laravel 專案：`composer create-project laravel/laravel .`
2. 建立命令架構：`php artisan make:command Deploy/DeployK8sCommand`
3. 建立核心服務層
4. 實作配置讀取和驗證
5. 實作 kubectl 操作包裝
6. 實作 Cloudflare API 整合

實作時，請遵循 docs/architecture.md 中記錄的部署順序和安全原則。
