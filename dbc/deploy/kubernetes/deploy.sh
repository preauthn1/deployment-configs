#!/bin/bash
# 部署 dbc-gateway 到目前 kubectl context 指向的叢集。
#
# 用法：
#   ./deploy.sh          # 用最新的 git release tag
#   ./deploy.sh v0.0.31  # 指定版本（例如回滾）
#
# 只處理 dbc-gateway.yaml 裡的資源（Namespace/ServiceAccount/Deployment/
# Service/PodDisruptionBudget，image tag 由本腳本替換）。三個 Secret
# （dbc-gateway-tls／dbc-gateway-google-credentials／dbc-gateway-config）
# 不在這個腳本管轄範圍內，變動頻率跟 release 不同，要另外手動處理。
#
# 之後要接 CI 自動部署，呼叫方式預期是：
#   ./deploy.sh "$CI_COMMIT_TAG"
# 只需要另外準備 KUBECONFIG（CI/CD 變數，比照 GCP_ARTIFACT_REGISTRY_KEY
# 的 base64 + masked/protected 處理方式）跟一個裝了 kubectl 的 job image，
# 這個腳本本身不用改。
set -euo pipefail

# 明確拒絕不認得的參數/旗標，不要靜默忽略——曾經因為多傳了一個
# 誤以為會被處理的 --dry-run=client，實際上被這裡忽略，變成真的執行了
# kubectl apply，而不是預期中的 dry-run。
if [ "$#" -gt 1 ]; then
	echo "用法：$0 [TAG]（不接受其他參數/旗標）" >&2
	exit 1
fi

TAG=${1:-$(git describe --tags --abbrev=0)}
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "部署 dbc-gateway:${TAG} 到目前 kubectl context（$(kubectl config current-context)）"
sed "s#dbc-gateway:TAG#dbc-gateway:${TAG}#" "${SCRIPT_DIR}/dbc-gateway.yaml" | kubectl apply -f -
