#Requires -Version 5.1
param(
    [ValidateSet('status', 'up', 'down')][string]$Action = 'status',
    [int]$Steps = 1
)

$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'local-build-env.ps1')
$root = Split-Path $PSScriptRoot -Parent
$configPath = Join-Path $root '.runtime\local-api.json'
$runtimePath = Join-Path $root '.runtime\postgres-rehearsal'
if (-not (Test-Path -LiteralPath $configPath)) {
    throw 'Local PostgreSQL configuration is missing. Complete local setup first.'
}
$settings = Get-Content -LiteralPath $configPath -Raw | ConvertFrom-Json
& (Join-Path $PSScriptRoot 'local-postgres.ps1') -Action Start
if ($LASTEXITCODE -ne 0) { throw 'Local PostgreSQL could not start.' }

# Schema changes use the local PostgreSQL administrator. The API keeps using its restricted app role.
$credential = Import-Clixml -LiteralPath (Join-Path $runtimePath 'credential.xml')
$env:DATABASE_URL = 'postgres://' + [Uri]::EscapeDataString($credential.UserName) + ':' + [Uri]::EscapeDataString($credential.GetNetworkCredential().Password) + '@127.0.0.1:55432/' + $settings.database + '?sslmode=disable'
Push-Location (Join-Path $root 'backend')
try {
    if ($Action -eq 'down') {
        go run ./cmd/migrate -steps $Steps down
    } else {
        go run ./cmd/migrate $Action
    }
    if ($LASTEXITCODE -ne 0) { throw "Migration command failed with exit code $LASTEXITCODE." }
} finally {
    Pop-Location
    Remove-Item Env:DATABASE_URL -ErrorAction SilentlyContinue
}