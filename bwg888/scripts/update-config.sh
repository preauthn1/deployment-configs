#!/bin/bash

# 导入共用库
source "$(dirname "$0")/lib/common.sh"

# 更新 cert-manager 配置
update_cert_manager_config() {
    log_info "更新 cert-manager 配置..."
    kubectl --kubeconfig="${KUBECONFIG_PATH}" apply -f manifests/cert-manager/cloudflare-secret.yaml
    kubectl --kubeconfig="${KUBECONFIG_PATH}" apply -f manifests/cert-manager/cluster-issuer.yaml
    kubectl --kubeconfig="${KUBECONFIG_PATH}" apply -f manifests/cert-manager/wildcard-certificate.yaml
}

# 更新 metallb 配置
update_metallb_config() {
    log_info "更新 metallb 配置..."
    kubectl --kubeconfig="${KUBECONFIG_PATH}" apply -f manifests/metallb/config.yaml
}

# 更新 nginx-ingress 配置
update_nginx_ingress_config() {
    log_info "更新 nginx-ingress 配置..."
    
    # 创建 custom headers configmap
    kubectl --kubeconfig="${KUBECONFIG_PATH}" apply -f manifests/nginx-ingress/nginx-ingress-custom-proxy-headers.yaml
    
    # 更新主配置
    kubectl --kubeconfig="${KUBECONFIG_PATH}" patch configmap ingress-nginx-controller -n ingress-nginx --patch '{
        "data": {
            "allow-snippet-annotations": "true",
            "use-forwarded-headers": "true",
            "allow-backend-server-header": "true",
            "disable-access-log": "true",
            "enable-underscores-in-headers": "true",
            "generate-request-id": "true",
            "ignore-invalid-headers": "false",
            "keep-alive": "75",
            "keep-alive-requests": "1000",
            "max-worker-connections": "65536",
            "proxy-body-size": "80m",
            "proxy-connect-timeout": "10",
            "proxy-set-headers": "ingress-nginx/nginx-ingress-custom-proxy-headers",
            "reuse-port": "true",
            "server-tokens": "false",
            "ssl-ciphers": "ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384:ECDHE-ECDSA-CHACHA20-POLY1305:ECDHE-RSA-CHACHA20-POLY1305:DHE-RSA-AES128-GCM-SHA256:DHE-RSA-AES256-GCM-SHA384:DHE-RSA-CHACHA20-POLY1305:ECDHE-ECDSA-AES128-SHA256:ECDHE-RSA-AES128-SHA256:ECDHE-ECDSA-AES128-SHA:ECDHE-RSA-AES128-SHA:ECDHE-ECDSA-AES256-SHA384:ECDHE-RSA-AES256-SHA384:ECDHE-ECDSA-AES256-SHA:ECDHE-RSA-AES256-SHA:DHE-RSA-AES128-SHA256:DHE-RSA-AES256-SHA256:AES128-GCM-SHA256:AES256-GCM-SHA384:AES128-SHA256:AES256-SHA256:AES128-SHA:AES256-SHA:DES-CBC3-SHA",
            "ssl-protocols": "TLSv1 TLSv1.1 TLSv1.2",
            "ssl-redirect": "false",
            "upstream-keepalive-connections": "1000",
            "upstream-keepalive-requests": "10000",
            "upstream-keepalive-timeout": "60",
            "worker-cpu-affinity": "auto"
        }
    }'
}

# 更新 registry secret
update_registry_secret() {
    log_info "更新 registry secret..."
    kubectl --kubeconfig="${KUBECONFIG_PATH}" create namespace bwg --dry-run=client -o yaml | kubectl --kubeconfig="${KUBECONFIG_PATH}" apply -f -
    kubectl --kubeconfig="${KUBECONFIG_PATH}" apply -f manifests/registry/regcred.yaml
}

# 检查配置状态
check_status() {
    log_info "检查配置状态..."
    
    # 检查证书状态
    log_info "证书状态:"
    kubectl --kubeconfig="${KUBECONFIG_PATH}" get certificate -n cert-manager
    
    # 检查 metallb 状态
    log_info "MetalLB 配置状态:"
    kubectl --kubeconfig="${KUBECONFIG_PATH}" get example.invalid -n metallb-system
    
    # 检查 nginx-ingress 状态
    log_info "Nginx Ingress 配置状态:"
    kubectl --kubeconfig="${KUBECONFIG_PATH}" get configmap ingress-nginx-controller -n ingress-nginx
    kubectl --kubeconfig="${KUBECONFIG_PATH}" get configmap nginx-ingress-custom-proxy-headers -n ingress-nginx
}

# 主流程
main() {
    # 解析命令行参数
    parse_args "$@"
    
    # 检查 kubectl 连接
    check_kubectl
    
    # 更新各组件配置
    update_cert_manager_config
    update_metallb_config
    update_nginx_ingress_config
    update_registry_secret
    
    # 检查状态
    check_status
}

# 执行主流程
main "$@" 