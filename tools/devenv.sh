#!/bin/bash
# devenv.sh · dev 环境变量注入（S4 本地联调用；读取 micro-deploy/compose/dev/.env，零硬编码密钥）
# 用法：source tools/devenv.sh
DEV_ENV="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/../micro-deploy/compose/dev/.env"

export REDIS_PASS=$(grep MICRO_DEV_REDIS_PW "$DEV_ENV" | cut -d= -f2)
export MINIO_ACCESS_KEY=$(grep MICRO_DEV_MINIO_ROOT_USER "$DEV_ENV" | cut -d= -f2)
export MINIO_SECRET_KEY=$(grep MICRO_DEV_MINIO_ROOT_PASSWORD "$DEV_ENV" | cut -d= -f2)
export JWT_KEYS_DIR="D:/Personal code/micro-new/micro-deploy/deploy/conf/keys"

# 数据密钥（keys.json env_usage 展开）
DK=$(python -c "import json; s=json.load(open('D:/Personal code/micro-new/micro-deploy/deploy/conf/keys/keys.json'))['data_key']['env_usage']; pp=s.split('=',2); print(pp[1]+'='+pp[2])")
export MICRO_DATA_KEYS="$DK"
export MICRO_DATA_KEY_KID=$(python -c "import json; print(json.load(open(r'D:/Personal code/micro-new/micro-deploy/deploy/conf/keys/keys.json'))['data_key']['kid'])")

# HMAC 索引键/文件签名键（与 .env 同源；未登记则退 dev 默认）
export MICRO_HASH_KEY=$(grep MICRO_HASH_KEY "$DEV_ENV" | cut -d= -f2)
export MICRO_FILE_SIGN_KEY=$(grep MICRO_FILE_SIGN_KEY "$DEV_ENV" | cut -d= -f2)
[ -n "$MICRO_HASH_KEY" ] || export MICRO_HASH_KEY="micro-dev-index-key"
[ -n "$MICRO_FILE_SIGN_KEY" ] || export MICRO_FILE_SIGN_KEY="micro-dev-file-sign-key"

# EMQX/TDengine/OTA（设备链路，S6 起消费；.env 同源，零硬编码）
export EMQX_BROKER="${EMQX_BROKER:-tcp://127.0.0.1:21883}"
export EMQX_API_BASE="${EMQX_API_BASE:-http://127.0.0.1:38083}"
export EMQX_DASHBOARD_USER=$(grep MICRO_DEV_EMQX_DASHBOARD_USER "$DEV_ENV" | cut -d= -f2)
export EMQX_DASHBOARD_PASS=$(grep MICRO_DEV_EMQX_DASHBOARD_PW "$DEV_ENV" | cut -d= -f2)
# 设备联调账号（模拟器/冒烟：ACL pub up/# + sub down/#）
export EMQX_DEVICE_USER=$(grep MICRO_DEV_EMQX_DEVICE_USER "$DEV_ENV" | cut -d= -f2)
export EMQX_DEVICE_PASS=$(grep MICRO_DEV_EMQX_DEVICE_PW "$DEV_ENV" | cut -d= -f2)
# 平台后端账号（device 服务指令下行 + iotingest forwarder：ACL pub down/# + sub up/#）
export EMQX_PLATFORM_USER=$(grep MICRO_DEV_EMQX_PLATFORM_USER "$DEV_ENV" | cut -d= -f2)
export EMQX_PLATFORM_PASS=$(grep MICRO_DEV_EMQX_PLATFORM_PW "$DEV_ENV" | cut -d= -f2)
[ -n "$EMQX_PLATFORM_USER" ] || export EMQX_PLATFORM_USER="$EMQX_DEVICE_USER"
[ -n "$EMQX_PLATFORM_PASS" ] || export EMQX_PLATFORM_PASS="$EMQX_DEVICE_PASS"
export MICRO_DEV_TDENGINE_PW=$(grep MICRO_DEV_TDENGINE_PW "$DEV_ENV" | cut -d= -f2)
export TDENGINE_REST="${TDENGINE_REST:-http://127.0.0.1:26041}"
export TDENGINE_PASS="$MICRO_DEV_TDENGINE_PW"
export OTA_PUBLIC_KEY="${OTA_PUBLIC_KEY:-D:/Personal code/micro-new/micro-deploy/deploy/conf/keys/ota_ed25519_public.pem}"
export OTA_SIGN_KEY="${OTA_SIGN_KEY:-D:/Personal code/micro-new/micro-deploy/deploy/conf/keys/ota_ed25519_private.pem}"

# 服务 DSN（参数：数据库名，如 micro_party）
export DSN_DB="${1:-}"
if [ -n "$DSN_DB" ]; then
  export DSN="micro_app:$(grep MICRO_DEV_MYSQL_APP_PW "$DEV_ENV" | cut -d= -f2)@tcp(127.0.0.1:23306)/${DSN_DB}?charset=utf8mb4&parseTime=true&loc=Local"
fi
