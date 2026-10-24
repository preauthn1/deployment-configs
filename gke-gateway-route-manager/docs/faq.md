# 常見問題（FAQ）

本文檔回答關於 GKE Gateway Route Manager 專案的常見問題。

## 目錄

- [基礎概念](#基礎概念)
- [部署順序](#部署順序)
- [DNS-01 ACME 挑戰](#dns-01-acme-挑戰)
- [cert-manager](#cert-manager)
- [Gateway](#gateway)
- [HTTPRoute](#httproute)
- [DNS 管理](#dns-管理)
- [多集群管理](#多集群管理)
- [安全性](#安全性)
- [故障排查](#故障排查)

## 基礎概念

### Q1: 這個專案是做什麼的？

**A:** 統一管理多集群 GKE Gateway、Cloudflare DNS 和 cert-manager 憑證的 Laravel CLI 工具。

**核心功能：**
- 🚀 一鍵部署所有 Kubernetes 資源（Gateway + Certificate + HTTPRoute）
- 🔒 DNS-01 驗證：憑證驗證不需要 DNS A 記錄
- 🛡️ 安全閘門：DNS 配置前自動檢查所有資源狀態
- 📋 智能配置：從 HTTPRoute 自動生成 Certificate 和 DNS 配置
- 🌐 多集群管理：統一管理跨區域、跨專案的 GKE 集群

### Q2: 為什麼要用這個工具？

**A:** 傳統方式管理多集群的域名、憑證、路由配置很繁瑣：

**傳統方式的問題：**
- ❌ 手動管理多個集群的配置文件
- ❌ 手動配置 Cloudflare DNS
- ❌ 手動管理 SSL/TLS 憑證
- ❌ 容易出錯、難以追蹤變更

**本工具的優勢：**
- ✅ 統一命令管理所有資源
- ✅ 自動生成配置減少重複工作
- ✅ 版本控制所有配置
- ✅ 安全檢查防止誤操作
- ✅ 智能反推功能簡化部署

### Q3: 為什麼不使用 GitLab CI 自動部署？

**A:** 手動部署更安全、更可控。

**手動部署的優勢：**
- ✅ 每個步驟都可控
- ✅ 可以隨時暫停檢查
- ✅ 避免自動化誤操作
- ✅ 更容易排查問題
- ✅ 符合變更管理最佳實踐

**GitLab CI 的定位：**
- 僅用於配置驗證（YAML 語法、格式檢查）
- 不執行實際部署操作

## 部署順序

### Q4: 為什麼部署順序是 Gateway → Certificate → HTTPRoute → DNS？

**A:** 這個順序遵循"先建設後開放"原則。

**詳細說明：**

1. **Gateway 優先**
   - 提供 IP 地址作為所有配置的基礎
   - 等待時間：3-5 分鐘

2. **Certificate 第二**
   - 使用 DNS-01 驗證，不需要 DNS A 記錄
   - 可以在 DNS 配置前完成憑證驗證
   - 等待時間：1-5 分鐘

3. **HTTPRoute 第三**
   - 配置內部路由規則
   - 在 DNS 開放前完成路由配置
   - 確保外部流量進來時路由已就緒

4. **檢查狀態第四**
   - 確認所有資源都已就緒
   - Gateway 有 IP 且 READY
   - Certificate READY: True
   - HTTPRoute 已配置

5. **DNS 最後**
   - 所有內部配置完成後才開放外部流量
   - 確保用戶訪問時一切已就緒
   - HTTPS 立即可用

### Q5: 可以先配置 DNS 再部署其他資源嗎？

**A:** 不推薦，會造成中間狀態。

**問題：**
- ⚠️ DNS 已指向但路由未配置：404 錯誤
- ⚠️ DNS 已指向但憑證未就緒：證書錯誤
- ⚠️ 外部流量進來時系統還未準備好

**推薦做法：**
始終將 DNS 配置作為最後一步，確保所有內部配置完成後才開放外部流量。

### Q6: HTTPRoute 必須在 DNS 之前部署嗎？

**A:** 是的，非常重要！

**原因：**
- HTTPRoute 定義域名到後端服務的路由規則
- 如果 DNS 已指向但路由未配置，會造成 404 錯誤
- 先配置內部路由，再開放外部流量，更安全

**推薦順序：**
```
Gateway → Certificate → HTTPRoute → 檢查狀態 → DNS
```

## DNS-01 ACME 挑戰

### Q7: 什麼是 DNS-01 ACME 挑戰？

**A:** 一種通過 DNS TXT 記錄驗證域名所有權的方式。

**工作原理：**
1. cert-manager 向 Let's Encrypt 請求憑證
2. Let's Encrypt 返回 DNS-01 挑戰
3. cert-manager 調用 Cloudflare API 創建 TXT 記錄：`_acmeexample.invalid`
4. Let's Encrypt 查詢 TXT 記錄進行驗證
5. 驗證通過後簽發憑證
6. cert-manager 自動清理 TXT 記錄

**關鍵：整個過程不需要 DNS A 記錄！**

### Q8: DNS-01 和 HTTP-01 有什麼區別？

**A:** 兩種不同的域名驗證方式。

| 特性 | DNS-01 | HTTP-01 |
|------|--------|---------|
| **驗證方式** | DNS TXT 記錄 | HTTP `/.well-known/` |
| **需要 DNS A 記錄** | ❌ 不需要 | ✅ 必須先配置 |
| **需要 Gateway 可訪問** | ❌ 不需要 | ✅ 必須可訪問 |
| **驗證速度** | 1-5 分鐘 | 15-60 分鐘 |
| **支持萬用字元** | ✅ `*.example.invalid` | ❌ 不支持 |
| **部署順序靈活性** | ✅ 非常靈活 | ❌ 必須嚴格順序 |

**本專案使用 DNS-01 的原因：**
- 憑證可以在 DNS A 記錄配置前驗證
- 支持萬用字元憑證
- 驗證速度更快
- 部署順序更靈活

### Q9: 沒有部署 HTTPRoute 之前，Certificate 可以驗證嗎？

**A:** 可以！DNS-01 完全不需要 HTTPRoute。

**工作原理：**
- cert-manager 使用 DNS-01 ACME 挑戰驗證域名所有權
- Let's Encrypt 查詢 DNS TXT 記錄：`_acmeexample.invalid`
- cert-manager 自動調用 Cloudflare API 創建/刪除 TXT 記錄
- 整個過程不需要 HTTP 訪問

**因此：**
- ✅ Certificate 可以在 HTTPRoute 之前部署
- ✅ Certificate 可以在 DNS A 記錄配置前驗證
- ✅ HTTPRoute 應該在 DNS 之前部署（先配置內部路由）

### Q10: DNS-01 驗證需要 DNS A 記錄嗎？

**A:** 不需要！

**關鍵：DNS-01 只查詢 TXT 記錄，不查詢 A 記錄**

```bash
# Let's Encrypt 驗證時查詢
dig _acmeexample.invalid TXT

# 返回（由 cert-manager 創建）
_acmeexample.invalid. 300 IN TXT "random-token"

# Let's Encrypt 不會查詢 A 記錄！
# 因此不需要 example.invalid 的 A 記錄存在
```

## cert-manager

### Q11: 為什麼選擇 cert-manager 而不是 GCP ManagedCertificate？

**A:** cert-manager 更靈活、更快速。

**cert-manager 的優勢：**
- ✅ 支持 DNS-01 驗證（不需要 DNS A 記錄）
- ✅ 支持萬用字元憑證
- ✅ 驗證速度快（1-5 分鐘 vs 15-60 分鐘）
- ✅ 部署順序靈活
- ✅ 跨雲平台（不綁定 GCP）
- ✅ 開源社區支持

**GCP ManagedCertificate 的限制：**
- ❌ 僅支持 HTTP-01 驗證
- ❌ 必須先配置 DNS A 記錄
- ❌ 不支持萬用字元憑證
- ❌ 驗證速度慢
- ❌ 僅限 GKE

### Q12: Certificate 驗證需要多久？

**A:** 使用 DNS-01 通常 1-5 分鐘。

**詳細時間分解：**
1. DNS TXT 記錄創建：10-30 秒
2. Let's Encrypt 驗證：1-3 分鐘
3. 憑證簽發和存儲：10-30 秒

**監控驗證進度：**
```bash
# 查看 Certificate 狀態
kubectl get certificate -n staging --watch

# 查看詳細驗證流程
kubectl get certificaterequest,order,challenge -n staging
```

### Q13: 憑證會自動續期嗎？

**A:** 是的，cert-manager 會自動處理續期。

**自動續期機制：**
- cert-manager 會在憑證到期前 30 天自動續期
- 使用相同的 DNS-01 驗證流程
- 無需人工干預
- 新憑證自動更新到 Secret

**監控憑證到期時間：**
```bash
kubectl describe certificate -n staging | grep "Not After"
```

### Q14: 可以使用萬用字元憑證嗎？

**A:** 可以！DNS-01 支持萬用字元憑證。

**配置範例：**
```yaml
apiVersion: example.invalid/v1
kind: Certificate
metadata:
  name: wildcard-example-com
spec:
  secretName: wildcard-example-com-tls
  issuerRef:
    name: letsencrypt-dns01
    kind: ClusterIssuer
  dnsNames:
  - "*.example.invalid"     # 萬用字元：覆蓋所有子域名
  - "example.invalid"       # 根域名（萬用字元不包含根域名）
```

**優勢：**
- 一個憑證覆蓋多個子域名
- 減少憑證管理複雜度
- 減少 Let's Encrypt 請求次數

## Gateway

### Q15: Gateway 獲得 IP 需要多久？

**A:** 通常 3-5 分鐘，最多可能需要 10 分鐘。

**等待過程：**
1. GKE 創建 Load Balancer：1-2 分鐘
2. 分配外部 IP：1-2 分鐘
3. 配置健康檢查：1-2 分鐘

**監控進度：**
```bash
# 等待 Gateway 獲得 IP
php artisan deploy:gateway cluster-a staging

# 或手動監控
kubectl get gateway main-gateway -n staging --watch
```

### Q16: Gateway IP 會變化嗎？

**A:** 如果不保留靜態 IP，重新部署時 IP 可能會變。

**解決方案：使用靜態 IP**

```bash
# 1. 在 GCP 保留靜態 IP
gcloud compute addresses create my-static-ip \
  --global \
  --ip-version IPV4

# 2. 在 Gateway 配置中指定
apiVersion: example.invalid/v1
kind: Gateway
metadata:
  name: main-gateway
  annotations:
    example.invalid/address: "my-static-ip"
spec:
  ...
```

### Q17: 一個集群可以有多個 Gateway 嗎？

**A:** 可以，但通常不需要。

**建議：**
- 同一集群的多個服務共用一個 Gateway
- 通過 HTTPRoute 區分不同服務和域名
- 減少 IP 消耗和成本

**特殊情況可以使用多個 Gateway：**
- 不同的 TLS 配置需求
- 不同的網絡隔離需求
- 不同的安全策略

## HTTPRoute

### Q18: HTTPRoute 和 Ingress 有什麼區別？

**A:** HTTPRoute 是 Gateway API 的一部分，更現代、更靈活。

| 特性 | HTTPRoute (Gateway API) | Ingress |
|------|------------------------|---------|
| **標準化** | CNCF 標準 | Kubernetes 標準 |
| **功能** | 更豐富 | 基礎功能 |
| **路由能力** | 高級路由、權重分配 | 基本路由 |
| **擴展性** | 易於擴展 | 較難擴展 |
| **未來發展** | 活躍發展 | 維護模式 |

**本專案使用 HTTPRoute 的原因：**
- GKE Gateway 原生支持
- 更靈活的路由配置
- 更好的多集群支持

### Q19: HTTPRoute 如何選擇後端服務？

**A:** 通過 `backendRefs` 指定後端服務。

**範例：**
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
  rules:
  - matches:
    - path:
        type: PathPrefix
        value: /api
    backendRefs:
    - name: api-service  # Service 名稱
      port: 8080         # Service 端口
```

**支持的功能：**
- 單個後端服務
- 多個後端服務（權重分配）
- 跨 namespace 引用（需要額外配置）

## DNS 管理

### Q20: dns:apply 會自動檢查什麼？

**A:** 自動檢查所有前置條件。

**檢查項目：**
1. ✅ Gateway 是否有 IP 且 READY
2. ✅ Certificate 是否 READY: True
3. ✅ HTTPRoute 是否已配置

**如果檢查失敗：**
- 命令會終止並顯示錯誤
- 防止在資源未就緒時配置 DNS
- 避免服務異常

**使用範例：**
```bash
# 會自動檢查前置條件
php artisan dns:apply cluster-a staging

# 如果檢查失敗
❌ 前置條件檢查失敗：Certificate 未就緒
```

### Q21: 為什麼 DNS 要最後配置？

**A:** 確保所有內部配置完成後才開放外部流量。

**原因：**
- DNS 配置後，外部流量立即進來
- 如果 Gateway/Certificate/HTTPRoute 未就緒，會造成錯誤
- 先建設內部資源，再開放流量，更安全

**推薦順序：**
```
Gateway → Certificate → HTTPRoute → 檢查狀態 → DNS
```

### Q22: Cloudflare Zone ID 需要手動配置嗎？

**A:** 不需要，系統會自動查詢。

**智能查詢機制：**
系統會從完整域名開始，逐級向上查找，直到找到在 Cloudflare 中註冊的 Zone。

**範例：**
```
example.invalid
  ↓ 嘗試查詢 example.invalid
  ↓ 嘗試查詢 example.invalid
  ↓ 嘗試查詢 example.invalid
  ✓ 找到 example.invalid 的 Zone ID
```

**支持的場景：**
- 二級域名：`example.invalid` → `example.invalid`
- 三級域名：`example.invalid` → `example.invalid`
- 四級域名：`example.invalid` → `example.invalid`
- 特殊 TLD：`example.invalid.uk` → `example.invalid.uk`

## 多集群管理

### Q23: 如何添加新的集群？

**A:** 按照以下步驟。

**1. 配置集群信息**
```yaml
# config/clusters.yaml
cluster-d:
  name: "gke-eu-prod"
  project_id: "my-gcp-project-eu"
  region: "europe-west1"
  description: "歐洲生產集群"
```

**2. 創建目錄結構**
```bash
mkdir -p clusters/cluster-d/{staging,production}/{gateways,certificates,httproutes}
```

**3. 複製配置模板**
```bash
cp -r clusters/cluster-a/staging/gateways/ clusters/cluster-d/staging/
# 修改配置文件中的 cluster 標籤
```

**4. 測試部署**
```bash
php artisan deploy:k8s cluster-d staging
php artisan status:all cluster-d staging
```

### Q24: 如何批量部署到多個集群？

**A:** 使用 for 循環或 xargs。

**方式 1：順序部署**
```bash
for cluster in cluster-a cluster-b cluster-c; do
  php artisan deploy:k8s $cluster staging
  php artisan status:all $cluster staging
  php artisan dns:apply $cluster staging
done
```

**方式 2：並行部署（更快）**
```bash
echo "cluster-a cluster-b cluster-c" | xargs -n1 -P3 -I {} sh -c '
  php artisan deploy:k8s {} staging &&
  php artisan status:all {} staging &&
  php artisan dns:apply {} staging
'
```

### Q25: 多集群間如何保持配置一致？

**A:** 使用模板和標籤管理。

**建議做法：**

1. **使用統一的配置模板**
   ```bash
   # 為所有集群使用相同的 Gateway 配置模板
   # 只修改 cluster 標籤和域名
   ```

2. **統一標籤**
   ```yaml
   labels:
     cluster: cluster-a
     environment: production
     region: asia-east1
     managed-by: gke-gateway-route-manager
   ```

3. **定期審計**
   ```bash
   # 比較不同集群的配置
   diff clusters/cluster-a/prod/gateways/main-gateway.yaml \
        clusters/cluster-b/prod/gateways/main-gateway.yaml
   ```

## 安全性

### Q26: Cloudflare API Token 如何管理？

**A:** 分為兩部分管理。

**1. Laravel 應用配置（本地）**
```bash
# .env 文件
CLOUDFLARE_API_TOKEN=[REDACTED]

# 不納入版本控制
echo ".env" >> .gitignore
```

**2. Kubernetes Secret（集群內）**
```bash
# 為每個 namespace 創建 Secret
kubectl create secret generic cloudflare-api-token \
  --namespace=staging \
  --from-literal=api-token=[REDACTED]
```

**安全最佳實踐：**
- 最小權限原則（只授予 Zone.DNS Edit + Zone.Zone Read）
- 定期輪換 Token
- 不同環境使用不同 Token（可選）

### Q27: 如何確保部署安全？

**A:** 本專案設計了多層安全檢查。

**安全機制：**

1. **前置條件檢查**
   - `dns:apply` 自動檢查 Gateway/Certificate/HTTPRoute 狀態
   - 檢查失敗則終止操作

2. **手動部署**
   - 每個步驟需要手動執行
   - 可以隨時檢查和暫停

3. **GitOps**
   - 所有配置納入版本控制
   - 變更可追溯、可回滾
   - Code Review 流程

4. **分階段部署**
   - 先 staging 驗證
   - 再 production 部署

## 故障排查

### Q28: Certificate 一直未就緒怎麼辦？

**A:** 按以下步驟排查。

**1. 檢查 Certificate 狀態**
```bash
kubectl get certificate -n staging
kubectl describe certificate tls-cert -n staging
```

**2. 檢查 Challenge 狀態**
```bash
kubectl get challenge -n staging
kubectl describe challenge <name> -n staging
```

**3. 查看 cert-manager 日誌**
```bash
kubectl logs -n cert-manager deployment/cert-manager -f
```

**4. 驗證 Cloudflare API Token**
```bash
kubectl get secret cloudflare-api-token -n staging

# 測試 Token
CLOUDFLARE_TOKEN=[REDACTED] get secret cloudflare-api-token -n staging -o jsonpath='{.data.api-token}' | base64 -d)
curl -X GET "https://example.invalid/client/v4/user/tokens/verify" \
  -H "Authorization: Bearer $CLOUDFLARE_TOKEN"
```

**常見原因：**
- Cloudflare API Token 權限不足
- Let's Encrypt 速率限制
- 域名不在 Cloudflare 管理
- ClusterIssuer 未就緒

### Q29: HTTPS 訪問返回證書錯誤怎麼辦？

**A:** 檢查證書配置和引用。

**診斷步驟：**

```bash
# 1. 檢查 Certificate 狀態
kubectl get certificate -n staging

# 2. 檢查 Secret 是否存在
kubectl get secret tls-cert -n staging

# 3. 檢查 Gateway 引用
kubectl get gateway main-gateway -n staging -o yaml | grep -A 5 certificateRefs

# 4. 測試證書
openssl s_client -connect example.invalid:443 -servername example.invalid
```

**常見問題：**
- Secret 名稱不匹配
- Certificate 域名不包含訪問的域名
- Gateway 未正確引用 Secret

### Q30: 命令執行很慢怎麼辦？

**A:** 這些是正常等待時間。

**各步驟等待時間：**
- Gateway 獲得 IP：3-5 分鐘
- Certificate 驗證：1-5 分鐘
- DNS 傳播：1-5 分鐘

**加速技巧：**
- 使用合併部署（`deploy:k8s`）減少步驟
- 批量部署時使用並行執行
- 使用靜態 IP 避免 Gateway IP 重新分配

## 其他問題

### Q31: 如何回滾配置？

**A:** 使用 Git 回滾。

```bash
# 1. Git 回滾配置
git revert <commit-hash>
git push origin main

# 2. 重新部署
php artisan deploy:k8s cluster-a staging
php artisan status:all cluster-a staging

# DNS 通常不需要回滾（IP 未變）
```

### Q32: 如何刪除不再使用的服務？

**A:** 按照相反的順序刪除。

**刪除順序：**
```
DNS → HTTPRoute → Certificate → Gateway
```

**詳細步驟：**
```bash
# 1. 先刪除或更新 DNS（停止外部流量）
# 手動在 Cloudflare Dashboard 刪除 A 記錄

# 2. 等待 DNS 傳播（1-5 分鐘）

# 3. 刪除 HTTPRoute
kubectl delete httproute api-route -n staging

# 4. 如無其他服務使用，刪除 Certificate
kubectl delete certificate tls-cert -n staging

# 5. 如無其他服務使用，刪除 Gateway
kubectl delete gateway main-gateway -n staging
```

### Q33: 文檔在哪裡？

**A:** 所有文檔都在 `docs/` 目錄。

**文檔列表：**
- **[README.md](../README.md)** - 專案概述和快速開始
- **[getting-started.md](getting-started.md)** - 初次設置指南
- **[deployment-guide.md](deployment-guide.md)** - 部署指南
- **[commands-reference.md](commands-reference.md)** - 命令參考
- **[architecture.md](architecture.md)** - 架構設計
- **[troubleshooting.md](troubleshooting.md)** - 故障排除
- **[faq.md](faq.md)** - 本文檔

### Q34: 如何獲取幫助？

**A:** 多種方式。

**1. 查看文檔**
```bash
# 所有文檔在 docs/ 目錄
ls docs/
```

**2. 命令幫助**
```bash
# 查看所有命令
php artisan list

# 查看特定命令幫助
php artisan help deploy:k8s
```

**3. 聯繫 SRE 團隊**
- 提供詳細錯誤信息
- 附上相關日誌
- 說明已嘗試的解決方法

**4. 創建 Issue**
- 在 Git 倉庫創建 Issue
- 詳細描述問題
- 附上重現步驟

## 總結

關鍵概念總結：

1. **部署順序**：Gateway → Certificate → HTTPRoute → 檢查狀態 → DNS
2. **DNS-01 優勢**：不需要 DNS A 記錄就能驗證憑證
3. **安全閘門**：DNS 配置前自動檢查所有資源狀態
4. **手動控制**：所有關鍵操作手動執行，確保可控
5. **GitOps 友好**：所有配置納入版本控制

如果還有其他問題，歡迎查看其他文檔或聯繫 SRE 團隊！
