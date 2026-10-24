# 命令參考

本文檔列出所有可用的 Laravel Artisan 命令及其詳細說明。

## 目錄

- [核心部署命令](#核心部署命令)
- [DNS 管理命令](#dns-管理命令)
- [狀態查詢命令](#狀態查詢命令)
- [智能分析命令](#智能分析命令)
- [集群管理命令](#集群管理命令)
- [驗證命令](#驗證命令)
- [刪除命令](#刪除命令)
- [命令參數說明](#命令參數說明)

## 核心部署命令

### deploy:k8s

**合併部署**所有 Kubernetes 資源（Gateway + Certificate + HTTPRoute）。

```bash
php artisan deploy:k8s <cluster> <env>
```

**參數：**
- `cluster`: 集群名稱（例如：cluster-a）
- `env`: 環境名稱（例如：staging, production）

**執行步驟：**
1. 連接到指定的 GKE 集群
2. 部署 Gateway 並等待獲得 IP（最多 5 分鐘）
3. 自動生成 Certificate 和 DNS 配置
4. 部署 Certificate（開始 DNS-01 驗證）
5. 部署 HTTPRoute（配置路由規則）
6. 等待 Certificate 驗證完成（READY: True，最多 5 分鐘）
7. 顯示所有資源狀態摘要

**範例：**
```bash
# 部署到 cluster-a 的 staging 環境
php artisan deploy:k8s cluster-a staging

# 部署到 cluster-b 的 production 環境
php artisan deploy:k8s cluster-b production
```

**輸出示例：**
```
✅ 連接到集群: cluster-a (staging)
✅ 部署 Gateway...
⏳ 等待 Gateway 獲得 IP...
✅ Gateway IP: 0.0.0.0
✅ 生成 Certificate 和 DNS 配置...
✅ 部署 Certificate...
⏳ 等待憑證驗證完成...
✅ 憑證驗證完成！
✅ 部署 HTTPRoute...
✅ 所有資源部署完成
```

**使用場景：**
- ✅ 日常部署新服務
- ✅ 批量部署多個集群
- ✅ 快速迭代測試

---

### deploy:gateway

部署 Gateway 並等待獲得 IP。

```bash
php artisan deploy:gateway <cluster> <env>
```

**參數：**
- `cluster`: 集群名稱
- `env`: 環境名稱

**執行步驟：**
1. 從 config/clusters.yaml 讀取集群配置
2. 連接到 GKE 集群
3. 檢查 clusters/{cluster}/{env}/gateways/ 目錄
4. 應用所有 Gateway YAML 文件
5. 等待 Gateway 獲得 IP（最多 5 分鐘）
6. 顯示 Gateway IP

**範例：**
```bash
php artisan deploy:gateway cluster-a staging
```

**輸出示例：**
```
✅ 連接到集群: cluster-a (staging)
✅ 應用 Gateway 配置: clusters/cluster-a/staging/gateways/
⏳ 等待 Gateway 獲得 IP...
✅ Gateway IP: 0.0.0.0
```

---

### deploy:certificate

部署 Certificate 並觸發 DNS-01 驗證。

```bash
php artisan deploy:certificate <cluster> <env>
```

**參數：**
- `cluster`: 集群名稱
- `env`: 環境名稱

**執行步驟：**
1. 連接到 GKE 集群
2. 檢查 `cloudflare-api-token` Secret 是否存在於 cert-manager namespace
3. 應用 cert-manager Certificate 配置
4. cert-manager 自動創建 DNS TXT 記錄進行 DNS-01 驗證
5. 等待 Let's Encrypt 驗證完成
6. 顯示憑證狀態和 Secret 信息

**範例：**
```bash
php artisan deploy:certificate cluster-a staging
```

**輸出示例：**
```
✅ 連接到集群: cluster-a (staging)
✅ 檢查 Cloudflare API Token Secret...
✅ 應用 Certificate 配置: clusters/cluster-a/staging/certificates/
🔐 cert-manager 開始處理...
📝 創建 DNS TXT 記錄進行 DNS-01 驗證
⏳ 等待 Let's Encrypt 驗證...
✅ 憑證已簽發！
📋 憑證狀態: Ready (通常 1-5 分鐘)
🔒 Secret 已創建: tls-cert
```

---

### deploy:httproute

部署 HTTPRoute 配置路由規則。

```bash
php artisan deploy:httproute <cluster> <env>
```

**參數：**
- `cluster`: 集群名稱
- `env`: 環境名稱

**執行步驟：**
1. 連接到 GKE 集群
2. 檢查 Certificate 狀態（僅警告，不阻止部署）
3. 應用 HTTPRoute 配置
4. 顯示 HTTPRoute 狀態

**範例：**
```bash
php artisan deploy:httproute cluster-a staging
```

**輸出示例：**
```
✅ 連接到集群: cluster-a (staging)
🔍 檢查憑證狀態...
   憑證狀態: Ready
✅ 應用 HTTPRoute 配置: clusters/cluster-a/staging/httproutes/
✅ HTTPRoute 部署完成
```

---

## DNS 管理命令

### dns:apply

應用 DNS 配置 YAML 到 Cloudflare。

```bash
php artisan dns:apply <cluster> <env>
```

**⚠️ 重要：此命令會自動檢查前置條件**
- Gateway 是否有 IP 且 READY
- Certificate 是否 READY: True
- HTTPRoute 是否已配置

如果檢查失敗，命令會終止並顯示錯誤，防止誤配置。

**參數：**
- `cluster`: 集群名稱
- `env`: 環境名稱

**執行步驟：**
1. 連接到 GKE 集群
2. 檢查前置條件（Gateway/Certificate/HTTPRoute）
3. 如果檢查失敗，顯示錯誤並終止
4. 如果檢查通過，讀取 `clusters/{cluster}/{env}/dns-config.yaml`
5. 為每個域名智能查詢對應的 Cloudflare Zone ID
6. 檢查 DNS 記錄是否已存在
7. 創建或更新 DNS A 記錄
8. 顯示配置結果

**範例：**
```bash
# 設置環境變數
export CLOUDFLARE_API_TOKEN=[REDACTED]"

# 應用 DNS 配置
php artisan dns:apply cluster-a staging
```

**輸出示例：**
```
🔍 檢查前置條件...
✅ Gateway: main-gateway (IP: 0.0.0.0) - READY
✅ Certificate: tls-cert - READY: True
✅ HTTPRoute: api-route - 2 hostnames configured

📋 應用 DNS 配置...
🔍 智能查詢 Zone ID: example.invalid
  ✅ 找到 Zone: example.invalid (ID: abc123...)
✅ DNS A 記錄: example.invalid -> 0.0.0.0
✅ DNS A 記錄: example.invalid -> 0.0.0.0

✅ DNS 配置完成
```

---

### dns:verify

驗證 DNS 配置是否正確。

```bash
php artisan dns:verify <cluster> <env>
```

**參數：**
- `cluster`: 集群名稱
- `env`: 環境名稱

**執行步驟：**
1. 讀取 `clusters/{cluster}/{env}/dns-config.yaml`
2. 連接到 GKE 集群獲取 Gateway IP
3. 使用 `dig` 查詢每個域名的 DNS A 記錄
4. 比對實際 IP 與預期 IP
5. 顯示驗證結果

**範例：**
```bash
php artisan dns:verify cluster-a staging
```

**輸出示例：**
```
🔍 驗證 DNS 配置...
Gateway IP: 0.0.0.0

✅ example.invalid -> 0.0.0.0 (正確)
✅ example.invalid -> 0.0.0.0 (正確)
❌ example.invalid -> 0.0.0.0 (錯誤)
   預期: 0.0.0.0
   實際: 0.0.0.0

總計: 2 個正確, 1 個錯誤
```

---

## 狀態查詢命令

### status:all

顯示所有資源（Gateway + Certificate + HTTPRoute）的狀態。

```bash
php artisan status:all <cluster> <env>
```

**參數：**
- `cluster`: 集群名稱
- `env`: 環境名稱

**範例：**
```bash
php artisan status:all cluster-a staging
```

**輸出示例：**
```
📋 集群狀態: cluster-a (staging)

✅ Gateway: main-gateway (IP: 0.0.0.0) - READY
✅ Certificate: tls-cert - READY: True
✅ HTTPRoute: api-route - 2 hostnames configured

📋 DNS 配置預覽：
  - example.invalid -> 0.0.0.0
  - example.invalid -> 0.0.0.0
```

---

### status:gateway

顯示 Gateway 狀態和 IP。

```bash
php artisan status:gateway <cluster> <env>
```

**範例：**
```bash
php artisan status:gateway cluster-a staging
```

**輸出示例：**
```
📋 Gateway 狀態

Gateway: main-gateway
Namespace: staging
Status: READY
IP: 0.0.0.0
Age: 5d3h
```

---

### status:certificate

顯示 Certificate 狀態。

```bash
php artisan status:certificate <cluster> <env>
```

**範例：**
```bash
php artisan status:certificate cluster-a staging
```

**輸出示例：**
```
📋 Certificate 狀態

Certificate: tls-cert
Namespace: staging
Status: READY: True
Secret: [REDACTED]
DNS Names:
  - example.invalid
  - example.invalid
Age: 2h15m
```

---

### status:httproute

顯示 HTTPRoute 狀態。

```bash
php artisan status:httproute <cluster> <env>
```

**範例：**
```bash
php artisan status:httproute cluster-a staging
```

**輸出示例：**
```
📋 HTTPRoute 狀態

HTTPRoute: api-route
Namespace: staging
Hostnames:
  - example.invalid
  - example.invalid
Parent Gateway: main-gateway
Backend: api-service:8080
Age: 1h30m
```

---

## 智能分析命令

### generate:config

從 HTTPRoute 自動生成 Certificate 和 DNS 配置。

```bash
php artisan generate:config <cluster> <env>
```

**參數：**
- `cluster`: 集群名稱
- `env`: 環境名稱

**執行步驟：**
1. 連接到 GKE 集群
2. 讀取所有 HTTPRoute 配置
3. 提取所有 `spec.hostnames` 中的域名
4. 生成 Certificate YAML：`clusters/{cluster}/{env}/certificates/tls-cert.yaml`
5. 生成 DNS 配置 YAML：`clusters/{cluster}/{env}/dns-config.yaml`
6. 顯示配置摘要

**範例：**
```bash
php artisan generate:config cluster-a staging
```

**輸出示例：**
```
📋 分析 HTTPRoute 配置...
發現以下域名：
  - example.invalid
  - example.invalid

✅ 生成 Certificate 配置: clusters/cluster-a/staging/certificates/tls-cert.yaml
✅ 生成 DNS 配置: clusters/cluster-a/staging/dns-config.yaml

📋 配置摘要：
  憑證涵蓋 2 個域名
  DNS 需要配置 2 筆 A 記錄
  Gateway IP: 0.0.0.0
```

---

### analyze:domains

分析 HTTPRoute 需要的域名（僅查看，不生成配置）。

```bash
php artisan analyze:domains <cluster> <env>
```

**參數：**
- `cluster`: 集群名稱
- `env`: 環境名稱

**範例：**
```bash
php artisan analyze:domains cluster-a staging
```

**輸出示例：**
```
📋 分析 HTTPRoute 配置...

發現以下域名需要配置：
  - example.invalid
  - example.invalid
  - example.invalid

需要的 Certificate 配置：
  ✓ example-com-cert 已存在
    包含域名: example.invalid, example.invalid
  ✗ staging-example-com-cert 不存在
    需要域名: example.invalid

需要的 DNS 配置：
  Gateway IP: 0.0.0.0
  - example.invalid -> 0.0.0.0 (✓ 已配置)
  - example.invalid -> 0.0.0.0 (✗ 未配置)
  - example.invalid -> 0.0.0.0 (✗ 未配置)
```

---

## 集群管理命令

### cluster:connect

連接到指定集群（用於手動操作）。

```bash
php artisan cluster:connect <cluster> <env>
```

**參數：**
- `cluster`: 集群名稱
- `env`: 環境名稱

**範例：**
```bash
# 連接到 cluster-a 的 staging 環境
php artisan cluster:connect cluster-a staging

# 之後可以直接使用 kubectl
kubectl get gateway -n staging
kubectl get certificate -n staging
kubectl get httproute -n staging
```

---

### cluster:list

列出所有配置的集群。

```bash
php artisan cluster:list
```

**範例：**
```bash
php artisan cluster:list
```

**輸出示例：**
```
📋 配置的集群列表

cluster-a:
  Name: gke-tw-prod
  Project: my-gcp-project
  Region: asia-east1
  Environments: staging, production

cluster-b:
  Name: gke-jp-prod
  Project: my-gcp-project
  Region: asia-northeast1
  Environments: staging, production

cluster-c:
  Name: gke-us-prod
  Project: my-gcp-project-us
  Region: us-central1
  Environments: production
```

---

## 驗證命令

### validate:config

驗證所有配置文件格式和內容。

```bash
php artisan validate:config
```

**檢查項目：**
- YAML 文件語法正確性
- 必填字段完整性
- Gateway 配置格式
- Certificate 配置格式
- HTTPRoute 配置格式
- DNS 配置格式
- 集群配置完整性

**範例：**
```bash
php artisan validate:config
```

**輸出示例：**
```
🔍 驗證配置文件...

✅ clusters/cluster-a/staging/gateways/main-gateway.yaml
✅ clusters/cluster-a/staging/certificates/tls-cert.yaml
✅ clusters/cluster-a/staging/httproutes/api-route.yaml
✅ clusters/cluster-a/staging/dns-config.yaml
❌ clusters/cluster-b/staging/httproutes/app-route.yaml
   錯誤: 缺少必填字段 spec.parentRefs

總計: 4 個文件通過, 1 個文件失敗
```

---

## 刪除命令

**⚠️ 警告：刪除操作不可逆，請謹慎使用！**

### delete:httproute

刪除 HTTPRoute。

```bash
php artisan delete:httproute <cluster> <env>
```

**範例：**
```bash
php artisan delete:httproute cluster-a staging
```

---

### delete:certificate

刪除 Certificate。

```bash
php artisan delete:certificate <cluster> <env>
```

**範例：**
```bash
php artisan delete:certificate cluster-a staging
```

---

### delete:gateway

刪除 Gateway。

**⚠️ 警告：刪除 Gateway 會導致服務中斷！**

```bash
php artisan delete:gateway <cluster> <env>
```

**範例：**
```bash
php artisan delete:gateway cluster-a staging
```

---

## 命令參數說明

### cluster

集群名稱，必須在 `config/clusters.yaml` 中定義。

**範例：**
- `cluster-a`
- `cluster-b`
- `cluster-c`

### env

環境名稱，對應 Kubernetes namespace。

**常見值：**
- `staging`
- `production`
- `dev`

**對應關係：**
```yaml
# config/environments.yaml
namespaces:
  staging: staging
  prod: production
  dev: dev
```

---

## 命令組合範例

### 完整部署流程（方案 A）

```bash
# 1. 合併部署所有 K8s 資源
php artisan deploy:k8s cluster-a staging

# 2. 檢查狀態
php artisan status:all cluster-a staging

# 3. 配置 DNS
php artisan dns:apply cluster-a staging

# 4. 驗證 DNS
php artisan dns:verify cluster-a staging
```

### 完整部署流程（方案 B）

```bash
# 1. 部署 Gateway
php artisan deploy:gateway cluster-a staging

# 2. 生成配置
php artisan generate:config cluster-a staging

# 3. 部署 Certificate
php artisan deploy:certificate cluster-a staging

# 4. 等待憑證驗證
kubectl get certificate -n staging --watch

# 5. 部署 HTTPRoute
php artisan deploy:httproute cluster-a staging

# 6. 檢查狀態
php artisan status:all cluster-a staging

# 7. 配置 DNS
php artisan dns:apply cluster-a staging

# 8. 驗證 DNS
php artisan dns:verify cluster-a staging
```

### 故障排查

```bash
# 1. 檢查所有資源狀態
php artisan status:all cluster-a staging

# 2. 檢查 Gateway 詳細狀態
php artisan status:gateway cluster-a staging
kubectl describe gateway main-gateway -n staging

# 3. 檢查 Certificate 詳細狀態
php artisan status:certificate cluster-a staging
kubectl describe certificate auto-generated-cert -n staging
kubectl get challenge -n staging

# 4. 檢查 HTTPRoute 詳細狀態
php artisan status:httproute cluster-a staging
kubectl describe httproute api-route -n staging

# 5. 驗證 DNS
php artisan dns:verify cluster-a staging
```

### 批量部署

```bash
# 部署到多個集群
for cluster in cluster-a cluster-b cluster-c; do
  php artisan deploy:k8s $cluster staging
  php artisan status:all $cluster staging
  php artisan dns:apply $cluster staging
done
```

---

## 常見問題

### Q: 命令執行失敗怎麼辦？

1. 檢查 kubectl 是否已連接到正確集群
2. 檢查環境變數是否設置正確（CLOUDFLARE_API_TOKEN）
3. 檢查配置文件格式是否正確
4. 查看詳細錯誤信息

### Q: 如何查看命令的詳細幫助？

```bash
php artisan help <command>

# 例如
php artisan help deploy:k8s
```

### Q: 如何列出所有可用命令？

```bash
php artisan list
```

### Q: 命令執行很慢怎麼辦？

- 等待 Gateway 獲得 IP 需要 3-5 分鐘
- 等待 Certificate 驗證需要 1-5 分鐘
- DNS 傳播需要 1-5 分鐘
- 這些都是正常等待時間

### Q: 可以跳過某些步驟嗎？

使用方案 B（分步驟部署）可以完全控制每個步驟。

---

## 下一步

- 查看 [部署指南](deployment-guide.md) 了解工作流程
- 查看 [架構設計](architecture.md) 深入理解系統
- 查看 [故障排除](troubleshooting.md) 解決問題
