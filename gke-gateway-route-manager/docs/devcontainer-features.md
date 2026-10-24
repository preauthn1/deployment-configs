# DevContainer Features 配置說明

本文檔說明 GKE Gateway Route Manager 專案使用的 DevContainer Features 配置。

## Common-Utils Feature

### 配置概覽

```json
"example.invalid/devcontainers/features/common-utils:2": {
  "installZsh": true,
  "installOhMyZsh": false,
  "upgradePackages": false,
  "username": "devuser",
  "uid": "1000",
  "gid": "1000",
  "configureZshAsDefaultShell": false
}
```

### 為什麼需要這個 Feature？

**核心問題：Docker 容器的檔案權限問題**

當在 Docker 容器中開發時，容器內創建的檔案在主機上可能無法編輯，反之亦然。這是因為 UID/GID 不匹配：

```bash
# 容器使用 root (UID: 0)
root@container$ touch file.txt
# 主機上查看（主機使用者 UID: 1000）
$ ls -l file.txt
-rw-r--r-- 1 root root 0 file.txt  # ❌ 主機使用者無法編輯
```

**common-utils 的解決方案：** 在容器內創建與主機 UID/GID 匹配的使用者。

### 配置參數說明

#### 1. username: "devuser"

創建名為 `devuser` 的非 root 使用者，該使用者：
- 自動加入 `sudo` 群組
- 可執行需要權限的命令
- 作為容器的預設使用者（配合 `"remoteUser": "devuser"`）

#### 2. uid: "1000" 和 gid: "1000"

**這是最關鍵的配置！** 設定容器使用者的 UID/GID 與主機使用者匹配。

**Linux/macOS 上的使用者通常是 UID 1000：**
```bash
# 查詢主機 UID
$ id
uid=1000(user) gid=1000(user) ...
```

**UID/GID 匹配後的效果：**
```bash
# 在容器內（devuser UID: 1000）
devuser@container$ touch file.txt
devuser@container$ ls -l file.txt
-rw-r--r-- 1 devuser devuser 0 file.txt

# 在主機上（user UID: 1000）
$ ls -l file.txt
-rw-r--r-- 1 user user 0 file.txt  # ✅ 可以無縫編輯
```

**掛載目錄的配置也要對應：**
```json
// 掛載到 devuser 的 home 目錄（而非 /root）
"mounts": [
  "source=${localEnv:HOME}/.kube,target=/home/devuser/.kube,type=bind"
]
```

#### 3. installZsh: true

安裝 Zsh shell，提供更好的命令列體驗（自動補全、語法高亮等）。

**為什麼不啟用為預設 shell？**
```json
"configureZshAsDefaultShell": false
```
- 保持 Bash 作為預設（自動化腳本更通用）
- 開發者可手動執行 `zsh` 切換

#### 4. installOhMyZsh: false

不安裝 Oh My Zsh 框架，原因：
- 保持容器映像輕量
- 減少建置時間
- 避免不必要的依賴

如需使用，可手動安裝：
```bash
sh -c "$(curl -fsSL https://example.invalid/ohmyzsh/ohmyzsh/master/tools/install.sh)"
```

#### 5. upgradePackages: false

不自動升級系統套件，確保：
- 開發環境一致性
- 避免意外的套件版本變更
- 可預測的建置結果

## 掛載目錄權限配置

配合 common-utils，本專案掛載以下主機目錄：

```json
"mounts": [
  // GCP 憑證（target 必須是 /home/devuser）
  "source=${localEnv:HOME}/.config/gcloud,target=/home/devuser/.config/gcloud,type=bind",

  // kubectl 配置
  "source=${localEnv:HOME}/.kube,target=/home/devuser/.kube,type=bind",

  // SSH 密鑰
  "source=${localEnv:HOME}/.ssh,target=/home/devuser/.ssh,type=bind",

  // Claude MCP 配置
  "source=${localEnv:HOME}/.config/claude,target=/home/devuser/.config/claude,type=bind"
]
```

**關鍵點：**
- ✅ 掛載到 `/home/devuser`（devuser 的 home）
- ❌ 不要掛載到 `/root`（僅 root 使用者可訪問）

## Dockerfile 配合設定

Dockerfile 需要確保工作目錄權限匹配：

```dockerfile
# 設定工作目錄並確保權限正確
WORKDIR /workspace
RUN chown -R 1000:1000 /workspace  # ← 與 devuser UID/GID 匹配
```

## 驗證配置

啟動 devcontainer 後，執行以下命令驗證：

```bash
# 1. 檢查當前使用者
$ whoami
devuser

# 2. 檢查 UID/GID
$ id
uid=1000(devuser) gid=1000(devuser) groups=1000(devuser),27(sudo)

# 3. 檢查工作目錄權限
$ ls -ld /workspace
drwxr-xr-x 1 devuser devuser 4096 /workspace

# 4. 測試檔案創建
$ touch /workspace/test.txt
$ ls -l test.txt
-rw-r--r-- 1 devuser devuser 0 test.txt  # ✅

# 5. 測試掛載目錄訪問
$ ls -l ~/.kube/config
-rw------- 1 devuser devuser ... config  # ✅

# 6. 測試 sudo 權限
$ sudo -v  # 應該可以使用 sudo
```

## 常見問題

### Q: 為什麼選擇 UID 1000？

**A:** Linux/macOS 系統通常將第一個使用者的 UID 設為 1000。大多數開發者的主機 UID 都是 1000。

**檢查你的 UID：**
```bash
$ id -u
1000
```

如果不是 1000，需調整配置：
```json
"uid": "你的UID",
"gid": "你的GID"
```

### Q: 為什麼掛載目錄無法訪問？

**A:** 檢查掛載路徑是否正確：

```json
// ❌ 錯誤：掛載到 /root（root 使用者的 home）
"source=${localEnv:HOME}/.kube,target=/root/.kube,type=bind"

// ✅ 正確：掛載到 /home/devuser（devuser 的 home）
"source=${localEnv:HOME}/.kube,target=/home/devuser/.kube,type=bind"
```

### Q: 為什麼檔案擁有者顯示數字而非名稱？

**症狀：**
```bash
$ ls -l
-rw-r--r-- 1 1001 1001 0 file.txt  # ← 顯示數字
```

**原因：** 容器內沒有 UID 1001 對應的使用者名稱。

**解決：** 調整容器 UID 匹配主機：
```json
"uid": "1001",
"gid": "1001"
```

## Git Feature

```json
"example.invalid/devcontainers/features/git:1": {
  "version": "latest"
}
```

**功能：** 安裝最新版本的 Git。

**用途：**
- 版本控制操作
- GitOps 工作流程
- 存取遠端倉庫（透過掛載的 SSH 密鑰）

## 參考資源

- **Common-Utils 官方文檔**: https://example.invalid/devcontainers/features/tree/main/src/common-utils
- **VS Code DevContainers**: https://example.invalid/docs/devcontainers/containers
- **Dev Containers UIDs 文章**: https://example.invalid/blog/posts/2024/dev-containers-uids/

## 總結

`common-utils` Feature 解決了 Docker 開發容器最常見的問題：**檔案權限不匹配**。

**核心配置：**
```json
{
  "features": {
    "example.invalid/devcontainers/features/common-utils:2": {
      "username": "devuser",  // 創建非 root 使用者
      "uid": "1000",          // 匹配主機 UID
      "gid": "1000"           // 匹配主機 GID
    }
  },
  "remoteUser": "devuser"     // 使用 devuser 登入容器
}
```

這樣配置後，容器內外的檔案操作完全透明，無需 `sudo` 或 `chown`。
