#Requires -Version 5.1
<#
.SYNOPSIS
  Run release steps in order: backup reminder → migrate → deploy → verify.
#>
param(
  [Parameter(Mandatory = $true)]
  [ValidateSet("checklist", "migrate", "deploy", "verify", "all")]
  [string]$Step,

  [string]$SourceRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path,
  [string]$DestRoot = "",
  [string]$ApiBase = "http://127.0.0.1:8002",
  [switch]$BuildFrontend,
  [switch]$BuildBackend
)

$ErrorActionPreference = "Stop"

function Show-Checklist {
  $path = Join-Path $SourceRoot "docs\ops\release-checklist.md"
  Write-Host "Open and follow: $path"
  if (Test-Path $path) {
    Get-Content $path | Select-Object -First 40
  }
}

function Invoke-Migrate {
  if (-not $env:DATABASE_URL) {
    throw "DATABASE_URL must be set before migrate."
  }
  Push-Location (Join-Path $SourceRoot "backend")
  try {
    Write-Host "Migration status (before):"
    go run ./cmd/migrate status
    Write-Host "Applying migrations..."
    go run ./cmd/migrate up
    Write-Host "Migration status (after):"
    go run ./cmd/migrate status
  } finally {
    Pop-Location
  }
}

function Invoke-Deploy {
  if (-not $DestRoot) {
    throw "DestRoot is required for deploy (a runtime folder separate from the repo)."
  }
  & (Join-Path $PSScriptRoot "deploy-app.ps1") `
    -SourceRoot $SourceRoot `
    -DestRoot $DestRoot `
    -BuildFrontend:$BuildFrontend `
    -BuildBackend:$BuildBackend
}

function Invoke-Verify {
  & (Join-Path $PSScriptRoot "verify-release.ps1") -ApiBase $ApiBase
}

switch ($Step) {
  "checklist" { Show-Checklist }
  "migrate" { Invoke-Migrate }
  "deploy" { Invoke-Deploy }
  "verify" { Invoke-Verify }
  "all" {
    Write-Host "=== 1) Backup ==="
    Write-Host "Create a verified backup from Admin → Settings → Backups before continuing."
    Write-Host "Confirm the zip exists in BACKUP_OFFSITE_DIR, then press Enter."
    Read-Host | Out-Null
    Write-Host "=== 2) Migrate ==="
    Invoke-Migrate
    Write-Host "=== 3) Deploy ==="
    Invoke-Deploy
    Write-Host "=== 4) Verify ==="
    Write-Host "Restart the API against the new build if needed, then press Enter to verify."
    Read-Host | Out-Null
    Invoke-Verify
  }
}
