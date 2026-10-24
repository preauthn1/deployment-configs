#!/bin/bash

# 导入共用库
source "$(dirname "$0")/lib/common.sh"

# 检查组件是否已安装
check_installation() {
    local component=$1
    local namespace=$2
    local label=$3
    
    log_info "检查 ${component} 是否已安装..."
    
    # 检查命名空间是否存在
    if kubectl --kubeconfig="${KUBECONFIG_PATH}" get namespace "${namespace}" &> /dev/null; then
        # 检查 Pod 是否运行
        if kubectl --kubeconfig="${KUBECONFIG_PATH}" get pods -n "${namespace}" -l "${label}" &> /dev/null; then
            log_info "${component} 已安装"
            return 0
        fi
    fi
    
    log_info "${component} 未安装"
    return 1
}

# 安装前检查
pre_install_check() {
    local force=${1:-false}
    
    if [ "$force" != "true" ]; then
        # 检查各组件
        if check_installation "cert-manager" "cert-manager" "example.invalid/instance=cert-manager"; then
            log_warn "cert-manager 已安装，跳过安装"
            SKIP_CERT_MANAGER=true
        fi
        
        if check_installation "metallb" "metallb-system" "example.invalid/name=metallb"; then
            log_warn "MetalLB 已安装，跳过安装"
            SKIP_METALLB=true
        fi
        
        if check_installation "nginx-ingress" "ingress-nginx" "example.invalid/component=controller"; then
            log_warn "nginx-ingress 已安装，跳过安装"
            SKIP_NGINX_INGRESS=true
        fi
        
        if check_installation "reloader" "reloader" "example.invalid/name=reloader"; then
            log_warn "reloader 已安装，跳过安装"
            SKIP_RELOADER=true
        fi
        
        if check_installation "metrics-server" "kube-system" "k8s-app=metrics-server"; then
            log_warn "Metrics Server 已安装，跳过安装"
            SKIP_METRICS_SERVER=true
        fi
        
        # 如果所有组件都已安装，退出
        if [ "$SKIP_CERT_MANAGER" = "true" ] && \
           [ "$SKIP_METALLB" = "true" ] && \
           [ "$SKIP_NGINX_INGRESS" = "true" ] && \
           [ "$SKIP_RELOADER" = "true" ] && \
           [ "$SKIP_METRICS_SERVER" = "true" ]; then
            log_warn "所有组件都已安装，如需重新安装请使用 -f 参数"
            exit 0
        fi
    fi
}

# 安装 cert-manager
install_cert_manager() {
    log_info "开始安装 cert-manager..."
    
    # 创建命名空间
    log_info "创建 cert-manager 命名空间..."
    kubectl --kubeconfig="${KUBECONFIG_PATH}" create namespace cert-manager --dry-run=client -o yaml | kubectl --kubeconfig="${KUBECONFIG_PATH}" apply -f -
    
    # 添加 Helm 仓库
    log_info "添加 Helm 仓库..."
    helm repo add jetstack https://example.invalid
    helm repo update
    
    # 安装 cert-manager
    log_info "安装 cert-manager..."
    helm --kubeconfig="${KUBECONFIG_PATH}" upgrade --install cert-manager jetstack/cert-manager \
        --namespace cert-manager \
        --create-namespace \
        -f charts/cert-manager/values.yaml
    
    # 等待 Pod 就绪
    log_info "等待 cert-manager Pod 就绪..."
    kubectl --kubeconfig="${KUBECONFIG_PATH}" wait --for=condition=ready pod -l example.invalid/instance=cert-manager -n cert-manager --timeout=300s
}

# 配置证书
setup_certificate() {
    log_info "开始配置证书..."
    
    # 部署 Cloudflare Secret
    log_info "部署 Cloudflare Secret..."
    kubectl --kubeconfig="${KUBECONFIG_PATH}" apply -f manifests/cert-manager/cloudflare-secret.yaml
    
    # 部署 ClusterIssuer
    log_info "部署 ClusterIssuer..."
    kubectl --kubeconfig="${KUBECONFIG_PATH}" apply -f manifests/cert-manager/cluster-issuer.yaml
    
    # 部署证书
    log_info "部署证书..."
    kubectl --kubeconfig="${KUBECONFIG_PATH}" apply -f manifests/cert-manager/wildcard-certificate.yaml
    
    # 等待证书就绪
    log_info "等待证书就绪..."
    kubectl --kubeconfig="${KUBECONFIG_PATH}" wait --for=condition=Ready certificate/wildcard-target-com -n cert-manager --timeout=300s
}

