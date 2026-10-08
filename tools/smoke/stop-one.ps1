# stop-one.ps1 · 按监听端口停服务（联调重启用）
param(
    [Parameter(Mandatory = $true)][int]$Port
)
$conns = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue
foreach ($c in $conns) {
    Stop-Process -Id $c.OwningProcess -Force -ErrorAction SilentlyContinue
}
