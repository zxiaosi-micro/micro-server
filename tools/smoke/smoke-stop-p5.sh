#!/bin/bash
# smoke-stop-p5.sh · 一键全停联调服务（与 smoke-start-p5.sh 对称）
# 用法：bash tools/smoke/smoke-stop-p5.sh [-all | 端口...]
#   无参      停 p5 的 11 个服务（identity/party/catalog/inventory/device/notification/order/finance/contract/ops/mobile-bff）
#   -all      按 tools/port_registry.txt 全量扫（含 station/analytics/hello/admin-bff 等其他已起服务）
#   端口...   只停指定端口
# 为什么必须显式停：服务经 start-one.ps1 分离启动（Start-Process，日志在 /tmp/micro-smoke/logs），
# 关终端/结束脚本都不会杀进程；遗留进程在中间件恢复后会苏醒重连（v4/04 kq 空转教训——
# 2026-10-12 曾有 8 个遗留进程把空载 broker 打到 150-400% CPU）。冒烟结束务必跑本脚本收尾。
set -e
TOOLDIR="$(cd "$(dirname "$0")" && pwd)"

PORTS="8081 8082 8083 8084 8085 8089 8090 8092 8093 8094 8890"
if [ "${1:-}" = "-all" ]; then
  PORTS="$(awk '$1 !~ /^#/ && $2 ~ /^[0-9]+$/ && $2 >= 8000 && $2 <= 8999 {print $2}' "$TOOLDIR/../port_registry.txt") 8182"
elif [ $# -gt 0 ]; then
  PORTS="$*"
fi

stopped=0
for p in $PORTS; do
  if netstat -ano 2>/dev/null | grep LISTENING | grep -q ":$p "; then
    pwsh -File "$TOOLDIR/stop-one.ps1" -Port "$p" >/dev/null 2>&1
    echo "· 已停 :$p"
    stopped=$((stopped + 1))
  fi
done

# 深扫兜底：按 exe 路径（%TEMP%\micro-smoke\）清理"进程还活着但没监听端口"的挂死实例
# ——按启动路径归属，绝不误伤同名系统进程（如微软的 identity_helper.exe）
pwsh -NoProfile -Command '
  Get-Process -ErrorAction SilentlyContinue |
    Where-Object { $_.Path -like "*\micro-smoke\*.exe" } |
    ForEach-Object { Write-Host ("· 深扫停止 " + $_.ProcessName + " (pid " + $_.Id + ")"); $_.Kill() }
' 2>/dev/null || true

echo "done：本次停掉 $stopped 个监听端口；日志在 /tmp/micro-smoke/logs"
