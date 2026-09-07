#Requires -Version 5.1
param([string]$ConfigPath = '')
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'local-build-env.ps1')
$root = Split-Path $PSScriptRoot -Parent
if ($ConfigPath -eq '') { $ConfigPath = Join-Path $root '.runtime\local-api.json' }
if (-not (Test-Path -LiteralPath $ConfigPath)) {
    throw 'PostgreSQL runtime configuration is missing. Complete the migration before starting. No JSON fallback is allowed.'
}
$settings = Get-Content -LiteralPath $ConfigPath -Raw | ConvertFrom-Json
if ($settings.database -notmatch '^chakuchuri_[a-z0-9_]+$') { throw 'Invalid local database name.' }
& (Join-Path $PSScriptRoot 'local-postgres.ps1') -Action Start
if ($LASTEXITCODE -ne 0) { throw 'Local PostgreSQL could not start.' }
$credential = Import-Clixml -LiteralPath $settings.credentialPath
$env:DATABASE_URL = 'postgres://' + [Uri]::EscapeDataString($credential.UserName) + ':' + [Uri]::EscapeDataString($credential.GetNetworkCredential().Password) + '@127.0.0.1:55432/' + $settings.database + '?sslmode=disable'
$backupCredential = Import-Clixml -LiteralPath $settings.backupCredentialPath
$env:BACKUP_DATABASE_URL = 'postgres://' + [Uri]::EscapeDataString($backupCredential.UserName) + ':' + [Uri]::EscapeDataString($backupCredential.GetNetworkCredential().Password) + '@127.0.0.1:55432/' + $settings.database + '?sslmode=disable'
$env:CC_ALLOW_JSON_STORE = '0'
$env:APP_ENV = 'development'
$env:CC_LOCAL_DISK_ONLY = '1'
$env:CC_LOCAL_DISK = 'D:'
$env:APP_ROOT = $root
$env:HOST = '0.0.0.0'
$env:PORT = '8002'
$env:DATA_DIR = Join-Path $root 'backend\data'
$env:STORAGE_DIR = Join-Path $root 'backend\storage'
$env:BACKUP_DIR = Join-Path $root 'backend\backups'
$env:MIGRATIONS_DIR = Join-Path $root 'backend\migrations'
$env:PATH = (Join-Path (Split-Path $root -Parent) 'tools\postgresql-16.15\pgsql\bin') + ';' + $env:PATH
if (-not (Test-Path -LiteralPath $settings.apiBinary)) { throw 'Configured API binary is missing; build and verify it first.' }
Push-Location (Join-Path $root 'backend')
try {
    & $settings.apiBinary
    if ($LASTEXITCODE -ne 0) { throw "API stopped with exit code $LASTEXITCODE" }
} finally {
    Pop-Location
    Remove-Item Env:DATABASE_URL -ErrorAction SilentlyContinue
    Remove-Item Env:BACKUP_DATABASE_URL -ErrorAction SilentlyContinue
}
