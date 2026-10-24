SITE_NAME={{ .example.invalid.site_name }}
APP_NAME={{ .example.invalid.name }}
APP_ENV={{ .example.invalid.env }}
APP_KEY={{ .example.invalid.key }}
APP_DEBUG={{ .example.invalid.debug }}
APP_URL={{ .example.invalid.url }}

APP_LOCALE=en
APP_FALLBACK_LOCALE=en
APP_FAKER_LOCALE=en_US

APP_MAINTENANCE_DRIVER=file
# APP_MAINTENANCE_STORE=database

PHP_CLI_SERVER_WORKERS=4

BCRYPT_ROUNDS=12

# 主要幣別
CURRENCY_ID={{ .example.invalid.url }}

# Filament
AUTH_MODEL=App\Models\Agent

LOG_CHANNEL={{ .example.invalid.log.channel }}
LOG_STACK=single
LOG_DEPRECATIONS_CHANNEL=deprecations
LOG_LEVEL={{ .example.invalid.log.level }}

DB_CONNECTION={{ .example.invalid.db.connection }}
DB_PORT={{ .example.invalid.db.port }}
DB_DATABASE={{ .example.invalid.db.database }}
DB_LOG_DATABASE={{ .example.invalid.db.database }}_log
DB_HOST_READ={{ .example.invalid.db.read.host }}
DB_USERNAME_READ={{ .example.invalid.db.read.username }}
DB_PASSWORD_READ={{ .example.invalid.db.read.password }}
DB_HOST_WRITE={{ .example.invalid.db.write.host }}
DB_USERNAME_WRITE=${DB_USERNAME_READ}
DB_PASSWORD_WRITE=${DB_PASSWORD_READ}

SESSION_DRIVER={{ .example.invalid.session.driver }}
SESSION_CONNECTION=session
SESSION_LIFETIME=120
SESSION_ENCRYPT=false
SESSION_PATH=/
SESSION_DOMAIN=null

BROADCAST_CONNECTION=log
{{ if .example.invalid.gcs.enabled }}
FILESYSTEM_DISK=gcs
{{ else }}
FILESYSTEM_DISK=local
{{ end }}
QUEUE_CONNECTION={{ .example.invalid.queue.connection | default "database" }}

CACHE_STORE=redis
CACHE_PREFIX=

REDIS_CLIENT=phpredis
REDIS_HOST={{ .example.invalid.redis.host }}
REDIS_PASSWORD={{ .example.invalid.redis.password }}
REDIS_PORT={{ .example.invalid.redis.port }}
REDIS_PREFIX=
REDIS_DB={{ .example.invalid.redis.db }}
REDIS_CACHE_DB=${REDIS_DB}
REDIS_QUEUE_CONNECTION=queue
REDIS_QUEUE_DB={{ .example.invalid.redis.queue_db | default "3" }}

# Horizon (#3967)
HORIZON_ALLOWED_IPS={{ .example.invalid.horizon.allowed_ips | default "" }}
# 注意：這個 TRUSTED_PROXIES 目前只餵給 Laravel 內建 TrustProxies middleware
# （Horizon 儀表板用），跟下面 target/tickets#4187 的 Common::resolveClientIp()
# 是兩套獨立機制，不要混用同一個 key。
TRUSTED_PROXIES={{ .example.invalid.horizon.trusted_proxies | default "" }}

# target/tickets#4187：Common::resolveClientIp() 信任鏈設定。
# skip_last_hop 只在已驗證屬於 GKE + Google Cloud LB 拓樸的環境開啟——
# REMOTE_ADDR 落在官方 GFE 範圍、via: 1.1 google／x-cloud-trace-context
# 都對得上才能開，否則可能讓攻擊者偽造的值被誤判為真實 IP（見
# app/Helpers/Common.php resolveClientIp() 文件）。
TRUSTED_PROXY_EXTRA_CIDRS={{ .example.invalid.trusted_proxy.extra_cidrs | default "" }}
TRUSTED_PROXY_SKIP_LAST_HOP={{ .example.invalid.trusted_proxy.skip_last_hop | default "false" }}
# Horizon metadata prefix：固定值，不走 APP_NAME fallback。
# agent/frontend 等 deployment 用 k8s env 覆蓋 APP_NAME（Agent_API/Frontend_API），horizon deployment 不覆蓋（Laravel），
# 若 prefix 依賴 APP_NAME 會導致 master 與 dashboard 讀寫不同 key → dashboard Inactive、completed/metrics 全空。
# 單站預設 {site_name}_horizon:；共用 redis 的 vworldmy/wintoto 於 values 設 app.horizon.prefix=xsgcasino_horizon: 合併 dashboard。
HORIZON_PREFIX={{ .example.invalid.horizon.prefix | default (printf "%s_horizon:" .example.invalid.site_name) }}

# Olympus Office 365 SSO（#4081）：redirect 由 app.azure.redirect_uri 明確指定（tools-* 後台瀏覽網域），
# 各環境 callback URI 須先登記進 Entra app registration；未配置的環境保持 force_sso=false 走帳密登入
AZURE_CLIENT_ID={{ .example.invalid.azure.client_id | default "" }}
AZURE_CLIENT_SECRET={{ .example.invalid.azure.client_secret | default "" }}
AZURE_TENANT={{ .example.invalid.azure.tenant | default "" }}
# redirect 必填 tools-* 後台瀏覽網域
AZURE_REDIRECT_URI={{ .example.invalid.azure.redirect_uri | default "" }}
OLYMPUS_FORCE_SSO={{ .example.invalid.azure.force_sso | default "false" }}

