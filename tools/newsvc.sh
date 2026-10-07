#!/bin/bash
# =============================================================================
# newsvc.sh · 新服务脚手架（S2-05，v4/02 附录 B checklist 第 1~2 步）
#
# 用法：./tools/newsvc.sh <name> rpc|api
#   rpc  → services/<name>/（pb 契约模板 + goctl rpc 骨架 + 独立 module）
#   api  → services/<name>/（api 契约模板 + goctl api 骨架 + 独立 module）
#
# 端口登记校验（E12）：name 必须已登记在 tools/port_registry.txt（与 v4/02 §5.2 同源），
# 未登记直接拒绝——先补 §5.2 表与本文件，再生成骨架。
# =============================================================================
set -euo pipefail

NAME="${1:-}"
TYPE="${2:-}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVER_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"       # micro-server 根
DEV_ROOT="$(cd "$SERVER_ROOT/.." && pwd)"          # 开发机约定根（go.work 不入库）

usage() { echo "用法：$0 <name> rpc|api"; exit 2; }
[ -n "$NAME" ] || usage
{ [ "$TYPE" = "rpc" ] || [ "$TYPE" = "api" ]; } || usage
echo "$NAME" | grep -qE '^[a-z][a-z0-9-]*$' || { echo "newsvc: 服务名限小写字母开头（a-z0-9-）"; exit 2; }

# ---- 1) 端口登记校验（E12：先登记后使用）----
REG="$SCRIPT_DIR/port_registry.txt"
ROW="$(grep -E "^${NAME} " "$REG" || true)"
if [ -z "$ROW" ]; then
  echo "newsvc: [E12 拒绝] $NAME 未登记端口——先在两处登记后再生成骨架："
  echo "  1) micro-deploy/docs/v4/02-技术文档.md §5.2 业务服务端口表"
  echo "  2) tools/port_registry.txt（本脚本取号源）"
  echo "  取号规则：RPC/HTTP 80xx、metrics 91xx；预留 8088/9108、8096/9116 不复用"
  exit 1
fi
PORT="$(echo "$ROW" | awk '{print $2}')"
METRICS="$(echo "$ROW" | awk '{print $3}')"
echo "端口取号：${PORT}（主）/ ${METRICS}（metrics）"

SVC_DIR="$SERVER_ROOT/services/$NAME"
[ -e "$SVC_DIR" ] && { echo "newsvc: $SVC_DIR 已存在"; exit 1; }
mkdir -p "$SVC_DIR"
cd "$SVC_DIR"

# ---- 2) 契约模板 + goctl 生成（契约先行，S0-05）----
UPPER_NAME="$(echo "$NAME" | sed -r 's/(^|-)([a-z])/\U\2/g')"   # admin-bff → AdminBff

if [ "$TYPE" = "rpc" ]; then
  mkdir -p pb
  cat > "pb/${NAME}.proto" <<EOF
syntax = "proto3";

package ${NAME};

option go_package = "./pb";

// ${UPPER_NAME} 服务契约（契约先行：先改本文件评审，再 goctl 生成实现）
service ${UPPER_NAME} {
  rpc Ping(PingReq) returns (PingResp);
}

message PingReq {}
message PingResp {
  string pong = 1;
}
EOF
  # --module 直接以服务全路径作为 module（生成物 import 路径随之正确）
  goctl rpc protoc "pb/${NAME}.proto" --go_out=. --go-grpc_out=. --zrpc_out=. --style go_zero --module "micro-server/services/${NAME}"
else
  mkdir -p api
  cat > "api/${NAME}.api" <<EOF
syntax = "v1"

info (
	title:   "${UPPER_NAME}"
	desc:    "${NAME} REST 契约（契约先行：先改本文件评审，再 goctl 生成实现）"
	author:  "micro"
	version: "v1"
)

type PingResp {
	Pong string \`json:"pong"\`
}

@server (
	prefix: /api/v1
)
service ${UPPER_NAME}Api {
	@handler ping
	get /ping returns (PingResp)
}
EOF
  goctl api go -api "api/${NAME}.api" -dir . --style go_zero
fi

# ---- 3) etc/<name>.yaml：${ENV} 占位（零硬编码密钥，E11）+ 可观测三段 ----
ETC_PORT="$PORT"
if [ "$TYPE" = "rpc" ]; then
  cat > "etc/${NAME}.yaml" <<EOF
Name: ${NAME}.rpc
ListenOn: "0.0.0.0:\${RPC_PORT|${ETC_PORT}}"
Etcd:
  Hosts:
    - \${ETCD_HOSTS|127.0.0.1:22379}
  Key: ${NAME}.rpc
DataSource: "\${DSN|}"
Cache:
  - Host: "\${REDIS_HOSTS|127.0.0.1:26379}"
    Pass: "\${REDIS_PASS|}"
    Type: node
Prometheus:
  Host: 0.0.0.0
  Port: ${METRICS}
Telemetry:
  Name: ${NAME}
  Endpoint: \${OTLP_ENDPOINT|127.0.0.1:24317}
  Sampler: 1.0
  Batcher: otlpgrpc        # E18：Batcher: jaeger 已废弃，配了启动失败
ConfigKey: \${CONFIG_KEY|/micro/config/${NAME}}   # configcenter（S4 起消费，ADR-10）
EOF
else
  cat > "etc/${NAME}.yaml" <<EOF
Name: ${NAME}-api
Host: 0.0.0.0
Port: ${ETC_PORT}
Upstreams:
  - Etcd:
      Hosts:
        - \${ETCD_HOSTS|127.0.0.1:22379}
    Target: \${UPSTREAM_TARGET|}
Prometheus:
  Host: 0.0.0.0
  Port: ${METRICS}
Telemetry:
  Name: ${NAME}
  Endpoint: \${OTLP_ENDPOINT|127.0.0.1:24317}
  Sampler: 1.0
  Batcher: otlpgrpc        # E18：Batcher: jaeger 已废弃，配了启动失败
EOF
fi

# ---- 4) 独立 module + go.work 登记（每服务独立 module，v4/02 §4.1）----
if [ ! -f go.mod ]; then
  go mod init "micro-server/services/${NAME}"
fi
go mod tidy
if [ -f "$SERVER_ROOT/go.work" ]; then
  (cd "$SERVER_ROOT" && go work use "./services/${NAME}")
fi
if [ -f "$DEV_ROOT/go.work" ]; then
  (cd "$DEV_ROOT" && go work use "./micro-server/services/${NAME}")
fi

# ---- 5) 编译验证 ----
go build ./...

echo
echo "== newsvc 完成：micro-server/services/${NAME}（module micro-server/services/${NAME}）=="
echo "后续步骤（附录 B checklist）："
echo "  3. 契约已生成 → 评审 → logic 实现 → swagger/apitypes 重生成（CI diff）"
echo "  4. 建库：go run ./tools/migrate -svc ${NAME} up"
echo "  6. 事件：eventbus.Emit + mqinit 预建 topic + cron 注册表登记"
echo "  7. 出站依赖登记 02 §10 韧性策略表"
echo "  8. 配置：新增项进 micro-deploy/config/${NAME}/ → PR → configpush 推送 etcd"
echo "  9. runbook 五节 → dodcheck 0 FAIL"
echo " 10. start_all.ps1 / §5.2 端口表 / README 服务清单同步登记"
