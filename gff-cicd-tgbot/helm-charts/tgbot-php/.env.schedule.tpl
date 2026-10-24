APP_NAME={{ .example.invalid.name }}
APP_ENV={{ .example.invalid.env }}
APP_KEY={{ .example.invalid.key }}
APP_DEBUG=false
APP_TIMEZONE=UTC
APP_URL={{ .example.invalid.url }}

API_EXTERNAL_URL= {{ .example.invalid.api_external_url }}
WEB_FRONTEND_URL= {{ .example.invalid.web_frontend_url }}


APP_LOCALE=en-us
APP_FALLBACK_LOCALE=en
APP_FAKER_LOCALE=en_US

APP_MAINTENANCE_DRIVER=file
# APP_MAINTENANCE_STORE=database

BCRYPT_ROUNDS=12

LOG_CHANNEL=laravel
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
QUEUE_CONNECTION=redis

CACHE_STORE=redis
CACHE_PREFIX=

REDIS_CLIENT=phpredis
REDIS_HOST={{ .example.invalid.redis.host }}
REDIS_PASSWORD={{ .example.invalid.redis.password }}
REDIS_PORT={{ .example.invalid.redis.port }}
REDIS_PREFIX={{ .example.invalid.redis.prefix }}
REDIS_DB={{ .example.invalid.redis.db }}

API_URL={{ .example.invalid.api.url }}
API_MD5_KEY={{ .example.invalid.api.md5_key }}
API_AES_KEY={{ .example.invalid.api.aes_key }}
API_IV={{ .example.invalid.api.iv }}
TOKEN_EXPIRED=1440

PLATFORM_API_SEC_KEY={{ .example.invalid.api.secret_key }}
