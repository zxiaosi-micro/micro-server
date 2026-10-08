# start-one.ps1 · 单服务分离启动（Start-Process 解耦 bash/pwsh 引号；WorkingDirectory 承载含空格路径）
param(
    [Parameter(Mandatory = $true)][string]$Name,
    [Parameter(Mandatory = $true)][int]$Port,
    [Parameter(Mandatory = $true)][string]$LogDir
)
$exe = "C:/Users/zxiaosi/AppData/Local/Temp/micro-smoke/$Name.exe"
$svcDir = "D:/Personal code/micro-new/micro-server/services/$Name"
Start-Process -FilePath $exe `
    -ArgumentList @('-f', "etc/$Name.yaml") `
    -WorkingDirectory $svcDir `
    -WindowStyle Hidden `
    -RedirectStandardOutput "$LogDir/$Name.log" `
    -RedirectStandardError "$LogDir/$Name.err.log"