# 检查证书状态
check_certificate() {
    log_info "检查证书状态..."
    # kubectl get certificate -n cert-manager   
    kubectl --kubeconfig="${KUBECONFIG_PATH}" get certificate -n cert-manager
    # kubectl get certificaterequest -n cert-manager
    kubectl --kubeconfig="${KUBECONFIG_PATH}" get certificaterequest -n cert-manager
    # kubectl get order -n cert-manager
    kubectl --kubeconfig="${KUBECONFIG_PATH}" get order -n cert-manager
}

# 安装 MetalLB
install_metallb() {
    log_info "开始安装 MetalLB..."
    
    # 创建命名空间
    log_info "创建 metallb-system 命名空间..."
    kubectl --kubeconfig="${KUBECONFIG_PATH}" create namespace metallb-system --dry-run=client -o yaml | kubectl --kubeconfig="${KUBECONFIG_PATH}" apply -f -
    
    # 创建 memberlist secret
    log_info "创建 memberlist secret..."
    kubectl --kubeconfig="${KUBECONFIG_PATH}" create secret generic -n metallb-system metallb-memberlist --from-literal=secretkey="$(openssl rand -base64 128)" --dry-run=client -o yaml | kubectl --kubeconfig="${KUBECONFIG_PATH}" apply -f -
    
    # 添加 Helm 仓库
    log_info "添加 MetalLB Helm 仓库..."
    helm repo add metallb https://example.invalid/metallb
    helm repo update
    
    # 安装 MetalLB
    log_info "安装 MetalLB..."
    helm --kubeconfig="${KUBECONFIG_PATH}" upgrade --install metallb metallb/metallb \
        --namespace metallb-system \
        --create-namespace \
        --version 0.13.12 \
        -f charts/metallb/values.yaml \
        --wait
    
    # 等待 CRDs 就绪
    log_info "等待 CRDs 就绪..."
    sleep 10
    
    # 应用 MetalLB 配置
    log_info "应用 MetalLB 配置..."
    kubectl --kubeconfig="${KUBECONFIG_PATH}" apply -f manifests/metallb/config.yaml
}

# 安装 nginx-ingress
install_nginx_ingress() {
    log_info "开始安装 nginx-ingress..."
    
    # 添加 Helm 仓库
    log_info "添加 nginx-ingress Helm 仓库..."
    helm repo add ingress-nginx https://example.invalid/ingress-nginx
    helm repo update
    
    # 安装 nginx-ingress
    log_info "安装 nginx-ingress..."
    helm --kubeconfig="${KUBECONFIG_PATH}" upgrade --install ingress-nginx ingress-nginx/ingress-nginx \
        --namespace ingress-nginx \
        --create-namespace \
        --version 4.7.1 \
        -f charts/nginx-ingress/values.yaml \
        --wait
    
    # 等待 Pod 就绪
    log_info "等待 nginx-ingress Pod 就绪..."
    kubectl --kubeconfig="${KUBECONFIG_PATH}" wait --for=condition=ready pod -l example.invalid/component=controller -n ingress-nginx --timeout=300s
}

# 安装 reloader
install_reloader() {
    log_info "开始安装 reloader..."
    
    # 检查是否已安装
    if check_installation "reloader" "reloader" "example.invalid/name=reloader"; then
        log_warn "reloader 已安装，跳过安装"
        SKIP_RELOADER=true
        return
    fi
    
    # 添加 Helm 仓库
    log_info "添加 stakater Helm 仓库..."
    helm repo add stakater https://example.invalid/stakater-charts
    helm repo update
    
    # 安装 reloader
    log_info "安装 reloader..."
    helm --kubeconfig="${KUBECONFIG_PATH}" upgrade --install reloader stakater/reloader \
        --namespace reloader \
        --create-namespace \
        --wait
    
    # 等待 Pod 就绪
    log_info "等待 reloader Pod 就绪..."
    kubectl --kubeconfig="${KUBECONFIG_PATH}" wait --for=condition=ready pod -l example.invalid/name=reloader -n reloader --timeout=300s
}

