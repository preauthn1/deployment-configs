# sre-onegate

原始路径：`sre-onegate`　|　文件：**22**　|　主语言：YAML 18

## 目录构成

```
helm-charts/laravel-app/  — 14
helm-charts/laravel-migration/  — 6
```

## 关键文件

- CI/构建：`Makefile`、`.gitlab-ci.yml`

## 主要文件

- `helm-charts/laravel-app/templates/deployment.yaml`
- `helm-charts/laravel-migration/templates/migration-job.yaml`
- `helm-charts/laravel-app/templates/cronjob.yaml`
- `helm-charts/laravel-app/templates/deployment-queue-work.yaml`
- `helm-charts/laravel-app/.env.tpl`
- `helm-charts/laravel-app/templates/NOTES.txt`
- `Makefile`
- `helm-charts/laravel-app/values_dev.yaml`

## 说明

该工程为平台的一个组件。仅收录文本源码与配置，图片、音频等二进制资源未收录；凭据、密钥、内网地址与域名已按实际内容替换。

