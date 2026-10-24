# 架構設計

本文檔深入解釋系統設計理念、技術選擇和最佳實踐。

## 目錄

- [設計理念](#設計理念)
- [系統架構](#系統架構)
- [部署順序](#部署順序)
- [DNS-01 ACME 挑戰](#dns-01-acme-挑戰)
- [為什麼選擇 cert-manager](#為什麼選擇-cert-manager)
- [多集群管理](#多集群管理)
- [安全設計](#安全設計)
- [最佳實踐](#最佳實踐)

## 設計理念

### 核心原則

1. **先建設後開放**
   - 所有內部配置（Gateway、Certificate、HTTPRoute）完成後
   - 才配置 DNS 開放外部流量
   - 確保外部流量進來時所有配置已就緒

2. **手動控制，自動化輔助**
   - 所有關鍵操作手動執行
   - 命令行工具提供便捷性
   - GitLab CI 僅用於驗證

3. **安全閘門機制**
   - DNS 配置前自動檢查所有資源狀態
   - 檢查失敗則終止操作
   - 防止誤配置導致服務異常

4. **GitOps 友好**
   - 所有配置納入版本控制
   - 變更可追溯、可回滾
   - 多人協作更安全

## 系統架構

### 架構圖

```
┌─────────────────────────────────────────────────────────────┐
│                          用戶流量                             │
└────────────────┬────────────────────────────────────────────┘
                 │
                 ↓
┌─────────────────────────────────────────────────────────────┐
│                     Cloudflare DNS                           │
│  example.invalid → Gateway IP (34.120.x.x)                   │
└────────────────┬────────────────────────────────────────────┘
                 │
                 ↓
┌─────────────────────────────────────────────────────────────┐
│                      GKE Gateway                             │
│  ┌──────────────┐  ┌────────────────┐                       │
│  │   Listener   │  │  TLS Terminate │                       │
│  │   :80 :443   │  │  cert-manager  │                       │
│  └──────────────┘  │   Secret       │                       │
│                    └────────────────┘                        │
└────────────────┬────────────────────────────────────────────┘
                 │
                 ↓
┌─────────────────────────────────────────────────────────────┐
│                      HTTPRoute                               │
│  ┌───────────────────────────────────────┐                  │
│  │  example.invalid → api-service:8080   │                  │
│  │  example.invalid → app-service:3000   │                  │
│  └───────────────────────────────────────┘                  │
└────────────────┬────────────────────────────────────────────┘
                 │
                 ↓
┌─────────────────────────────────────────────────────────────┐
│                    Backend Services                          │
│  ┌──────────────┐  ┌──────────────┐                         │
│  │ api-service  │  │ app-service  │                         │
│  │    :8080     │  │    :3000     │                         │
│  └──────────────┘  └──────────────┘                         │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│                    cert-manager                              │
│  ┌────────────────────────────────────┐                     │
│  │  DNS-01 Challenge                  │                     │
│  │  ↓                                 │                     │
│  │  Cloudflare API                    │                     │
│  │  創建 TXT: _acme-challenge.*       │                     │
│  │  ↓                                 │                     │
│  │  Let's Encrypt 驗證                │                     │
│  │  ↓                                 │                     │
│  │  簽發憑證 → Secret                 │                     │
│  └────────────────────────────────────┘                     │
└─────────────────────────────────────────────────────────────┘
```

### 組件交互

1. **cert-manager + Cloudflare**
   - cert-manager 調用 Cloudflare API
   - 創建 DNS TXT 記錄 `_acmeexample.invalid`
   - Let's Encrypt 查詢 TXT 記錄驗證域名所有權
   - 驗證通過後簽發憑證
   - 憑證存儲為 Kubernetes Secret

2. **Gateway + Certificate Secret**
   - Gateway 引用 cert-manager 創建的 Secret
   - TLS Termination 在 Gateway 層
   - 自動續期無需手動干預

3. **HTTPRoute + Gateway**
   - HTTPRoute 綁定到特定 Gateway
   - 定義域名到後端服務的路由規則
   - 支持路徑匹配、權重分配等高級功能

4. **Cloudflare DNS + Gateway IP**
   - DNS A 記錄指向 Gateway IP
   - 流量經過 Cloudflare 到達 GKE
   - 可選啟用 Cloudflare Proxy（橙色雲朵）

## 部署順序

### 推薦順序

```
Gateway → Certificate → HTTPRoute → 檢查狀態 → DNS
  ↓          ↓            ↓            ↓         ↓
獲得 IP   DNS-01 驗證   配置路由    確認就緒   開放流量
```

### 為什麼是這個順序？

#### 1. Gateway 優先

**原因：**
- Gateway 部署後獲得 IP 地址
- 這個 IP 是所有後續配置的基礎
- Certificate、HTTPRoute、DNS 都依賴這個 IP

**等待時間：**
- GKE 分配 IP：3-5 分鐘
- Load Balancer 就緒：5-10 分鐘

#### 2. Certificate 第二

**原因：**
- 使用 DNS-01 挑戰，**不需要 DNS A 記錄**
- 可以在 DNS 配置前就完成憑證驗證
- 避免 Gateway 可訪問但憑證未就緒的窗口期

**工作原理：**
```
1. 部署 Certificate
   ↓
2. cert-manager 創建 DNS TXT 記錄
   _acmeexample.invalid → "random-token"
   ↓
3. Let's Encrypt 查詢 TXT 記錄（不是 A 記錄！）
   ↓
4. 驗證通過，簽發憑證
   ↓
5. 憑證存儲為 Secret
```

**等待時間：**
- DNS TXT 記錄創建：10-30 秒
- Let's Encrypt 驗證：1-3 分鐘
- 總計：1-5 分鐘

#### 3. HTTPRoute 第三

**原因：**
- 配置內部路由規則
- 定義域名到後端服務的映射
- 在 DNS 開放前完成路由配置

**為什麼在 DNS 之前？**
- 確保外部流量進來時，路由已配置完成
- 避免 DNS 已指向但路由未配置的窗口期
- 符合"先建設後開放"原則

#### 4. 檢查狀態第四（重要！）

**原因：**
- 在開放外部流量前，確認所有資源就緒
- Gateway 有 IP 且 READY
- Certificate READY: True
- HTTPRoute 已配置

**檢查命令：**
```bash
php artisan status:all cluster-a staging

# 必須確認所有 ✅
✅ Gateway: main-gateway (IP: 0.0.0.0) - READY
✅ Certificate: tls-cert - READY: True
✅ HTTPRoute: api-route - 2 hostnames configured
```

#### 5. DNS 最後

**原因：**
- 所有內部配置完成後，才開放外部流量
- 確保用戶訪問時一切已就緒
- HTTPS 立即可用

**DNS 配置自動檢查：**
```bash
php artisan dns:apply cluster-a staging
# 此命令會自動檢查 Gateway/Certificate/HTTPRoute
# 檢查失敗會終止操作
```

### 不推薦的順序

❌ **先配置 DNS 再部署其他資源**

```
Gateway → DNS → HTTPRoute → Certificate
```

**問題：**
- DNS 已指向但路由未配置：404 錯誤
- DNS 已指向但憑證未就緒：證書錯誤
- 外部流量進來時系統還未準備好

❌ **HTTPRoute 在 DNS 之後**

```
Gateway → Certificate → DNS → HTTPRoute
```

**問題：**
- DNS 已指向但路由未配置
- 外部流量無法正確路由到後端服務
- 造成短暫的服務中斷

## DNS-01 ACME 挑戰

### 什麼是 DNS-01？

DNS-01 是 Let's Encrypt ACME 協議的一種域名驗證方式，通過 DNS TXT 記錄驗證域名所有權。

### 工作流程

```
1. 創建 Certificate
   kubectl apply -f certificate.yaml
   ↓
2. cert-manager 向 Let's Encrypt 請求憑證
   ↓
3. Let's Encrypt 返回 DNS-01 挑戰
   Challenge: "請在 _acmeexample.invalid 創建 TXT 記錄"
   ↓
4. cert-manager 調用 Cloudflare API
   創建 TXT 記錄: _acmeexample.invalid → "random-token"
   ↓
5. cert-manager 通知 Let's Encrypt 開始驗證
   ↓
6. Let's Encrypt 查詢 DNS TXT 記錄
   dig _acmeexample.invalid TXT
   ↓
7. 驗證通過
   Let's Encrypt 簽發憑證
   ↓
8. cert-manager 存儲憑證
   創建 Kubernetes Secret
   ↓
9. cert-manager 清理 TXT 記錄
   刪除 _acmeexample.invalid
```

### DNS-01 vs HTTP-01

| 特性 | DNS-01 | HTTP-01 |
|------|--------|---------|
| **驗證方式** | DNS TXT 記錄 | HTTP `/.well-known/` |
| **需要 DNS A 記錄** | ❌ 不需要 | ✅ 必須先配置 |
| **需要 Gateway 可訪問** | ❌ 不需要 | ✅ 必須可訪問 |
| **驗證速度** | 1-5 分鐘 | 15-60 分鐘 |
| **支持萬用字元** | ✅ `*.example.invalid` | ❌ 不支持 |
| **部署順序靈活性** | ✅ 非常靈活 | ❌ 必須嚴格順序 |
| **API Token 需求** | ✅ 需要 DNS API | ❌ 不需要 |
| **內網環境** | ✅ 支持 | ❌ 不支持 |

### 為什麼 DNS-01 不需要 A 記錄？

**關鍵：DNS-01 只查詢 TXT 記錄，不查詢 A 記錄**

```bash
# Let's Encrypt 驗證時的查詢
dig _acmeexample.invalid TXT

# 返回
_acmeexample.invalid. 300 IN TXT "random-token-from-cert-manager"

# Let's Encrypt 不會查詢 A 記錄！
# 因此不需要 example.invalid 的 A 記錄存在
```

### DNS-01 的優勢

1. **先驗證憑證，後開放流量**
   - 憑證可以在 DNS A 記錄配置前就驗證完成
   - 符合"先建設後開放"原則
   - 更安全的部署流程

2. **支持萬用字元憑證**
   ```yaml
   dnsNames:
   - "*.example.invalid"  # 覆蓋所有子域名
   - "example.invalid"    # 根域名
   ```

3. **驗證速度快**
   - DNS TXT 記錄更新：10-30 秒
   - Let's Encrypt 驗證：1-3 分鐘
   - 總計：1-5 分鐘
   - vs HTTP-01 的 15-60 分鐘

4. **支持內網環境**
   - 不需要 Gateway 對外開放
   - 適合內部服務憑證簽發

## 為什麼選擇 cert-manager

### cert-manager vs GCP ManagedCertificate

| 特性 | cert-manager | GCP ManagedCertificate |
|------|--------------|------------------------|
| **驗證方式** | DNS-01 | HTTP-01 |
| **驗證速度** | 1-5 分鐘 | 15-60 分鐘 |
| **支持萬用字元** | ✅ | ❌ |
| **部署順序** | 靈活 | 嚴格（DNS 必須先配置） |
| **跨雲平台** | ✅ 任何 K8s | ❌ 僅 GKE |
| **自動續期** | ✅ | ✅ |
| **成本** | 免費 | 免費 |

### cert-manager 的優勢

1. **部署順序靈活**
   - Gateway → Certificate → HTTPRoute → DNS
   - Certificate 可以在 DNS 前驗證完成

2. **支持萬用字元憑證**
   - 一個憑證覆蓋多個子域名
   - 減少憑證管理複雜度

3. **跨雲平台**
   - 不綁定 GCP
   - 可以遷移到其他雲平台

4. **開源社區**
   - CNCF 項目
   - 活躍的社區支持
   - 豐富的文檔和範例

## 多集群管理

### 集群架構

```
┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐
│   cluster-a      │  │   cluster-b      │  │   cluster-c      │
│   台灣 (TW)      │  │   日本 (JP)      │  │   美國 (US)      │
│   asia-east1     │  │  asia-northeast1 │  │   us-central1    │
└────────┬─────────┘  └────────┬─────────┘  └────────┬─────────┘
         │                     │                     │
         └─────────────────────┴─────────────────────┘
                              │
                    ┌─────────┴─────────┐
                    │  Cloudflare DNS   │
                    │  GeoDNS Routing   │
                    └───────────────────┘
```

### 地理位置路由

**策略：**
- 台灣/香港用戶 → cluster-a (asia-east1)
- 日本/韓國用戶 → cluster-b (asia-northeast1)
- 美洲用戶 → cluster-c (us-central1)

**實現：**
- 使用 Cloudflare Load Balancer
- 配置地理位置路由規則
- 自動容錯轉移

### 配置一致性

**標籤管理：**
```yaml
labels:
  cluster: cluster-a
  environment: production
  region: asia-east1
  managed-by: gke-gateway-route-manager
```

**模板化配置：**
- 使用相同的 Gateway 配置模板
- Certificate 配置各集群獨立
- HTTPRoute 根據服務定制

## 安全設計

### 密鑰管理

1. **Cloudflare API Token**
   - 儲存為 Kubernetes Secret
   - 每個 namespace 獨立創建
   - 最小權限原則（Zone.DNS Edit + Zone.Zone Read）

2. **GCP Service Account**
   - 僅授予必要的 GKE 訪問權限
   - 定期輪換密鑰
   - 使用 Workload Identity（推薦）

3. **環境變數**
   - 敏感信息不納入版本控制
   - 使用 `.env` 文件（`.gitignore`）
   - 團隊成員各自管理

### 網絡安全

1. **Gateway 安全**
   - TLS Termination 在 Gateway 層
   - 強制 HTTPS（HTTP → HTTPS 重定向）
   - 使用強加密套件

2. **NetworkPolicy**
   ```yaml
   # 限制 Pod 間通信
   apiVersion: example.invalid/v1
   kind: NetworkPolicy
   metadata:
     name: api-network-policy
   spec:
     podSelector:
       matchLabels:
         app: api-service
     policyTypes:
     - Ingress
     - Egress
   ```

### 訪問控制

1. **Kubernetes RBAC**
   - 最小權限原則
   - 按環境劃分權限
   - 定期審計

2. **GitLab 權限**
   - 保護 main 分支
   - 強制 Code Review
   - 限制 Merge 權限

## 最佳實踐

### 設計最佳實踐

1. **單一職責**
   - Gateway：負責流量入口和 TLS
   - Certificate：負責憑證管理
   - HTTPRoute：負責路由規則

2. **配置分離**
   - 每個集群獨立配置目錄
   - 環境隔離（staging/production）
   - 資源類型分離（gateways/certificates/httproutes）

3. **版本控制**
   - 所有配置納入 Git
   - 有意義的 commit message
   - 使用分支管理變更

### 運維最佳實踐

1. **漸進式部署**
   ```
   Dev → Staging → Production-小集群 → Production-大集群
   ```

2. **監控告警**
   - Certificate 到期監控
   - Gateway 可用性監控
   - DNS 解析監控

3. **文檔維護**
   - 更新 Runbook
   - 記錄重大變更
   - 分享經驗教訓

### 安全最佳實踐

1. **最小權限**
   - API Token 僅授予必要權限
   - Service Account 僅訪問特定資源
   - RBAC 細粒度控制

2. **定期審計**
   - 每季度檢查權限
   - 清理未使用的資源
   - 更新密鑰

3. **加密傳輸**
   - 強制 HTTPS
   - 使用 TLS 1.2+
   - 禁用弱加密套件

## 未來擴展

### 可能的改進

1. **自動化程度**
   - 考慮 Flux/ArgoCD GitOps
   - 自動同步配置變更
   - 但保留關鍵步驟的手動控制

2. **多雲支持**
   - 支持 AWS EKS
   - 支持 Azure AKS
   - 統一管理介面

3. **更多 DNS Provider**
   - 支持 Route53
   - 支持 Azure DNS
   - 抽象 DNS 管理層

### 架構演進

```
當前架構：手動部署 + 自動化工具
          ↓
下一階段：半自動化（GitOps + 手動審批）
          ↓
未來階段：全自動化（CI/CD + Policy Gate）
```

## 總結

本架構設計的核心理念是**先建設後開放**：

1. ✅ **Gateway 優先**：獲得 IP 作為基礎
2. ✅ **Certificate 第二**：DNS-01 驗證，不需要 A 記錄
3. ✅ **HTTPRoute 第三**：配置內部路由規則
4. ✅ **檢查狀態第四**：確認所有資源就緒
5. ✅ **DNS 最後**：開放外部流量

這個順序確保：
- 外部流量進來時，一切已準備就緒
- HTTPS 立即可用
- 沒有中間狀態的服務異常
- 更安全、更可控的部署流程

## 下一步

- 查看 [部署指南](deployment-guide.md) 了解實際操作
- 查看 [命令參考](commands-reference.md) 了解工具使用
- 查看 [故障排除](troubleshooting.md) 解決問題
