# bwg888

原始路径：`bwg888`　|　文件：**16**　|　主语言：YAML 13, Shell 3

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

## 主要文件

- `scripts/deploy.sh`
- `scripts/update-config.sh`
- `scripts/lib/common.sh`
- `.gitlab-ci.yml`
- `manifests/examples/example-app.yaml`
- `manifests/examples/example-ingress.yaml`
- `manifests/cert-manager/cluster-issuer.yaml`
- `environments/production/cert-manager.yaml`

## 说明

该工程为平台的一个组件。仅收录文本源码与配置，图片、音频等二进制资源未收录；凭据、密钥、内网地址与域名已按实际内容替换。

