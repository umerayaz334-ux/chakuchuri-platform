#Requires -Version 5.1
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'local-build-env.ps1')
$root = Split-Path $PSScriptRoot -Parent
$logs = Join-Path $root '.logs'
New-Item -ItemType Directory -Force -Path $logs | Out-Null

function Test-Port([int]$Port) {
    return [bool](Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue)
}

if (-not (Test-Port 8002)) {
    if (-not (Test-Path -LiteralPath (Join-Path $root '.runtime\local-api.json'))) {
        throw 'PostgreSQL migration/configuration is required. No JSON fallback will be started.'
    }
    $arguments = '-NoProfile -ExecutionPolicy Bypass -File "' + (Join-Path $PSScriptRoot 'run-api.ps1') + '"'
    $process = Start-Process powershell.exe -ArgumentList $arguments -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $logs 'backend-8002.out.log') -RedirectStandardError (Join-Path $logs 'backend-8002.err.log')
    $deadline = (Get-Date).AddSeconds(30)
    while (-not (Test-Port 8002) -and (Get-Date) -lt $deadline -and -not $process.HasExited) {
        Start-Sleep -Milliseconds 500
    }
}
# Refuse to advertise an old JSON process or an unrelated service as the live API.
& (Join-Path $PSScriptRoot 'verify-release.ps1') -ApiBase 'http://localhost:8002'
if (-not (Test-Port 5170)) {
    Start-Process -FilePath (Join-Path $PSScriptRoot 'run-web.cmd') -WindowStyle Hidden
    $deadline = (Get-Date).AddSeconds(30)
    while (-not (Test-Port 5170) -and (Get-Date) -lt $deadline) { Start-Sleep -Milliseconds 500 }
    if (-not (Test-Port 5170)) { throw 'Frontend did not start on port 5170.' }
}
Write-Output 'Open http://localhost:5170. The API uses PostgreSQL on port 8002.'
