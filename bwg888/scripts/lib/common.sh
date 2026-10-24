#!/bin/bash

# 颜色定义
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# kubeconfig 路径
KUBECONFIG_PATH=""

# 函数定义
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 显示用法
show_usage() {
    echo "用法: $0 -k|--kubeconfig <kubeconfig路径> [-f|--force]"
    echo "选项:"
    echo "  -k, --kubeconfig    指定 kubeconfig 文件路径（必需）"
    echo "  -f, --force         强制重新安装所有组件"
    echo "  -h, --help          显示此帮助信息"
}

# 解析命令行参数
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -k|--kubeconfig)
                KUBECONFIG_PATH="$2"
                shift 2
                ;;
            -f|--force)
                FORCE_INSTALL=true
                shift
                ;;
            -h|--help)
                show_usage
                exit 0
                ;;
            *)
                log_error "未知参数: $1"
                show_usage
                exit 1
                ;;
        esac
    done

    # 检查是否提供了 kubeconfig
    if [ -z "${KUBECONFIG_PATH}" ]; then
        log_error "必须使用 -k 或 --kubeconfig 指定 kubeconfig 文件路径"
        show_usage
        exit 1
    fi

    # 检查 kubeconfig 文件是否存在
    if [ ! -f "${KUBECONFIG_PATH}" ]; then
        log_error "kubeconfig 文件不存在: ${KUBECONFIG_PATH}"
        exit 1
    fi
}

# 检查 kubectl 连接
check_kubectl() {
    log_info "使用 kubeconfig: ${KUBECONFIG_PATH}"
    log_info "检查 kubectl 连接..."
    if ! kubectl --kubeconfig="${KUBECONFIG_PATH}" get nodes &> /dev/null; then
        log_error "kubectl 无法连接到集群"
        exit 1
    fi
    log_info "kubectl 连接正常"
    
    # 显示集群信息
    log_info "集群信息:"
    kubectl --kubeconfig="${KUBECONFIG_PATH}" config current-context
    kubectl --kubeconfig="${KUBECONFIG_PATH}" cluster-info
} 