# target Kubernetes 基础设施

本仓库管理 target Kubernetes 集群的基础设施配置。

## 组件清单

- cert-manager：证书管理
- nginx-ingress：入口控制器
- load-balancer：负载均衡

## 目录结构

target-infra/
├── charts/ # Helm charts 配置
├── manifests/ # Kubernetes 原生配置文件
└── environments/ # 环境特定配置
├── production/ # 生产环境配置
└── staging/ # 测试环境配置

## 部署说明

待补充

## 维护指南

待补充