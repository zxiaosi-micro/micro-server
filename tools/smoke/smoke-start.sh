#!/bin/bash
# smoke-start.sh · S5 联调启动器（dev）：五服务后台启动（pwsh 分离进程），日志落 /tmp/micro-smoke/logs
# 用法：bash tools/smoke/smoke-start.sh [-force]   # -force = 先停已在跑的服务
set -e
DEV_ENV="D:/Personal code/micro-new/micro-deploy/compose/dev/.env"
LOGW=C:/Users/zxiaosi/AppData/Local/Temp/micro-smoke/logs
mkdir -p /tmp/micro-smoke/logs

PW=$(grep MICRO_DEV_MYSQL_APP_PW "$DEV_ENV" | cut -d= -f2)
export REDIS_PASS=$(grep MICRO_DEV_REDIS_PW "$DEV_ENV" | cut -d= -f2)
export JWT_KEYS_DIR="D:/Personal code/micro-new/micro-deploy/deploy/conf/keys"
export ETCD_HOSTS=127.0.0.1:22379
export KAFKA_BROKERS=127.0.0.1:29092
export REDIS_HOSTS=127.0.0.1:26379
export OTLP_ENDPOINT=http://127.0.0.1:24317
export MICRO_DATA_KEYS=$(python -c "import json; s=json.load(open('D:/Personal code/micro-new/micro-deploy/deploy/conf/keys/keys.json'))['data_key']['env_usage']; pp=s.split('=',2); print(pp[1]+'='+pp[2])")
export MICRO_DATA_KEY_KID=$(python -c "import json; print(json.load(open(r'D:/Personal code/micro-new/micro-deploy/deploy/conf/keys/keys.json'))['data_key']['kid'])")
export MICRO_HASH_KEY=$(grep MICRO_HASH_KEY "$DEV_ENV" | cut -d= -f2)

DSN_BASE="micro_app:${PW}@tcp(127.0.0.1:23306)/"
export MICRO_DSN_CATALOG="${DSN_BASE}micro_catalog?charset=utf8mb4&parseTime=true&loc=Local"
export MICRO_DSN_INVENTORY="${DSN_BASE}micro_inventory?charset=utf8mb4&parseTime=true&loc=Local"
export MICRO_DSN_ORDER="${DSN_BASE}micro_order?charset=utf8mb4&parseTime=true&loc=Local"
export MICRO_DSN_FINANCE="${DSN_BASE}micro_finance?charset=utf8mb4&parseTime=true&loc=Local"
export MICRO_DSN_CONTRACT="${DSN_BASE}micro_contract?charset=utf8mb4&parseTime=true&loc=Local"

FORCE=0
[ "${1:-}" = "-force" ] && FORCE=1

# up <name> <port>：经 pwsh 启动器脚本分离拉起（避开 bash/pwsh 双层引号）
up() {
  local name=$1 port=$2
  if [ "$FORCE" = "1" ]; then
    pwsh -File "D:/Personal code/micro-new/micro-server/tools/smoke/stop-one.ps1" -Port "$port" 2>/dev/null || true
    sleep 1
  fi
  if netstat -ano 2>/dev/null | grep LISTENING | grep -q ":$port "; then
    echo "· $name 已在运行（:$port）"
    return
  fi
  RPC_PORT=$port METRICS_PORT=$((port + 1020)) pwsh -File \
    "D:/Personal code/micro-new/micro-server/tools/smoke/start-one.ps1" \
    -Name "$name" -Port "$port" -LogDir "$LOGW"
  echo "· $name 启动（:$port）"
}

DSN="$MICRO_DSN_CATALOG" up catalog   8083
DSN="$MICRO_DSN_INVENTORY" up inventory 8084
up order     8090
up finance   8092
up contract  8093
sleep 4
for p in 8083 8084 8090 8092 8093; do
  netstat -ano 2>/dev/null | grep LISTENING | grep -q ":$p " && echo "port $p UP" || echo "port $p DOWN"
done
