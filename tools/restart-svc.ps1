# S2-05 · restart-svc.ps1：重编译 + 拉起单个服务 + 日志重定向 logs/
# 用法：pwsh -File tools/restart-svc.ps1 <svc> [-NoBuild]
#   - 重编译（workspace 根执行 go build）
#   - 杀旧进程（logs/<svc>.pid）→ 启动新进程（-f services/<svc>/etc/<svc>.yaml）
#   - stdout/stderr 重定向 logs/<svc>_stdout.log / logs/<svc>_stderr.log
#     （logx file 模式另写 logs/<svc>_run.log，由 Promtail 采集——*_run.log 已 gitignore）
# E9：所有含空格路径一律引号包裹。
param(
    [Parameter(Mandatory = $true)][string]$svc,
    [switch]$NoBuild
)
$ErrorActionPreference = 'Stop'

$serverRoot = Split-Path -Parent $PSScriptRoot   # micro-server 根
$logsDir = Join-Path $serverRoot 'logs'
$exe = Join-Path $logsDir "$svc.exe"
$cfg = Join-Path $serverRoot "services\$svc\etc\$svc.yaml"
$pidFile = Join-Path $logsDir "$svc.pid"

New-Item -ItemType Directory -Force -Path $logsDir | Out-Null
if (-not (Test-Path $cfg)) { Write-Error "配置不存在：$cfg（先用 tools/newsvc.sh 生成服务骨架）" }

# ---- 重编译 ----
if (-not $NoBuild) {
    Write-Host "== go build $svc =="
    Push-Location $serverRoot
    try { go build -o "$exe" "./services/$svc"; if ($LASTEXITCODE -ne 0) { throw "go build 失败（exit $LASTEXITCODE）" } }
    finally { Pop-Location }
}
elseif (-not (Test-Path $exe)) { Write-Error "-NoBuild 但 $exe 不存在" }

# ---- 杀旧进程 ----
if (Test-Path $pidFile) {
    $oldPid = Get-Content $pidFile -ErrorAction SilentlyContinue
    if ($oldPid) {
        $old = Get-Process -Id $oldPid -ErrorAction SilentlyContinue
        if ($old) { Write-Host "停止旧进程 pid=$oldPid"; Stop-Process -Id $oldPid -Force; Start-Sleep -Milliseconds 500 }
    }
    Remove-Item $pidFile -ErrorAction SilentlyContinue
}
# 兜底：同可执行名的残留进程
Get-Process -Name "$svc" -ErrorAction SilentlyContinue | ForEach-Object {
    Write-Host "停止残留进程 pid=$($_.Id)"; Stop-Process -Id $_.Id -Force
}

# ---- 拉起 ----
$stdout = Join-Path $logsDir "${svc}_stdout.log"
$stderr = Join-Path $logsDir "${svc}_stderr.log"
$proc = Start-Process -FilePath $exe -ArgumentList @('-f', $cfg) `
    -WorkingDirectory $serverRoot `
    -RedirectStandardOutput $stdout -RedirectStandardError $stderr `
    -PassThru -WindowStyle Hidden
Set-Content -Path $pidFile -Value $proc.Id

Write-Host "== $svc 已启动 pid=$($proc.Id) =="
Write-Host "   日志：logs\${svc}_run.log（logx）/ ${svc}_stdout.log / ${svc}_stderr.log"
Write-Host "   metrics: http://127.0.0.1:$((Select-String -Path $cfg -Pattern 'Port:\s*(\d+)' | ForEach-Object { $_.Matches[0].Groups[1].Value } | Select-Object -Last 1))"
