#!/bin/sh
set -e

# 启动 cron
cron

# 启动 php-fpm
exec php-fpm -F