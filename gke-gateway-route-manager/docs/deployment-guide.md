# 部署指南

本指南涵蓋日常部署工作流程、常見場景和最佳實踐。

## 目錄

- [部署方式](#部署方式)
- [常見工作流程](#常見工作流程)
- [多集群部署](#多集群部署)
- [變更管理](#變更管理)
- [CI/CD 集成](#cicd-集成)
- [最佳實踐](#最佳實踐)

## 部署方式

本專案提供兩種部署方式，根據你的需求選擇。

### 方案 A：合併部署（推薦）

最簡單的部署方式，適合日常使用和批量部署。

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

# 3. 確認所有 ✅ 後，配置 DNS 開放流量
php artisan dns:apply cluster-a staging

# 4. 驗證 DNS 生效
php artisan dns:verify cluster-a staging
```

**適用場景：**
- ✅ 日常部署新服務
- ✅ 批量部署多個集群
- ✅ 快速迭代測試

**優點：**
- 操作簡單，只需 4 個命令
- 自動等待憑證驗證
- 自動檢測並報告錯誤
- 減少人為錯誤

### 方案 B：分步驟部署（細粒度控制）

需要更細緻控制或排查問題時使用。

```bash
# 1. 部署 Gateway 並獲取 IP
php artisan deploy:gateway cluster-a staging
# 輸出: Gateway IP: 0.0.0.0

# 2. 生成配置（Certificate 和 DNS）
php artisan generate:config cluster-a staging

# 3. 部署 Certificate（DNS-01 驗證）
php artisan deploy:certificate cluster-a staging

# 4. 等待憑證驗證完成（1-5 分鐘）
kubectl get certificate -n staging --watch
# 確認 READY: True

# 5. 部署 HTTPRoute（配置路由規則）
php artisan deploy:httproute cluster-a staging

# 6. 檢查所有資源狀態（重要！）
php artisan status:all cluster-a staging

# 7. 配置 DNS A 記錄（開放外部流量）
php artisan dns:apply cluster-a staging

# 8. 驗證 DNS 生效
php artisan dns:verify cluster-a staging
```

**適用場景：**
- ✅ 故障排查
- ✅ 學習理解部署流程
- ✅ 特殊情況需要暫停某個步驟

**優點：**
- 完全控制每個步驟
- 方便中途檢查和調整
- 更容易理解部署原理

## 常見工作流程

### 新增服務

從零開始部署一個新服務的完整流程。

#### Step 1: 創建配置文件

```bash
# 創建 HTTPRoute 配置（定義域名和路由規則）
vim clusters/cluster-a/staging/httproutes/new-service.yaml
```

範例配置：

```yaml
apiVersion: example.invalid/v1
kind: HTTPRoute
metadata:
  name: new-service-route
  namespace: staging
spec:
  parentRefs:
  - name: main-gateway
  hostnames:
  - "example.invalid"
  rules:
  - matches:
    - path:
        type: PathPrefix
        value: /
    backendRefs:
    - name: new-service
      port: 8080
```

同時準備 Gateway 配置（如果尚未存在）：

```bash
vim clusters/cluster-a/staging/gateways/main-gateway.yaml
```

#### Step 2: 部署並配置 DNS

```bash
# 使用合併部署（推薦）
php artisan deploy:k8s cluster-a staging
php artisan status:all cluster-a staging
php artisan dns:apply cluster-a staging
```

#### Step 3: 驗證部署

```bash
# 驗證 DNS
php artisan dns:verify cluster-a staging

# 測試 HTTP/HTTPS
curl -I https://example.invalid
```

### 更新現有配置

修改已部署的資源配置。

#### 更新 Gateway

```bash
# 1. 修改配置文件
vim clusters/cluster-a/staging/gateways/main-gateway.yaml

# 2. 重新部署 Gateway
php artisan deploy:gateway cluster-a staging

# 3. 檢查狀態
php artisan status:gateway cluster-a staging
```

#### 更新 Certificate

```bash
# 1. 修改配置（例如添加新域名）
vim clusters/cluster-a/staging/certificates/tls-cert.yaml

# 2. 重新部署 Certificate
php artisan deploy:certificate cluster-a staging

# 3. 等待憑證更新（如有新域名）
kubectl get certificate -n staging --watch

# 4. 檢查狀態
php artisan status:certificate cluster-a staging
```

#### 更新 HTTPRoute

```bash
# 1. 修改配置（例如修改路由規則）
vim clusters/cluster-a/staging/httproutes/api-route.yaml

# 2. 重新部署 HTTPRoute
php artisan deploy:httproute cluster-a staging

# 3. 檢查狀態
php artisan status:httproute cluster-a staging

# 4. 測試路由
curl -I https://example.invalid/new-path
```

#### 更新 DNS

```bash
# 1. 修改 DNS 配置（如果手動創建）
vim clusters/cluster-a/staging/dns-config.yaml

# 2. 應用更新
php artisan dns:apply cluster-a staging

# 3. 驗證更新
php artisan dns:verify cluster-a staging
```

### 刪除服務

安全地刪除已部署的服務。

```bash
# 1. 先刪除 DNS（停止外部流量）
# 手動在 Cloudflare Dashboard 刪除 A 記錄
# 或修改 dns-config.yaml 並重新應用

# 2. 等待 DNS 傳播（1-5 分鐘）
dig example.invalid +short

# 3. 刪除 HTTPRoute
kubectl delete httproute api-route -n staging

# 4. 檢查是否有其他服務使用該憑證
php artisan status:certificate cluster-a staging

# 5. 如無其他服務使用，可刪除 Certificate
kubectl delete certificate tls-cert -n staging

# 6. 檢查是否有其他服務使用該 Gateway
php artisan status:gateway cluster-a staging

# 7. 如無其他服務使用，可刪除 Gateway
kubectl delete gateway main-gateway -n staging
```

**重要：刪除順序**
```
DNS → HTTPRoute → Certificate → Gateway
 ↓         ↓          ↓            ↓
停流量   刪路由    刪憑證      刪網關
```

### 添加域名到現有服務

為現有 HTTPRoute 添加新域名。

#### Step 1: 更新 HTTPRoute

```bash
# 編輯 HTTPRoute 添加新域名
vim clusters/cluster-a/staging/httproutes/api-route.yaml
```

```yaml
spec:
  hostnames:
  - "example.invalid"    # 原有域名
  - "example.invalid" # 新添加的域名
```

#### Step 2: 更新 Certificate

```bash
# 編輯 Certificate 添加新域名
vim clusters/cluster-a/staging/certificates/tls-cert.yaml
```

```yaml
spec:
  dnsNames:
  - example.invalid
  - example.invalid
  - example.invalid   # 新添加的域名
```

#### Step 3: 部署更新

```bash
# 1. 部署 Certificate 更新
php artisan deploy:certificate cluster-a staging

# 2. 等待憑證更新完成
kubectl get certificate -n staging --watch

# 3. 部署 HTTPRoute 更新
php artisan deploy:httproute cluster-a staging

# 4. 檢查狀態
php artisan status:all cluster-a staging

# 5. 配置新域名的 DNS
php artisan dns:apply cluster-a staging

# 6. 驗證
curl -I https://example.invalid
```

## 多集群部署

批量部署到多個集群。

### 批量部署相同配置

部署相同服務到多個集群（例如不同區域）。

```bash
# 方式 1: 使用 for 循環
for cluster in cluster-a cluster-b cluster-c; do
  echo "部署到 $cluster..."
  php artisan deploy:k8s $cluster staging
  php artisan status:all $cluster staging
  php artisan dns:apply $cluster staging
done

# 方式 2: 使用 xargs 並行部署（更快）
echo "cluster-a cluster-b cluster-c" | xargs -n1 -P3 -I {} sh -c '
  php artisan deploy:k8s {} staging &&
  php artisan status:all {} staging &&
  php artisan dns:apply {} staging
'
```

### 分階段部署（金絲雀部署）

先部署到 staging，驗證後再部署到 production。

```bash
# Stage 1: 部署到 staging
php artisan deploy:k8s cluster-a staging
php artisan status:all cluster-a staging
php artisan dns:apply cluster-a staging

# Stage 2: 驗證 staging
curl -I https://example.invalid
# 運行測試...

# Stage 3: 部署到 production
php artisan deploy:k8s cluster-a production
php artisan status:all cluster-a production
php artisan dns:apply cluster-a production

# Stage 4: 驗證 production
curl -I https://example.invalid
```

### 區域優先部署

按區域優先順序部署（例如先亞洲，後美洲）。

```bash
# 1. 亞洲集群
for cluster in cluster-tw cluster-jp cluster-sg; do
  php artisan deploy:k8s $cluster production
  php artisan status:all $cluster production
  php artisan dns:apply $cluster production
done

# 2. 驗證亞洲集群
# 運行健康檢查...

# 3. 美洲集群
for cluster in cluster-us cluster-ca; do
  php artisan deploy:k8s $cluster production
  php artisan status:all $cluster production
  php artisan dns:apply $cluster production
done
```

## 變更管理

### Git 工作流程

本專案使用 GitOps 模式，所有配置都納入版本控制。

```bash
# 1. 創建功能分支
git checkout -b feature/add-new-service

# 2. 創建或修改配置文件
vim clusters/cluster-a/staging/httproutes/new-service.yaml

# 3. 提交變更
git add clusters/cluster-a/staging/httproutes/new-service.yaml
git commit -m "Add new service HTTPRoute for staging"

# 4. 推送分支
git push origin feature/add-new-service

# 5. 創建 Merge Request
# 在 GitLab 創建 MR 並等待審核

# 6. 合併後手動部署
git checkout main
git pull
php artisan deploy:k8s cluster-a staging
php artisan status:all cluster-a staging
php artisan dns:apply cluster-a staging
```

### 審核流程

建議的審核清單：

- [ ] HTTPRoute 配置格式正確
- [ ] 域名符合命名規範
- [ ] 後端服務已存在
- [ ] Certificate 包含所需域名
- [ ] Gateway 配置正確引用 Certificate Secret
- [ ] 環境變數（staging/production）正確
- [ ] 命名空間正確

### 回滾變更

如果部署後發現問題，快速回滾。

```bash
# 方式 1: Git 回滾配置
git revert <commit-hash>
git push origin main

# 重新部署舊配置
php artisan deploy:k8s cluster-a staging
php artisan status:all cluster-a staging
# DNS 通常不需要回滾（IP 未變）

# 方式 2: Kubernetes 資源回滾
kubectl rollout undo deployment/your-app -n staging

# 方式 3: DNS 回滾（如果 IP 變更）
# 編輯 dns-config.yaml 恢復舊 IP
php artisan dns:apply cluster-a staging
```

## CI/CD 集成

本專案使用 GitLab CI **僅用於配置驗證**，實際部署透過**手動執行命令**。

### GitLab CI 配置範例

`.gitlab-ci.yml`:

```yaml
stages:
  - validate

validate-config:
  stage: validate
  image: php:8.4-cli
  before_script:
    - apt-get update && apt-get install -y git
    - composer install --no-dev
  script:
    # 驗證配置文件格式
    - php artisan validate:config

    # 檢查 YAML 語法
    - |
      for file in $(find clusters -name "*.yaml"); do
        echo "Checking $file..."
        php -r "yaml_parse_file('$file');"
      done
  only:
    - merge_requests
    - main
```

### 手動部署流程

**不使用自動部署的原因：**
- ✅ **更安全**：每個變更都需要手動確認
- ✅ **更可控**：可以選擇部署時機
- ✅ **更靈活**：可以跳過某些步驟
- ✅ **更容易排查**：問題發生時更容易定位

**推薦部署流程：**

```bash
# 1. 確保在 devcontainer 中
# 2. 拉取最新代碼
git pull origin main

# 3. 驗證配置
php artisan validate:config

# 4. 部署到 staging
php artisan deploy:k8s cluster-a staging
php artisan status:all cluster-a staging
php artisan dns:apply cluster-a staging

# 5. 驗證 staging
curl -I https://example.invalid
# 運行測試...

# 6. 部署到 production
php artisan deploy:k8s cluster-a production
php artisan status:all cluster-a production
php artisan dns:apply cluster-a production

# 7. 驗證 production
curl -I https://example.invalid
```

## 最佳實踐

### 部署前檢查清單

每次部署前檢查：

- [ ] Git 分支已更新到最新
- [ ] 配置文件格式正確（YAML 語法）
- [ ] 後端服務已部署且健康
- [ ] Cloudflare API Token 有效
- [ ] kubectl 已連接到正確集群
- [ ] 命名空間存在且正確
- [ ] Cloudflare API Token Secret 已在 cert-manager namespace 創建（使用 ClusterIssuer）

### 安全最佳實踐

1. **DNS 最後配置**
   - 始終將 DNS 配置作為最後一步
   - 確保所有內部資源就緒後才開放流量

2. **使用 status:all 檢查**
   ```bash
   php artisan status:all cluster-a staging
   # 必須確認所有資源都顯示 ✅
   ```

3. **分環境部署**
   - 先部署 staging，驗證後再部署 production
   - 不要跳過 staging 直接部署 production

4. **保護 API Token**
   - 使用環境變數或 `.env` 文件
   - 不要將 Token 提交到 Git
   - 定期輪換 Token

### 性能最佳實踐

1. **使用萬用字元憑證**
   ```yaml
   # 一個憑證覆蓋多個子域名
   dnsNames:
   - "*.example.invalid"
   - "example.invalid"
   ```

2. **批量部署使用並行**
   ```bash
   # 使用 xargs -P3 並行部署 3 個集群
   echo "cluster-a cluster-b cluster-c" | xargs -n1 -P3 -I {}...
   ```

3. **復用 Gateway**
   - 同一集群的多個服務共用一個 Gateway
   - 減少 IP 消耗和成本

### 維護最佳實踐

1. **定期檢查憑證狀態**
   ```bash
   kubectl get certificate -A
   # 確認所有憑證 READY: True
   ```

2. **監控憑證過期時間**
   ```bash
   kubectl describe certificate -n production | grep "Not After"
   ```

3. **清理未使用的資源**
   ```bash
   # 刪除未使用的 Certificate
   # 刪除未使用的 Gateway
   # 刪除未使用的 DNS 記錄
   ```

4. **記錄變更**
   - 每次變更都寫清楚的 Git commit message
   - 重大變更記錄在文檔中
   - 保留 DNS 配置變更歷史

### 故障排查最佳實踐

1. **使用分步驟部署排查問題**
   - 遇到問題時使用方案 B（分步驟部署）
   - 逐步確認每個資源狀態

2. **查看詳細日誌**
   ```bash
   # cert-manager 日誌
   kubectl logs -n cert-manager deployment/cert-manager -f

   # Gateway Controller 日誌
   kubectl logs -n gke-system deployment/gateway-controller -f

   # 查看 Challenge 詳情
   kubectl describe challenge -n staging
   ```

3. **驗證 Cloudflare 配置**
   ```bash
   # 手動驗證 DNS
   dig example.invalid +short

   # 驗證 TXT 記錄（DNS-01）
   dig _acmeexample.invalid TXT +short
   ```

## 下一步

- 查看 [命令參考](commands-reference.md) 了解所有可用命令
- 查看 [架構設計](architecture.md) 深入理解系統設計
- 查看 [故障排除](troubleshooting.md) 解決常見問題