# 安装 Metrics Server
install_metrics_server() {
    log_info "开始安装 Metrics Server..."
    
    # 安装 Metrics Server
    log_info "安装 Metrics Server..."
    kubectl --kubeconfig="${KUBECONFIG_PATH}" apply -f https://example.invalid/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
    
    # 等待 Pod 就绪
    log_info "等待 Metrics Server Pod 就绪..."
    kubectl --kubeconfig="${KUBECONFIG_PATH}" wait --for=condition=ready pod -l k8s-app=metrics-server -n kube-system --timeout=300s
    
    # 修改 Metrics Server 配置以允许跳过 TLS 验证
    log_info "配置 Metrics Server..."
    kubectl --kubeconfig="${KUBECONFIG_PATH}" patch deployment metrics-server -n kube-system --type=json -p '[{"op": "add", "path": "/spec/template/spec/containers/0/args/-", "value": "--kubelet-insecure-tls"}]'
    
    # 等待 Pod 重新就绪
    log_info "等待 Metrics Server Pod 重新就绪..."
    kubectl --kubeconfig="${KUBECONFIG_PATH}" rollout status deployment/metrics-server -n kube-system --timeout=300s
    kubectl --kubeconfig="${KUBECONFIG_PATH}" wait --for=condition=ready pod -l k8s-app=metrics-server -n kube-system --timeout=300s
}

# 主流程
main() {
    # 解析命令行参数
    parse_args "$@"
    
    log_info "开始部署流程..."
    
    # 检查 kubectl 连接
    check_kubectl
    
    # 检查已安装组件
    pre_install_check "${FORCE_INSTALL}"
    
    # 安装 Metrics Server
    if [ "$SKIP_METRICS_SERVER" != "true" ]; then
        if [ -n "$CI" ]; then
            install_metrics_server
        else
            read -p "是否继续安装 Metrics Server? (y/n) " -n 1 -r
            echo
            if [[ $REPLY =~ ^[Yy]$ ]]; then
                install_metrics_server
            fi
        fi
    fi
    
    # 安装 cert-manager
    if [ "$SKIP_CERT_MANAGER" != "true" ]; then
        if [ -n "$CI" ]; then
            install_cert_manager
        else
            read -p "是否继续安装 cert-manager? (y/n) " -n 1 -r
            echo
            if [[ $REPLY =~ ^[Yy]$ ]]; then
                install_cert_manager
            fi
        fi
    fi
    
    # 配置证书
    if [ "$SKIP_CERT_MANAGER" != "true" ]; then
        if [ -n "$CI" ]; then
            setup_certificate
            check_certificate
        else
            read -p "是否继续配置证书? (y/n) " -n 1 -r
            echo
            if [[ $REPLY =~ ^[Yy]$ ]]; then
                setup_certificate
                check_certificate
            fi
        fi
    fi
    
    # 安装 MetalLB
    if [ "$SKIP_METALLB" != "true" ]; then
        if [ -n "$CI" ]; then
            install_metallb
        else
            read -p "是否安装 MetalLB? (y/n) " -n 1 -r
            echo
            if [[ $REPLY =~ ^[Yy]$ ]]; then
                install_metallb
            fi
        fi
    fi
    
    # 安装 nginx-ingress
    if [ "$SKIP_NGINX_INGRESS" != "true" ]; then
        if [ -n "$CI" ]; then
            install_nginx_ingress
        else
            read -p "是否安装 nginx-ingress? (y/n) " -n 1 -r
            echo
            if [[ $REPLY =~ ^[Yy]$ ]]; then
                install_nginx_ingress
            fi
        fi
    fi
    
    # 安装 reloader
    if [ "$SKIP_RELOADER" != "true" ]; then
        if [ -n "$CI" ]; then
            install_reloader
        else
            read -p "是否安装 reloader? (y/n) " -n 1 -r
            echo
            if [[ $REPLY =~ ^[Yy]$ ]]; then
                install_reloader
            fi
        fi
    fi
    
    log_info "部署完成"
}

# 执行主流程
main "$@" 