# 初次設置指南

本指南將帶你完成從零開始到首次成功部署的完整流程。

## 目錄

- [前置需求](#前置需求)
- [初次設定](#初次設定)
- [第一次部署](#第一次部署)
- [驗證部署](#驗證部署)

## 前置需求

在開始之前，請確保已完成以下準備工作。

### 1. 安裝 cert-manager

cert-manager 是 Kubernetes 原生的憑證管理工具，負責自動申請、續期和管理 SSL/TLS 憑證。

```bash
# 安裝 cert-manager（使用 Helm）
helm repo add jetstack https://example.invalid
helm repo update

# 安裝 cert-manager 及 CRDs（使用最新穩定版）
helm install cert-manager jetstack/cert-manager \
  --namespace cert-manager \
  --create-namespace \
  --version v1.16.2 \
  --set installCRDs=true

# 驗證安裝
kubectl get pods -n cert-manager
# 應該看到 cert-manager, cert-manager-cainjector, cert-manager-webhook 三個 Pod 運行
```

### 2. 配置 Cloudflare API Token Secret

cert-manager 需要使用 Cloudflare API Token 來創建 DNS TXT 記錄以完成 DNS-01 ACME 挑戰。

#### 獲取 Cloudflare API Token

1. 登入 Cloudflare Dashboard
2. 前往 **My Profile** → **API Tokens**
3. 點擊 **Create Token**
4. 選擇 **Edit zone DNS** 模板
5. 配置權限：
   - **Permissions**: `Zone - DNS - Edit` + `Zone - Zone - Read`
   - **Zone Resources**: `Include - All zones`（或選擇特定 Zone）
6. 創建並複製 Token

#### 在 Kubernetes 中創建 Secret

**重要：使用 ClusterIssuer 時，Secret 需要創建在 cert-manager namespace**

```bash
# ClusterIssuer 使用的 Secret 必須在 cert-manager namespace 中
kubectl create secret generic cloudflare-api-token \
  --namespace=cert-manager \
  --from-literal=api-token=[REDACTED]

# 驗證 Secret 已創建
kubectl get secret cloudflare-api-token -n cert-manager
```

**重要說明：**
- ✅ **ClusterIssuer 方案**：Secret 只需創建在 `cert-manager` namespace 一次即可
- ✅ **集群級別共享**：所有 namespace 的 Certificate 都可以使用同一個 ClusterIssuer
- 🔐 **更簡單的管理**：不需要在每個應用 namespace 中重複創建 Secret

#### 替代方案：使用 Issuer（namespace 級別）

如果您需要更細粒度的控制（例如多租戶環境），可以使用 Issuer 而非 ClusterIssuer：

```bash
# Issuer 方案：在每個應用 namespace 創建 Secret
for NS in staging production; do
  kubectl create secret generic cloudflare-api-token \
    --namespace=$NS \
    --from-literal=api-token=[REDACTED]
done
```

**Issuer vs ClusterIssuer 對比：**

| 特性 | ClusterIssuer（推薦） | Issuer |
|------|---------------------|--------|
| **Secret 位置** | cert-manager namespace | 應用 namespace |
| **管理複雜度** | 低（一次配置） | 高（每個 namespace 配置） |
| **權限需求** | cluster-admin | namespace-admin |
| **適用場景** | 單一團隊管理 | 多租戶環境 |

**本指南使用 ClusterIssuer 方案**

### 3. 配置 ClusterIssuer

ClusterIssuer 定義了如何與 Let's Encrypt ACME 伺服器交互以申請憑證。

創建 `config/cert-manager/cluster-issuer.yaml`：

```yaml
apiVersion: example.invalid/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-dns01
spec:
  acme:
    # Let's Encrypt ACME 伺服器（生產環境）
    server: https://example.invalid/directory

    # 用於 ACME 賬戶註冊的郵箱
    email: user@example.invalid

    # 儲存 ACME 賬戶私鑰的 Secret 名稱
    privateKeySecretRef:
      name: letsencrypt-dns01-account-key

    # DNS-01 挑戰配置
    solvers:
    - dns01:
        cloudflare:
          # Cloudflare API Token Secret（在 cert-manager namespace 中創建）
          apiTokenSecretRef:
            name: cloudflare-api-token
            key: api-token

# 可選：測試環境 ClusterIssuer（用於測試，避免達到 Let's Encrypt 速率限制）
---
apiVersion: example.invalid/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-dns01-staging
spec:
  acme:
    # Let's Encrypt Staging 伺服器（測試用）
    server: https://example.invalid/directory
    email: user@example.invalid
    privateKeySecretRef:
      name: letsencrypt-dns01-staging-account-key
    solvers:
    - dns01:
        cloudflare:
          apiTokenSecretRef:
            name: cloudflare-api-token
            key: api-token
```

**部署 ClusterIssuer：**

```bash
kubectl apply -f config/cert-manager/cluster-issuer.yaml

# 驗證 ClusterIssuer 狀態
kubectl get clusterissuer
# NAME                         READY   AGE
# letsencrypt-dns01            True    10s
# letsencrypt-dns01-staging    True    10s
```

**重要說明：**
- ✅ **ClusterIssuer** 是集群級資源，可以被任何命名空間的 Certificate 使用
- ✅ **生產環境** 使用 `letsencrypt-dns01`
- ✅ **測試環境** 使用 `letsencrypt-dns01-staging`（避免速率限制）
- ⚠️ **Secret 位置** 使用 ClusterIssuer 時，`cloudflare-api-token` Secret 只需在 `cert-manager` namespace 創建一次

## 初次設定

### 1. Clone 專案

```bash
git clone <repository-url>
cd gke-gateway-route-manager
```

### 2. 啟動 devcontainer

```bash
# 使用 VS Code 開啟專案
# 點擊 "Reopen in Container" 或執行命令
# Dev Containers: Reopen in Container
```

### 3. 安裝 Laravel 依賴

```bash
composer install
```

### 4. 配置環境變數

```bash
# 複製環境配置
cp .env.example .env

# 編輯 .env 設定
vim .env
```

```.env
# Cloudflare API Token（只需要 Token，Zone ID 會自動查詢）
CLOUDFLARE_API_TOKEN=[REDACTED]

# GCP 配置
GCP_PROJECT_ID=your-project-id
GCP_SERVICE_ACCOUNT_KEY_PATH=/path/to/sa-key.json

# 預設集群和環境（可選）
DEFAULT_CLUSTER=cluster-a
DEFAULT_ENV=staging
```

### 5. 配置集群資訊

```bash
# 編輯集群配置
vim config/clusters.yaml
```

範例 `config/clusters.yaml`：

```yaml
clusters:
  cluster-a:
    project: gcp-project-id
    region: asia-east1
    name: cluster-a
    environments:
      - staging
      - production

  cluster-b:
    project: gcp-project-id
    region: us-central1
    name: cluster-b
    environments:
      - production
```

### 6. 驗證配置

```bash
php artisan validate:config
```

## 第一次部署

本節將帶你完成首次部署的完整流程。我們推薦使用**合併部署**方式，最簡單快速。

### Step 0: 準備 HTTPRoute 配置

首先創建 HTTPRoute 配置文件，定義需要的域名和路由規則：

```bash
# 創建 HTTPRoute 配置
vim clusters/cluster-a/staging/httproutes/api-route.yaml
```

範例配置：

```yaml
apiVersion: example.invalid/v1
kind: HTTPRoute
metadata:
  name: api-route
  namespace: staging
spec:
  parentRefs:
  - name: main-gateway
  hostnames:
  - "example.invalid"
  - "example.invalid"
  rules:
  - matches:
    - path:
        type: PathPrefix
        value: /api
    backendRefs:
    - name: api-service
      port: 8080
```

同時準備 Gateway 配置：

```bash
vim clusters/cluster-a/staging/gateways/main-gateway.yaml
```

範例配置：

```yaml
apiVersion: example.invalid/v1
kind: Gateway
metadata:
  name: main-gateway
  namespace: staging
spec:
  gatewayClassName: gke-l7-global-external-managed
  listeners:
  - name: http
    protocol: HTTP
    port: 80
  - name: https
    protocol: HTTPS
    port: 443
    tls:
      mode: Terminate
      certificateRefs:
      - kind: Secret
        group: ""
        name: tls-cert
```

同時準備 Certificate 配置（可選，deploy:k8s 會自動生成）：

```bash
vim clusters/cluster-a/staging/certificates/tls-cert.yaml
```

範例配置：

```yaml
apiVersion: example.invalid/v1
kind: Certificate
metadata:
  name: tls-cert
  namespace: staging
spec:
  # 憑證保存的 Secret 名稱（Gateway 會引用此 Secret）
  secretName: tls-cert

  # 憑證包含的域名列表
  dnsNames:
  - "example.invalid"
  - "example.invalid"

  # 引用 ClusterIssuer 來簽發憑證
  issuerRef:
    name: letsencrypt-dns01
    kind: ClusterIssuer
    group: example.invalid
```

**說明：**
- `secretName`: 必須與 Gateway 的 `certificateRefs[].name` 一致
- `dnsNames`: 與 HTTPRoute 的 `hostnames` 一致
- `issuerRef.group`: example.invalid（最佳實踐，明確指定）

### 方案 A：合併部署（推薦）

最簡單的部署方式，只需 4 個命令：

```bash
# 1. 一次性部署所有 Kubernetes 資源
php artisan deploy:k8s cluster-a staging

# 此命令會自動：
# 1. 部署 Gateway 並等待獲得 IP
# 2. 生成 Certificate 和 DNS 配置
# 3. 部署 Certificate（開始 DNS-01 驗證）
# 4. 部署 HTTPRoute（配置路由規則）
# 5. 等待憑證驗證完成（READY: True）
# 6. 顯示狀態摘要

# 2. 檢查所有資源狀態（重要！）
php artisan status:all cluster-a staging

# 輸出範例：
# ✅ Gateway: main-gateway (IP: 0.0.0.0) - READY
# ✅ Certificate: tls-cert - READY: True
# ✅ HTTPRoute: api-route - 2 hostnames configured
#
# 📋 DNS 配置預覽：
#   - example.invalid -> 0.0.0.0
#   - example.invalid -> 0.0.0.0

# 3. 確認所有資源都顯示 ✅ 後，配置 DNS（開放外部流量）
php artisan dns:apply cluster-a staging

# 此命令會自動檢查：
# - Gateway 是否有 IP 且 READY
# - Certificate 是否 READY: True
# - HTTPRoute 是否已配置
# 如果檢查失敗，命令會終止並顯示錯誤

# 4. 驗證 DNS 生效
php artisan dns:verify cluster-a staging
```

### 方案 B：分步驟部署（學習理解）

如果你想更細緻地了解每一步，可以使用分步驟部署：

```bash
# 1. 部署 Gateway 並獲取 IP
php artisan deploy:gateway cluster-a staging
# 輸出: Gateway IP: 0.0.0.0

# 2. 生成配置（Certificate 和 DNS）
php artisan generate:config cluster-a staging

# 3. 部署 Certificate（DNS-01 驗證，不需要 A 記錄）
php artisan deploy:certificate cluster-a staging

# 4. 等待憑證驗證完成（1-5 分鐘）
kubectl get certificate -n staging --watch
# 確認 READY: True

# 5. 部署 HTTPRoute（配置路由規則）
php artisan deploy:httproute cluster-a staging

# 6. 檢查所有資源狀態（重要！）
php artisan status:all cluster-a staging
# 確認：
# - Gateway 有 IP 且 READY
# - Certificate READY: True
# - HTTPRoute 已配置

# 7. 配置 DNS A 記錄（開放外部流量）
php artisan dns:apply cluster-a staging

# 8. 驗證 DNS 生效
php artisan dns:verify cluster-a staging
```

### 為什麼按照這個順序部署？

```
Gateway → Certificate → HTTPRoute → 檢查狀態 → DNS
  ↓          ↓            ↓            ↓         ↓
獲得 IP   DNS-01 驗證   配置路由    確認就緒   開放流量
```

**部署順序說明：**
- 🏗️ **Gateway 優先**：提供 IP 地址，所有配置基於此 IP
- 🔒 **憑證第二**：使用 DNS-01 驗證，不需要 A 記錄即可簽發
- 🛣️ **HTTPRoute 第三**：配置內部路由規則，準備處理流量
- 🔍 **檢查狀態第四**：確認所有資源已就緒
- 🌐 **DNS 最後**：所有配置就緒後，才開放外部流量

**核心優勢：**
- ✅ **避免中間狀態**：DNS 指向服務時，所有配置已就緒
- 🔒 **更安全**：HTTPS 在流量進來時已經可用
- 🚀 **零停機部署**：所有內部配置完成後才開放流量
- 🛡️ **強制檢查**：DNS 配置前必須檢查所有資源狀態

## 驗證部署

### 1. 檢查 Kubernetes 資源

```bash
# 檢查 Gateway
kubectl get gateway -n staging
kubectl describe gateway main-gateway -n staging

# 檢查 Certificate
kubectl get certificate -n staging
kubectl describe certificate tls-cert -n staging

# 檢查 HTTPRoute
kubectl get httproute -n staging
kubectl describe httproute api-route -n staging

# 查看憑證驗證流程（如果需要）
kubectl get certificaterequest -n staging
kubectl get order -n staging
kubectl get challenge -n staging
```

### 2. 驗證 DNS

```bash
# 使用工具驗證
php artisan dns:verify cluster-a staging

# 或手動驗證
dig example.invalid +short
dig example.invalid +short
# 應該都返回 Gateway IP: 0.0.0.0
```

### 3. 測試 HTTP/HTTPS 連接

```bash
# 測試 HTTP 連接
curl -I http://example.invalid
curl -I http://example.invalid

# 測試 HTTPS 連接（憑證已準備好，應該立即可用）
curl -I https://example.invalid
curl -I https://example.invalid

# 測試特定路徑
curl https://example.invalid/api/health
```

### 4. 查看整體狀態

```bash
# 使用整合命令查看所有狀態
php artisan status:all cluster-a staging

# 輸出應該全部顯示 ✅
# ✅ Gateway: main-gateway (IP: 0.0.0.0) - READY
# ✅ Certificate: tls-cert - READY: True
# ✅ HTTPRoute: api-route - 2 hostnames configured
# ✅ DNS: All records configured correctly
```

## DNS-01 ACME 挑戰原理

理解 DNS-01 挑戰工作原理有助於故障排查。

### 工作流程

1. ✅ **創建 Certificate**：部署 Certificate 資源到 Kubernetes
2. 🔐 **請求 ACME 挑戰**：cert-manager 向 Let's Encrypt 請求憑證
3. 📝 **創建 DNS TXT 記錄**：cert-manager 自動調用 Cloudflare API，創建 `_acmeexample.invalid` TXT 記錄
4. ✅ **Let's Encrypt 驗證**：Let's Encrypt 查詢 DNS TXT 記錄進行驗證
5. 🎉 **簽發憑證**：驗證通過後，Let's Encrypt 簽發憑證
6. 🔒 **存儲憑證**：cert-manager 將憑證存儲到 Kubernetes Secret

### 核心優勢

- ✅ **無需 DNS A 記錄**：驗證過程只需要創建 TXT 記錄，不需要 A 記錄
- ✅ **快速驗證**：通常 1-5 分鐘內完成（vs HTTP-01 的 15-60 分鐘）
- ✅ **支持萬用字元**：可以簽發 `*.example.invalid` 萬用字元憑證
- ✅ **完全自動化**：cert-manager 自動處理 TXT 記錄的創建和清理
- ✅ **無需 Gateway 可訪問**：憑證驗證不需要 Gateway 對外開放

### 為什麼不需要等待 DNS A 記錄？

DNS-01 挑戰使用 **DNS TXT 記錄** 進行驗證，與 DNS A 記錄完全獨立：

```bash
# DNS-01 驗證使用 TXT 記錄
_acmeexample.invalid.  TXT  "random-token-string"

# DNS A 記錄用於實際流量路由（可以稍後配置）
example.invalid.  A  0.0.0.0
```

因此，憑證可以在 DNS A 記錄配置前就驗證完成。

## 常見問題

### Q1: ClusterIssuer 狀態一直不是 READY

檢查 cert-manager 日誌：

```bash
kubectl logs -n cert-manager deployment/cert-manager -f
```

常見原因：
- Cloudflare API Token 權限不足
- Secret 未正確創建
- 網絡連接問題

### Q2: Certificate 一直處於 Pending 狀態

查看 Certificate 詳細狀態：

```bash
kubectl describe certificate -n staging
kubectl get certificaterequest -n staging
kubectl get challenge -n staging
kubectl describe challenge -n staging
```

常見原因：
- Cloudflare API Token Secret 未在 cert-manager namespace 創建（本專案使用 ClusterIssuer）
- DNS TXT 記錄創建失敗
- Cloudflare API 速率限制

### Q3: Gateway 沒有獲得 IP

檢查 Gateway 狀態：

```bash
kubectl describe gateway main-gateway -n staging
```

常見原因：
- GKE Gateway Controller 未正確安裝
- 資源配額不足
- GCP 防火牆規則限制

### Q4: DNS 配置後無法訪問

檢查清單：

```bash
# 1. 驗證 DNS 記錄
dig example.invalid +short

# 2. 檢查 Gateway 狀態
kubectl get gateway -n staging

# 3. 檢查 HTTPRoute 狀態
kubectl get httproute -n staging

# 4. 檢查後端服務
kubectl get svc -n staging
kubectl get pods -n staging
```

## 下一步

恭喜！你已經完成了首次部署。接下來可以：

- 查看 [部署指南](deployment-guide.md) 了解日常部署工作流程
- 查看 [命令參考](commands-reference.md) 了解所有可用命令
- 查看 [架構設計](architecture.md) 深入理解系統設計
- 查看 [故障排除](troubleshooting.md) 解決常見問題
