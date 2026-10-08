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

# 服务 DSN（参数：数据库名，如 micro_party）
export DSN_DB="${1:-}"
if [ -n "$DSN_DB" ]; then
  export DSN="micro_app:$(grep MICRO_DEV_MYSQL_APP_PW "$DEV_ENV" | cut -d= -f2)@tcp(127.0.0.1:23306)/${DSN_DB}?charset=utf8mb4&parseTime=true&loc=Local"
fi
