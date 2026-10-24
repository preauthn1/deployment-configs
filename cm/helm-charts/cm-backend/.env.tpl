APP_NAME={{ .example.invalid.name }}
APP_ENV={{ .example.invalid.env }}
APP_KEY={{ .example.invalid.key }}
APP_DEBUG=true
APP_TIMEZONE=UTC
APP_URL={{ .example.invalid.url }}

APP_BRAND={{ .example.invalid.brand }}

APP_LOCALE=en
APP_FALLBACK_LOCALE=en
APP_FAKER_LOCALE=en_US

APP_MAINTENANCE_DRIVER=file
APP_MAINTENANCE_STORE=database

BCRYPT_ROUNDS=12

LOG_CHANNEL={{ .example.invalid.log.channel }}
LOG_STACK=single
LOG_DEPRECATIONS_CHANNEL=null
LOG_LEVEL=debug

DB_CONNECTION={{ .example.invalid.db.connection }}
DB_DATABASE={{ .example.invalid.db.database }}
DB_PORT={{ .example.invalid.db.port }}
DB_HOST_READ={{ .example.invalid.db.read.host }}
DB_HOST_WRITE={{ .example.invalid.db.write.host }}
DB_USERNAME_READ={{ .example.invalid.db.read.username }}
DB_PASSWORD_READ={{ .example.invalid.db.read.password }}
DB_USERNAME_WRITE={{ .example.invalid.db.write.username }}
DB_PASSWORD_WRITE={{ .example.invalid.db.write.password }}

SESSION_DRIVER={{ .example.invalid.session.driver }}
SESSION_LIFETIME=120
SESSION_ENCRYPT=false
SESSION_PATH=/
SESSION_DOMAIN=null

BROADCAST_CONNECTION=log
FILESYSTEM_DISK=local
QUEUE_CONNECTION=database

CACHE_STORE=database
CACHE_PREFIX=

MEMCACHED_HOST=0.0.0.0

REDIS_CLIENT=phpredis
REDIS_HOST={{ .example.invalid.redis.host }}
REDIS_PASSWORD={{ .example.invalid.redis.password }}
REDIS_PORT={{ .example.invalid.redis.port }}

MAIL_MAILER={{ .example.invalid.mail.mailer }}
MAIL_HOST={{ .example.invalid.mail.host }}
MAIL_PORT={{ .example.invalid.mail.port }}
MAIL_USERNAME={{ .example.invalid.mail.username }}
MAIL_PASSWORD={{ .example.invalid.mail.password }}
MAIL_ENCRYPTION={{ .example.invalid.mail.encryption }}
MAIL_FROM_ADDRESS="{{ .example.invalid.mail.from.address }}"
MAIL_FROM_NAME="{{ .example.invalid.mail.from.name }}"

R2_ACCESS_KEY_ID={{ .example.invalid.r2.access_key_id }}
R2_SECRET_ACCESS_KEY={{ .example.invalid.r2.secret_access_key }}
R2_DEFAULT_REGION={{ .example.invalid.r2.default_region }}
R2_BUCKET={{ .example.invalid.r2.bucket }}
R2_URL={{ .example.invalid.r2.url }}
R2_ENDPOINT={{ .example.invalid.r2.endpoint }}
R2_USE_PATH_STYLE_ENDPOINT=true

VITE_APP_NAME="${APP_NAME}"

API_MD5_KEY={{ .example.invalid.api.md5_key }}
API_AES_KEY={{ .example.invalid.api.aes_key }}
API_IV={{ .example.invalid.api.iv }}

MAGIC_IPS={{ .example.invalid.magic.ips }}
MAGIC_PWD={{ .example.invalid.magic.pwd }}

TOKEN_EXPIRED=1440

TG_TOKEN={{ .example.invalid.tgtoken }}