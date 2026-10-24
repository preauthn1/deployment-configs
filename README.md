# deployment-configs

20 个工程，共 629 个文本文件。

## 成员

- `pgs-kube` — `pgs/kube`（51 文件）
- `deployment-game-server` — `pgs/deployment/game-server`（32 文件）
- `base-modules` — `pgs/deployment/base-modules`（4 文件）
- `deployment-backend` — `pgs/deployment/backend`（72 文件）
- `pgs-hall` — `pgs/hall`（76 文件）
- `deployment-hall` — `pgs/deployment/hall`（30 文件）
- `marscard` — `pgs/deployment/marscard`（16 文件）
- `deployment-tgbot` — `pgs/deployment/tgbot`（14 文件）
- `deployment-gameicons` — `pgs/deployment/gameicons`（5 文件）
- `gs88` — `sre/gs88`（2 文件）
- `cm` — `sre/gff-cicd/cm`（64 文件）
- `bwg888` — `sre/bwg888`（17 文件）
- `gffv3` — `sre/gff-cicd/gffv3`（85 文件）
- `gffv1proxy` — `sre/gff-cicd/gffv1proxy`（4 文件）
- `gff-cicd-tgbot` — `sre/gff-cicd/tgbot`（26 文件）
- `gff-cicd-gffpay` — `sre/gff-cicd/gffpay`（27 文件）
- `gke-gateway-route-manager` — `sre/gke-gateway-route-manager`（12 文件）
- `sre-onegate` — `sre/onegate`（27 文件）
- `pgs-op_tickets` — `pgs/op_tickets`（1 文件）
- `dbc` — `sre/dbc`（64 文件）

## 脱敏（按各仓实际命中）

| 类型 | 命中 |
|---|---|
| 数据库口令 | 159 |
| Telegram bot token | 25 |
| 连接串 | 19 |
| 私钥 | 14 |
| AWS 访问密钥 | 5 |

处理最集中的工程：

- `sre/gff-cicd/gffv3` — 数据库口令 45、Telegram bot token 12、私钥 4
- `pgs/kube` — 数据库口令 24、连接串 19、私钥 1
- `pgs/deployment/backend` — 数据库口令 25、Telegram bot token 4
- `sre/dbc` — 数据库口令 26
- `sre/gff-cicd/cm` — 数据库口令 16、AWS 访问密钥 5、私钥 3
- `sre/gff-cicd/tgbot` — 数据库口令 10、私钥 2
- `sre/gff-cicd/gffpay` — 数据库口令 6、私钥 2、Telegram bot token 2
- `sre/onegate` — 数据库口令 3、私钥 2、Telegram bot token 1
- `pgs/deployment/tgbot` — Telegram bot token 5
- `pgs/deployment/game-server` — 数据库口令 2

## 说明

按仓库实际内容处理：命中的令牌、口令、私钥、云凭据、第三方 API key、内网地址、域名与邮箱均已替换或删除；二进制资源未收录。