MAIL_MAILER={{ .example.invalid.mail.driver }}
MAIL_SCHEME={{ .example.invalid.mail.scheme }}
MAIL_HOST={{ .example.invalid.mail.host }}
MAIL_PORT={{ .example.invalid.mail.port }}
MAIL_USERNAME={{ .example.invalid.mail.username }}
MAIL_PASSWORD={{ .example.invalid.mail.password | quote }}
MAIL_FROM_ADDRESS={{ .example.invalid.mail.from.address | quote }}
MAIL_FROM_NAME={{ .example.invalid.mail.from.name | quote }}

AWS_ACCESS_KEY_ID=
AWS_SECRET_ACCESS_KEY=
[REDACTED]
AWS_BUCKET=
AWS_USE_PATH_STYLE_ENDPOINT=false

VITE_APP_NAME="${APP_NAME}"

# Timezone
FRONTEND_TIMEZONE={{ .example.invalid.timezone.frontend }}
BACKEND_TIMEZONE={{ .example.invalid.timezone.backend }}

# Domain
API_EXTERNAL_URL={{ .example.invalid.api_external_url }}
API_CS_URL={{ .example.invalid.api_cs_url | quote }}
WEB_FRONTEND_URL={{ .example.invalid.web_frontend_url }}

# API 加密
AES_KEY={{ .example.invalid.api.aes_key }}
AES_IV={{ .example.invalid.api.aes_iv }}
AES_MD5={{ .example.invalid.api.aes_md5 }}

# 3rd-party API 加密
THIRD_PARTY_AES_KEY={{ .example.invalid.third_party.aes_key | quote }}
THIRD_PARTY_IV={{ .example.invalid.third_party.aes_iv | quote }}
THIRD_PARTY_MD5_KEY={{ .example.invalid.third_party.aes_md5 | quote }}

# 辦公室
OFFICE_IP={{ .example.invalid.office.ip }}
OFFICE_PASSWORD={{ .example.invalid.office.password }}

# 無限代前綴
UNLIMIT_PREFIX={{ .example.invalid.frontend.unlimit_prefix }}
EVENT_CODE_PREFIX={{ .example.invalid.frontend.event_code_prefix }}
PGS_LAYOUT_LOBBY_AGENT={{ .example.invalid.frontend.pgs_layout_lobby_agent }}

# 報表排除的遊戲商（逗號分隔 vendor_id）：代理／站點看板的投注、輸贏、盤費、投注人數不計這批遊戲商，
# 值與 WEB_AgentBackend 各站 _init.php 的 SR11_NO_PRESELECTION 常數同步（target/tickets#4335）；空值＝不排除
SR11_NO_PRESELECTION={{ .example.invalid.report.sr11_no_preselection }}

# Bot Notify
BOT_TELEGRAM_TOKEN={{ .example.invalid.bot.telegram_token }}
BOT_DEBUG_CHAT_ID={{ .example.invalid.bot.debug_chat_id }}
BOT_INFO_CHAT_ID={{ .example.invalid.bot.info_chat_id }}
BOT_ERROR_CHAT_ID={{ .example.invalid.bot.error_chat_id }}
BOT_WARN_CHAT_ID={{ .example.invalid.bot.warn_chat_id }}

# 大額提款通知
LARGE_AMOUNT_CHAT_ID={{ .example.invalid.large_amount.chat_id }}
LARGE_AMOUNT_WATCH_AGENTS={{ .example.invalid.large_amount.watch_agents }}
LARGE_AMOUNT_THRESHOLD={{ .example.invalid.large_amount.threshold | default 5000 }}

# 開關
SWITCH_LOG_REQUEST={{ .example.invalid.switch.log_request }}
SWITCH_LOG_SQL={{ .example.invalid.switch.log_sql }}
SCRAMBLE_ENABLED={{ .example.invalid.switch.scramble_enabled }}
ACTIVITY_LOGGER_ENABLED={{ .example.invalid.switch.activity_logger_enabled }}
# 代付銀行碼改查 cash_flow_banks 的金流商名單：填金流商 driver 名（通常是 APIs 的 Provider 目錄名，例如 FPay），逗號分隔；空＝全部走舊的 available_banks JSON。
# 一站一份、一家一家開，開之前先在該站跑 cash-flow:verify-payout-bank-parity 確認新舊兩條路徑答案一致（APIs config/payment.php 有完整說明）。
PAYOUT_BANK_LOOKUP_PROVIDERS={{ .example.invalid.switch.payout_bank_lookup_providers | default "" }}

# SMS
SMS_TOKEN_EXPIRED=6
SMS_MAX_SEND=3

# Google Cloud Storage
GOOGLE_CLOUD_PROJECT_ID={{ .example.invalid.gcs.project_id }}
GOOGLE_CLOUD_KEY_FILE={{ .example.invalid.gcs.key_file.mount_path }}
GOOGLE_CLOUD_STORAGE_BUCKET={{ .example.invalid.gcs.storage_bucket }}

# Tiktok
TT_API_URL={{ .example.invalid.tt.api_url }}
TT_API_KEY={{ .example.invalid.tt.api_key }}
TT_PRIVATE_KEY={{ .example.invalid.tt.private_key }}

HTTP_CLIENT_LOGGER_ENABLED=true
HTTP_CLIENT_LOGGER_CHANNEL={{ .example.invalid.log.channel }}
HTTP_CLIENT_LOGGER_FILTER_ALL=true

LIVECHAT_BOT_API_KEY={{ .example.invalid.live_chat.bot_api_key | quote }}