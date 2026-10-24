# bwg888

> 本目录原始 README 见文末，本节为快照补充信息。

原始路径：`bwg888`　|　文件 **16**　|　主语言：YAML 13, Shell 3

## 目录构成

```
manifests/cert-manager/  — 3
manifests/examples/  — 2
scripts/  — 2
manifests/registry/  — 1
manifests/nginx-ingress/  — 1
manifests/metallb/  — 1
environments/production/  — 1
scripts/lib/  — 1
charts/cert-manager/  — 1
charts/nginx-ingress/  — 1
charts/metallb/  — 1
```

## 关键文件

- CI/构建：`.gitlab-ci.yml`

---

## 原始说明

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

