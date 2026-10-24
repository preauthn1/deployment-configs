# GKE Gateway Route Manager

統一管理多集群 GKE Gateway、Cloudflare DNS 和 cert-manager 憑證的 Laravel CLI 工具。

## 核心特性

- 🚀 **一鍵部署** - `deploy:k8s` 合併部署所有 Kubernetes 資源
- 🔒 **DNS-01 驗證** - cert-manager + Cloudflare，憑證驗證不需要 DNS A 記錄
- 🛡️ **安全閘門** - DNS 配置前自動檢查所有資源狀態
- 📋 **智能配置** - 從 HTTPRoute 自動生成 Certificate 和 DNS 配置
- 🌐 **多集群管理** - 統一管理跨區域、跨專案的 GKE 集群
- 🎯 **手動控制** - 所有操作手動執行，確保每步可控

## ⚠️ 專案狀態

本專案目前處於**規劃/設計階段**：
- ✅ 完整文檔已完成
- ✅ DevContainer 環境已配置
- ❌ Laravel 應用程式尚未建立

**開始使用前**，需先初始化 Laravel 專案：
```bash
composer create-project laravel/laravel .
```

## 快速開始

```bash
# 1. 啟動 devcontainer 並初始化 Laravel 專案
composer create-project laravel/laravel .
cp .env.example .env
# 編輯 .env 填入 CLOUDFLARE_API_TOKEN

# 2. 部署所有 Kubernetes 資源
php artisan deploy:k8s cluster-a staging

# 3. 檢查狀態（確認所有 ✅）
php artisan status:all cluster-a staging

# 4. 配置 DNS 開放流量
php artisan dns:apply cluster-a staging
```

📖 詳細設置請看 [初次設置指南](docs/getting-started.md)

## 部署順序

```
Gateway → Certificate → HTTPRoute → 檢查狀態 → DNS
  ↓          ↓            ↓            ↓         ↓
獲得 IP   DNS-01 驗證   配置路由    確認就緒   開放流量
```

**核心原則**：先建設內部資源，確認就緒後，才配置 DNS 開放外部流量。

📖 詳細說明請看 [架構設計](docs/architecture.md#部署順序)

## 常用命令

| 命令 | 說明 |
|------|------|
| `php artisan deploy:k8s <cluster> <env>` | 合併部署 Gateway + Certificate + HTTPRoute |
| `php artisan status:all <cluster> <env>` | 檢查所有資源狀態 |
| `php artisan dns:apply <cluster> <env>` | 配置 DNS A 記錄（自動檢查前置條件） |
| `php artisan generate:config <cluster> <env>` | 從 HTTPRoute 生成配置 |
| `php artisan deploy:gateway <cluster> <env>` | 單獨部署 Gateway |
| `php artisan deploy:certificate <cluster> <env>` | 單獨部署 Certificate |
| `php artisan deploy:httproute <cluster> <env>` | 單獨部署 HTTPRoute |
| `php artisan dns:verify <cluster> <env>` | 驗證 DNS 配置 |

📖 完整命令列表請看 [命令參考](docs/commands-reference.md)

## 文檔導航

### 新手入門
- **[初次設置](docs/getting-started.md)** - 環境設置、首次部署完整流程
- **[部署指南](docs/deployment-guide.md)** - 合併部署 vs 分步驟部署

### 日常使用
- **[命令參考](docs/commands-reference.md)** - 所有命令的詳細說明
- **[常見工作流程](docs/deployment-guide.md#workflows)** - 新增服務、更新配置、批量部署

### 深入理解
- **[架構設計](docs/architecture.md)** - 為什麼這樣設計、DNS-01 優勢
- **[最佳實踐](docs/architecture.md#最佳實踐)** - 變更管理、安全性

### 問題排查
- **[故障排除](docs/troubleshooting.md)** - 常見錯誤和解決方案
- **[FAQ](docs/faq.md)** - 常見問題解答

## 項目結構

```
gke-gateway-route-manager/
├── README.md                    # 本文檔
├── docs/                        # 詳細文檔
├── app/Console/Commands/        # Laravel Artisan 命令
│   ├── Deploy/                  # 部署命令
│   ├── Dns/                     # DNS 管理命令
│   └── Status/                  # 狀態查詢命令
├── clusters/                    # 集群配置
│   ├── cluster-a/
│   │   ├── production/
│   │   │   ├── gateways/
│   │   │   ├── certificates/
│   │   │   └── httproutes/
│   │   └── staging/
│   └── ...
└── config/
    ├── clusters.yaml            # 集群列表配置
    └── cert-manager/            # cert-manager 配置
```

## 前置需求

- Docker + VS Code（devcontainer）
- PHP 8.4 / Composer
- kubectl / gcloud CLI
- Cloudflare API Token（Zone.DNS Edit + Zone.Zone Read）
- GCP Service Account（GKE 訪問權限）

## 常見工作流程

### 新增服務

```bash
# 1. 創建 HTTPRoute 配置
vim clusters/cluster-a/staging/httproutes/new-service.yaml

# 2. 部署並配置 DNS
php artisan deploy:k8s cluster-a staging
php artisan status:all cluster-a staging
php artisan dns:apply cluster-a staging
```

### 更新現有配置

```bash
# 1. 修改配置文件
vim clusters/cluster-a/staging/gateways/main-gateway.yaml

# 2. 重新部署對應資源
php artisan deploy:gateway cluster-a staging
php artisan status:gateway cluster-a staging
```

### 多集群批量部署

```bash
for cluster in cluster-a cluster-b cluster-c; do
  php artisan deploy:k8s $cluster staging
  php artisan status:all $cluster staging
  php artisan dns:apply $cluster staging
done
```

## 關鍵概念

### DNS-01 ACME 挑戰

使用 cert-manager + Cloudflare DNS-01 簽發憑證：

- ✅ 憑證驗證**不需要** DNS A 記錄
- ✅ 憑證驗證**不需要** Gateway 可訪問
- ✅ 憑證可以在 DNS 配置前就驗證完成
- ✅ 支持萬用字元憑證（`*.example.invalid`）
- ⚡ 驗證速度快（1-5 分鐘 vs HTTP-01 的 15-60 分鐘）

📖 詳細原理請看 [架構設計 - DNS-01 說明](docs/architecture.md#dns-01-acme-挑戰)

### 安全檢查

`dns:apply` 命令會自動檢查前置條件：
- ✅ Gateway 是否有 IP 且 READY
- ✅ Certificate 是否 READY: True
- ✅ HTTPRoute 是否已配置

如果檢查失敗，命令會終止並顯示錯誤，防止誤配置。

## GitLab CI 集成

本專案使用 GitLab CI 進行**配置驗證**，實際部署透過**手動執行命令**。

```yaml
# .gitlab-ci.yml
validate:
  stage: validate
  script:
    - php artisan validate:config
```

實際部署在 devcontainer 中手動執行，確保每步可控。

📖 詳細配置請看 [部署指南 - CI/CD](docs/deployment-guide.md#cicd)

## 獲取幫助

- 📖 **查看文檔**：[docs/](docs/) 目錄
- 📋 **命令幫助**：`php artisan list` 或 `php artisan help <command>`
- 🐛 **問題反饋**：GitHub Issues
- 💬 **尋求協助**：聯繫 SRE 團隊

## 授權

MIT License
