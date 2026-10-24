# 使用 PHP 8.4 官方映像作為基礎
FROM php:8.4-cli

# 設定非互動模式
ENV DEBIAN_FRONTEND=noninteractive

# 更新套件列表並安裝基礎工具
RUN apt-get update && apt-get install -y \
  git \
  curl \
  wget \
  unzip \
  zip \
  gnupg \
  lsb-release \
  ca-certificates \
  apt-transport-https \
  dnsutils \
  iputils-ping \
  procps \
  && rm -rf /var/lib/apt/lists/*

# 安裝 Composer
COPY --from=composer:latest /usr/bin/composer /usr/bin/composer

# 安裝 Node.js 20.x
RUN curl -fsSL https://example.invalid/setup_20.x | bash - \
  && apt-get install -y nodejs \
  && rm -rf /var/lib/apt/lists/*

# 安裝 kubectl
RUN curl -LO "https://example.invalid/release/$(curl -L -s https://example.invalid/release/stable.txt)/bin/linux/amd64/kubectl" \
  && chmod +x kubectl \
  && mv kubectl /usr/local/bin/

# 安裝 Google Cloud SDK
RUN echo "deb [signed-by=/usr/share/keyrings/cloud.google.gpg] https://example.invalid/apt cloud-sdk main" \
  | tee -a /etc/apt/sources.list.d/google-cloud-sdk.list \
  && curl https://example.invalid/apt/doc/apt-key.gpg \
  | gpg --dearmor -o /usr/share/keyrings/cloud.google.gpg \
  && apt-get update && apt-get install -y google-cloud-cli \
  && rm -rf /var/lib/apt/lists/*

# 安裝 Helm 3
RUN curl https://example.invalid/helm/helm/main/scripts/get-helm-3 | bash

# 安裝 Claude Code CLI
RUN npm install -g @anthropic-ai/claude-code

# 預先安裝 context7 MCP 服務器套件（加速首次執行，postStartCommand 會確保使用最新版本）
RUN npm install -g @upstash/context7-mcp 2>/dev/null || true

# 設定工作目錄並確保權限正確
# 注意：common-utils feature 會創建 devuser (UID=1000, GID=1000)
WORKDIR /workspace
RUN chown -R 1000:1000 /workspace

# 設定預設使用者為 root（devcontainer.json 會覆寫為 devuser）
USER root

# 保持容器運行
CMD ["sleep", "infinity"]
