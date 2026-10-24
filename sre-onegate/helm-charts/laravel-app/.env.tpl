APP_NAME={{ .example.invalid.name }}
APP_ENV={{ .example.invalid.env }}
APP_KEY={{ .example.invalid.key }}
APP_DEBUG={{ .example.invalid.debug }}
APP_URL={{ .example.invalid.url }}
API_URL={{ .example.invalid.api_url }}

# API_DEBUGGER_ENABLED=false

APP_LOCALE=en
APP_FALLBACK_LOCALE=en
APP_FAKER_LOCALE=en_US

APP_MAINTENANCE_DRIVER=file
# APP_MAINTENANCE_STORE=database

PHP_CLI_SERVER_WORKERS=4

BCRYPT_ROUNDS=12

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
QUEUE_CONNECTION=database

CACHE_STORE=redis
CACHE_PREFIX=cache:

REDIS_CLIENT=phpredis
REDIS_HOST={{ .example.invalid.redis.host }}
REDIS_PASSWORD={{ .example.invalid.redis.password }}
REDIS_PORT={{ .example.invalid.redis.port }}
REDIS_PREFIX=
REDIS_DB={{ .example.invalid.redis.db }}
REDIS_CACHE_DB=${REDIS_DB}

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

# Google Cloud Storage
GOOGLE_CLOUD_PROJECT_ID=
GOOGLE_CLOUD_KEY_FILE=
GOOGLE_CLOUD_STORAGE_BUCKET=


VITE_APP_NAME="${APP_NAME}"

# Timezone
FRONTEND_TIMEZONE={{ .example.invalid.timezone.frontend }}
BACKEND_TIMEZONE={{ .example.invalid.timezone.backend }}

# Bot Notify
BOT_TELEGRAM_TOKEN={{ .example.invalid.bot.telegram_token }}
BOT_DEBUG_CHAT_ID={{ .example.invalid.bot.debug_chat_id }}
BOT_INFO_CHAT_ID=${BOT_DEBUG_CHAT_ID}
BOT_ERROR_CHAT_ID={{ .example.invalid.bot.error_chat_id }}
BOT_WARN_CHAT_ID={{ .example.invalid.bot.warn_chat_id }}

# 開關
SWITCH_LOG_REQUEST={{ .example.invalid.switch.log_request }}
SWITCH_LOG_SQL={{ .example.invalid.switch.log_sql }}
SCRAMBLE_ENABLED={{ .example.invalid.switch.scramble_enabled }}
