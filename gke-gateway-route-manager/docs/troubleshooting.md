# 故障排除

本文檔涵蓋常見問題和解決方案。

## 目錄

- [Gateway 問題](#gateway-問題)
- [Certificate 問題](#certificate-問題)
- [HTTPRoute 問題](#httproute-問題)
- [DNS 問題](#dns-問題)
- [命令執行問題](#命令執行問題)
- [網絡連接問題](#網絡連接問題)
- [調試技巧](#調試技巧)

## Gateway 問題

### Gateway 未獲得 IP 地址

**症狀：**
```bash
kubectl get gateway -n staging
# NAME            CLASS                                     ADDRESS   READY
# main-gateway    gke-l7-global-external-managed                      False
```

**可能原因：**
1. GKE 集群未啟用 Gateway API
2. GatewayClass 不存在或配置錯誤
3. GCP 資源配額不足
4. 權限不足

**診斷步驟：**

```bash
# 1. 檢查 Gateway 詳細狀態
kubectl describe gateway main-gateway -n staging

# 2. 檢查 GatewayClass
kubectl get gatewayclass
kubectl describe gatewayclass gke-l7-global-external-managed

# 3. 檢查事件
kubectl get events -n staging --sort-by='.lastTimestamp' | grep gateway

# 4. 檢查 Gateway Controller 日誌
kubectl logs -n gke-system -l k8s-app=gateway-controller --tail=100
```

**解決方案：**

1. **啟用 Gateway API（如未啟用）**
   ```bash
   gcloud container clusters update CLUSTER_NAME \
     --gateway-api=standard \
     --region=REGION
   ```

2. **檢查資源配額**
   ```bash
   gcloud compute project-info describe --project=PROJECT_ID \
     | grep -A 10 quotas
   ```

3. **檢查服務帳號權限**
   ```bash
   # 確保有 compute.addresses.create 權限
   ```

4. **等待更長時間**
   - Gateway 獲得 IP 通常需要 3-5 分鐘
   - 最多可能需要 10 分鐘

### Gateway IP 頻繁變更

**症狀：**
每次重新部署 Gateway，IP 地址都會改變。

**原因：**
Gateway 沒有保留靜態 IP。

**解決方案：**

```yaml
# 在 Gateway 配置中指定靜態 IP
apiVersion: example.invalid/v1
kind: Gateway
metadata:
  name: main-gateway
  annotations:
    example.invalid/address: "MY_STATIC_IP_NAME"
spec:
  gatewayClassName: gke-l7-global-external-managed
  ...
```

先在 GCP 保留靜態 IP：
```bash
gcloud compute addresses create MY_STATIC_IP_NAME \
  --global \
  --ip-version IPV4
```

### Gateway 狀態一直是 Pending

**診斷：**
```bash
kubectl describe gateway main-gateway -n staging
```

查看 `Status.Conditions` 中的錯誤信息。

**常見錯誤：**

1. **"listener must have at least one certificateRef"**
   - Gateway 配置的 HTTPS listener 沒有引用憑證
   - 解決：添加 `certificateRefs` 或先使用 HTTP listener

2. **"backend not found"**
   - HTTPRoute 引用的後端服務不存在
   - 解決：先部署後端服務

## Certificate 問題

### Certificate 一直處於 Pending 狀態

**症狀：**
```bash
kubectl get certificate -n staging
# NAME                   READY   SECRET               AGE
# tls-cert    False   tls-cert   10m
```

**診斷步驟：**

```bash
# 1. 檢查 Certificate 詳細狀態
kubectl describe certificate tls-cert -n staging

# 2. 檢查 CertificateRequest
kubectl get certificaterequest -n staging
kubectl describe certificaterequest <name> -n staging

# 3. 檢查 Order
kubectl get order -n staging
kubectl describe order <name> -n staging

# 4. 檢查 Challenge
kubectl get challenge -n staging
kubectl describe challenge <name> -n staging

# 5. 查看 cert-manager 日誌
kubectl logs -n cert-manager deployment/cert-manager -f | grep tls-cert
```

**常見原因和解決方案：**

#### 1. Cloudflare API Token Secret 不存在

**錯誤信息：**
```
Error: secret "cloudflare-api-token" not found
```

**解決方案：**

**本專案使用 ClusterIssuer**，Secret 必須在 `cert-manager` namespace：

```bash
# 在 cert-manager namespace 創建 Secret
kubectl create secret generic cloudflare-api-token \
  --namespace=cert-manager \
  --from-literal=api-token=[REDACTED]

# 驗證 Secret
kubectl get secret cloudflare-api-token -n cert-manager
```

**注意：** 如果你的專案使用 Issuer（namespace 級別），Secret 需要在應用 namespace（如 `staging`）創建。詳見 [初次設置指南](getting-started.md#cloudflare-api-token-secret)。

#### 2. Cloudflare API Token 權限不足

**錯誤信息：**
```
Error: Cloudflare API error: Authentication error (Code: 9103)
```

**解決方案：**

1. 檢查 Token 權限：
   - 必須有 `Zone - DNS - Edit`
   - 必須有 `Zone - Zone - Read`

2. 測試 Token：
   ```bash
   CLOUDFLARE_TOKEN=[REDACTED]
   curl -X GET "https://example.invalid/client/v4/user/tokens/verify" \
     -H "Authorization: Bearer $CLOUDFLARE_TOKEN" \
     -H "Content-Type: application/json"
   ```

3. 重新創建 Token 並更新 Secret：
   ```bash
   # 本專案使用 ClusterIssuer，Secret 在 cert-manager namespace
   kubectl delete secret cloudflare-api-token -n cert-manager
   kubectl create secret generic cloudflare-api-token \
     --namespace=cert-manager \
     --from-literal=api-token=[REDACTED]
   ```

#### 3. Let's Encrypt 速率限制

**錯誤信息：**
```
Error: too many certificates (5) already issued for this exact set of domains in the last 168 hours
```

**解決方案：**

1. **測試環境使用 Staging Issuer**
   ```yaml
   spec:
     issuerRef:
       name: letsencrypt-dns01-staging  # 使用 staging
       kind: ClusterIssuer
   ```

2. **等待速率限制重置**
   - 每週最多 50 個憑證
   - 相同域名組合每週最多 5 個憑證

3. **查看速率限制詳情**
   - https://example.invalid/docs/rate-limits/

#### 4. DNS TXT 記錄創建失敗

**錯誤信息：**
```
Error: DNS record for this domain could not be created
```

**診斷：**
```bash
# 1. 檢查域名是否在 Cloudflare 管理
# 登入 Cloudflare Dashboard 確認

# 2. 檢查 Zone ID 是否正確
kubectl describe challenge <name> -n staging

# 3. 手動測試 TXT 記錄創建
curl -X POST "https://example.invalid/client/v4/zones/<ZONE_ID>/dns_records" \
  -H "Authorization: Bearer $CLOUDFLARE_TOKEN" \
  -H "Content-Type: application/json" \
  --data '{"type":"TXT","name":"_acme-challenge.test","content":"test"}'
```

#### 5. ClusterIssuer 未就緒

**診斷：**
```bash
kubectl get clusterissuer
# NAME                         READY   AGE
# letsencrypt-dns01            False   1h
```

**解決方案：**
```bash
kubectl describe clusterissuer letsencrypt-dns01

# 常見問題：
# - ACME server URL 錯誤
# - Email 格式錯誤
# - Solver 配置錯誤
```

### Certificate 驗證成功但 Gateway 仍顯示證書錯誤

**診斷：**

```bash
# 1. 檢查 Certificate 狀態
kubectl get certificate -n staging
# READY 應該是 True

# 2. 檢查 Secret 是否存在
kubectl get secret tls-cert -n staging

# 3. 檢查 Gateway 是否引用正確的 Secret
kubectl get gateway main-gateway -n staging -o yaml | grep -A 5 certificateRefs

# 4. 測試 HTTPS
curl -I https://example.invalid
```

**解決方案：**

1. **Secret 名稱不匹配**
   ```yaml
   # Certificate
   spec:
     secretName: tls-cert  # 這裡

   # Gateway
   spec:
     listeners:
     - tls:
         certificateRefs:
         - name: tls-cert  # 必須匹配
   ```

2. **Secret 和 Gateway 不在同一 namespace**
   - 確保在同一 namespace

3. **Gateway 需要重啟**
   ```bash
   # 刪除並重新創建 Gateway
   kubectl delete gateway main-gateway -n staging
   php artisan deploy:gateway cluster-a staging
   ```

## HTTPRoute 問題

### HTTPRoute 無法路由流量

**症狀：**
訪問域名返回 404 或 503 錯誤。

**診斷步驟：**

```bash
# 1. 檢查 HTTPRoute 狀態
kubectl get httproute -n staging
kubectl describe httproute api-route -n staging

# 2. 檢查後端服務
kubectl get svc -n staging
kubectl get pods -n staging

# 3. 測試後端服務
kubectl port-forward svc/api-service 8080:8080 -n staging
curl http://localhost:8080
```

**常見問題：**

#### 1. 後端服務不存在

**錯誤信息：**
```
Status.Parents[0].Conditions:
  Type: ResolvedRefs
  Status: False
  Reason: BackendNotFound
```

**解決方案：**
```bash
# 確認後端服務存在
kubectl get svc api-service -n staging

# 如果不存在，創建服務
kubectl apply -f backend-service.yaml
```

#### 2. parentRefs 配置錯誤

**錯誤信息：**
```
Status.Parents: null
```

**解決方案：**
```yaml
spec:
  parentRefs:
  - name: main-gateway  # 必須匹配 Gateway 名稱
    namespace: staging  # 如果在不同 namespace，需要指定
```

#### 3. hostname 不匹配

**問題：**
HTTPRoute 的 hostname 與訪問的域名不匹配。

**解決方案：**
```yaml
spec:
  hostnames:
  - "example.invalid"  # 必須完全匹配（包括子域名）
```

### 404 錯誤：找不到路由

**診斷：**

```bash
# 檢查路徑匹配規則
kubectl get httproute api-route -n staging -o yaml | grep -A 10 rules
```

**常見問題：**

```yaml
# 錯誤：PathPrefix 沒有包含實際請求路徑
rules:
- matches:
  - path:
      type: PathPrefix
      value: /api  # 只匹配 /api/*

# 訪問 / 會 404
# 解決：添加更多匹配規則或使用 /
rules:
- matches:
  - path:
      type: PathPrefix
      value: /  # 匹配所有路徑
```

## DNS 問題

### DNS 記錄未生效

**症狀：**
```bash
dig example.invalid +short
# 沒有返回或返回錯誤的 IP
```

**診斷步驟：**

```bash
# 1. 使用 Cloudflare DNS 伺服器查詢
dig @0.0.0.0 example.invalid +short

# 2. 查詢 DNS 記錄類型
dig example.invalid A +short
dig example.invalid AAAA +short
dig example.invalid CNAME +short

# 3. 檢查 TTL
dig example.invalid +nocmd +noall +answer

# 4. 使用驗證命令
php artisan dns:verify cluster-a staging
```

**常見原因：**

#### 1. DNS 尚未傳播

**解決方案：**
- DNS 傳播通常需要 1-5 分鐘
- 清除本地 DNS 緩存：
  ```bash
  # macOS
  sudo dscacheutil -flushcache
  sudo killall -HUP mDNSResponder

  # Linux
  sudo systemd-resolve --flush-caches

  # Windows
  ipconfig /flushdns
  ```

#### 2. DNS 記錄配置錯誤

**檢查：**
```bash
# 登入 Cloudflare Dashboard
# 檢查 DNS 記錄是否存在且正確
```

**手動修復：**
```bash
# 使用 dns:apply 重新應用
php artisan dns:apply cluster-a staging
```

#### 3. Cloudflare Proxy 干擾

**問題：**
啟用 Cloudflare Proxy（橙色雲朵）時，返回的 IP 是 Cloudflare 的 IP，不是 Gateway IP。

**這是正常的！**
- Cloudflare Proxy 會返回 Cloudflare 的 Anycast IP
- 流量會經過 Cloudflare CDN 到達 Gateway
- 只要能正常訪問就沒問題

**如果要看真實 IP：**
- 關閉 Proxy（灰色雲朵）
- 或直接查詢 Origin IP（內部記錄）

### dns:apply 命令失敗

**錯誤信息：**
```
❌ 前置條件檢查失敗：Gateway 沒有 IP
```

**原因：**
`dns:apply` 命令會自動檢查：
- Gateway 是否有 IP 且 READY
- Certificate 是否 READY: True
- HTTPRoute 是否已配置

**解決方案：**

```bash
# 1. 檢查狀態
php artisan status:all cluster-a staging

# 2. 修復未就緒的資源
# 如果 Gateway 沒有 IP
php artisan deploy:gateway cluster-a staging

# 如果 Certificate 未就緒
kubectl get certificate -n staging
kubectl describe certificate tls-cert -n staging

# 如果 HTTPRoute 未部署
php artisan deploy:httproute cluster-a staging

# 3. 再次嘗試
php artisan dns:apply cluster-a staging
```

## 命令執行問題

### kubectl: command not found

**解決方案：**

```bash
# 安裝 kubectl
# macOS
brew install kubectl

# Linux
curl -LO "https://example.invalid/release/$(curl -L -s https://example.invalid/release/stable.txt)/bin/linux/amd64/kubectl"
sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl
```

### gcloud: command not found

**解決方案：**

```bash
# 安裝 gcloud CLI
# macOS
brew install --cask google-cloud-sdk

# Linux
curl https://example.invalid | bash
exec -l $SHELL
```

### php artisan 命令失敗

**錯誤：**
```
Could not open input file: artisan
```

**解決方案：**

```bash
# 確保在項目根目錄
cd /path/to/gke-gateway-route-manager

# 檢查 artisan 文件是否存在
ls -la artisan
```

**錯誤：**
```
Class not found
```

**解決方案：**

```bash
# 重新安裝依賴
composer install

# 清除緩存
php artisan clear-compiled
composer dump-autoload
```

### 權限錯誤

**錯誤：**
```
Error: Insufficient Permission
```

**診斷：**

```bash
# 檢查當前 GCP 用戶
gcloud auth list

# 檢查 kubectl context
kubectl config current-context

# 檢查 RBAC 權限
kubectl auth can-i create gateway -n staging
```

**解決方案：**

```bash
# 重新認證
gcloud auth login

# 獲取集群憑證
gcloud container clusters get-credentials CLUSTER_NAME \
  --region=REGION \
  --project=PROJECT_ID
```

## 網絡連接問題

### HTTPS 連接失敗

**症狀：**
```bash
curl https://example.invalid
# curl: (60) SSL certificate problem
```

**診斷步驟：**

```bash
# 1. 測試 HTTP
curl -I http://example.invalid

# 2. 測試 HTTPS（忽略證書）
curl -I -k https://example.invalid

# 3. 檢查證書
openssl s_client -connect example.invalid:443 -servername example.invalid

# 4. 檢查證書詳細信息
echo | openssl s_client -connect example.invalid:443 2>/dev/null | openssl x509 -noout -dates -subject
```

**常見問題：**

#### 1. 證書域名不匹配

**錯誤：**
```
SSL: certificate subject name 'example.invalid' does not match target host name 'example.invalid'
```

**解決方案：**
```yaml
# 確保 Certificate 包含正確的域名
spec:
  dnsNames:
  - example.invalid  # 必須包含
```

#### 2. 證書過期

**解決方案：**
```bash
# 檢查證書到期時間
kubectl get certificate -n staging
kubectl describe certificate tls-cert -n staging

# cert-manager 會自動續期
# 如果沒有自動續期，手動觸發
kubectl delete certificaterequest -n staging --all
```

### 連接超時

**症狀：**
```bash
curl https://example.invalid
# curl: (28) Connection timed out
```

**診斷：**

```bash
# 1. 檢查 DNS
dig example.invalid +short

# 2. 檢查 Gateway IP
kubectl get gateway main-gateway -n staging

# 3. 測試 IP 連接
curl -I http://<GATEWAY_IP>

# 4. 檢查防火牆規則
gcloud compute firewall-rules list --project=PROJECT_ID
```

**常見原因：**
- DNS 未指向正確的 IP
- GCP 防火牆阻擋
- Gateway 未就緒
- 後端服務不健康

### 503 Service Unavailable

**診斷：**

```bash
# 1. 檢查後端服務
kubectl get pods -n staging
kubectl get svc -n staging

# 2. 檢查 Pod 健康狀態
kubectl describe pod <pod-name> -n staging

# 3. 查看 Pod 日誌
kubectl logs <pod-name> -n staging

# 4. 檢查 HTTPRoute
kubectl describe httproute api-route -n staging
```

**常見原因：**
- 後端 Pod 未運行
- Service selector 不匹配
- Pod 健康檢查失敗
- 端口配置錯誤

## 調試技巧

### 啟用詳細日誌

```bash
# Gateway Controller 日誌
kubectl logs -n gke-system -l k8s-app=gateway-controller --tail=100 -f

# cert-manager 日誌
kubectl logs -n cert-manager deployment/cert-manager -f

# 特定 Pod 日誌
kubectl logs <pod-name> -n staging --follow

# 前一個容器的日誌（如果重啟過）
kubectl logs <pod-name> -n staging --previous
```

### 使用 kubectl debug

```bash
# 創建 debug 容器
kubectl debug -it <pod-name> -n staging --image=busybox --target=<container-name>

# 在 Node 上運行 debug 容器
kubectl debug node/<node-name> -it --image=ubuntu

# 複製 Pod 進行調試
kubectl debug <pod-name> -n staging --copy-to=<pod-name>-debug
```

### 測試網絡連接

```bash
# 從 Pod 內測試後端服務
kubectl run tmp-shell --rm -i --tty --image nicolaka/netshoot -n staging

# 在 Pod 內執行
curl http://api-service:8080
nslookup api-service
ping api-service
```

### 檢查資源配額

```bash
# 查看 namespace 資源使用
kubectl top pods -n staging
kubectl top nodes

# 查看資源限制
kubectl describe resourcequota -n staging
kubectl describe limitrange -n staging
```

### 導出配置進行比較

```bash
# 導出當前配置
kubectl get gateway main-gateway -n staging -o yaml > current-gateway.yaml
kubectl get certificate tls-cert -n staging -o yaml > current-cert.yaml
kubectl get httproute api-route -n staging -o yaml > current-route.yaml

# 與文件配置比較
diff current-gateway.yaml clusters/cluster-a/staging/gateways/main-gateway.yaml
```

## 獲取幫助

如果以上方法都無法解決問題：

1. **查看日誌**
   - Gateway Controller 日誌
   - cert-manager 日誌
   - 後端服務日誌

2. **檢查官方文檔**
   - [GKE Gateway API](https://example.invalid/kubernetes-engine/docs/concepts/gateway-api)
   - [cert-manager](https://example.invalid/docs/)
   - [Cloudflare API](https://example.invalid/api/)

3. **聯繫 SRE 團隊**
   - 提供詳細的錯誤信息
   - 附上相關日誌
   - 說明已嘗試的解決方法

4. **創建 Issue**
   - 在項目 Git 倉庫創建 Issue
   - 詳細描述問題
   - 附上重現步驟

## 下一步

- 查看 [FAQ](faq.md) 常見問題解答
- 查看 [命令參考](commands-reference.md) 了解工具使用
- 查看 [架構設計](architecture.md) 深入理解系統
