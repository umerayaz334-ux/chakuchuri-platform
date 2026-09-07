#Requires -Version 5.1
<#
.SYNOPSIS
  Deploy application code without overwriting production data.

.DESCRIPTION
  Copies frontend build output, backend binary, and migrations into DestRoot.
  Never copies or deletes data, storage, backups, Postgres folders, or .env files.
#>
param(
  [Parameter(Mandatory = $true)]
  [string]$SourceRoot,

  [Parameter(Mandatory = $true)]
  [string]$DestRoot,

  [switch]$BuildFrontend,
  [switch]$BuildBackend,
  [switch]$WhatIf
)

$ErrorActionPreference = "Stop"

$ProtectedNames = @(
  "data",
  "storage",
  "backups",
  ".runtime",
  "postgres",
  "pgdata",
  "postgres-data",
  ".env",
  ".env.local",
  ".env.production"
)

$ProtectedPatterns = @("*.dump", "*.dev.json", "chakuchuri-*.zip")

function Test-ProtectedName([string]$Name) {
  foreach ($item in $ProtectedNames) {
    if ($Name -ieq $item) { return $true }
  }
  foreach ($pattern in $ProtectedPatterns) {
    if ($Name -like $pattern) { return $true }
  }
  return $false
}

function Assert-NotProtectedPath([string]$Path) {
  $leaf = Split-Path -Leaf $Path
  if (Test-ProtectedName $leaf) {
    throw "Refusing to touch protected path: $Path"
  }
}

$SourceRoot = (Resolve-Path $SourceRoot).Path
if (-not (Test-Path $DestRoot)) {
  if ($WhatIf) {
    Write-Host "Would create $DestRoot"
  } else {
    New-Item -ItemType Directory -Path $DestRoot | Out-Null
  }
}
$DestRoot = (Resolve-Path $DestRoot).Path

if ($SourceRoot -ieq $DestRoot) {
  throw "Destination must be a separate runtime folder, not the source repo."
}

Write-Host "Source: $SourceRoot"
Write-Host "Dest:   $DestRoot"
Write-Host "Protected paths will never be copied or deleted."

if ($BuildFrontend) {
  Push-Location (Join-Path $SourceRoot "frontend")
  try {
    if ($WhatIf) {
      Write-Host "Would run npm run build"
    } else {
      npm run build
    }
  } finally {
    Pop-Location
  }
}

$backendOut = Join-Path $SourceRoot "backend\bin\chakuchuri-api.exe"
if ($BuildBackend) {
  Push-Location (Join-Path $SourceRoot "backend")
  try {
    $binDir = Join-Path $SourceRoot "backend\bin"
    if (-not $WhatIf) {
      New-Item -ItemType Directory -Force -Path $binDir | Out-Null
      go build -o $backendOut ./cmd/api
    } else {
      Write-Host "Would build $backendOut"
    }
  } finally {
    Pop-Location
  }
}

$frontendDist = Join-Path $SourceRoot "frontend\dist"
if (-not (Test-Path $frontendDist)) {
  throw "frontend/dist is missing. Run with -BuildFrontend or build first."
}

$pairs = @(
  @{ From = $frontendDist; To = (Join-Path $DestRoot "frontend") },
  @{ From = (Join-Path $SourceRoot "backend\migrations"); To = (Join-Path $DestRoot "migrations") }
)

foreach ($pair in $pairs) {
  Assert-NotProtectedPath $pair.To
  if ($WhatIf) {
    Write-Host "Would sync $($pair.From) -> $($pair.To)"
    continue
  }
  if (Test-Path $pair.To) {
    Get-ChildItem -Force $pair.To | ForEach-Object {
      if (Test-ProtectedName $_.Name) {
        Write-Host "Skipping protected item in dest: $($_.FullName)"
        return
      }
      Remove-Item -Recurse -Force $_.FullName
    }
  } else {
    New-Item -ItemType Directory -Path $pair.To | Out-Null
  }
  Copy-Item -Path (Join-Path $pair.From "*") -Destination $pair.To -Recurse -Force
}

if (Test-Path $backendOut) {
  $destBin = Join-Path $DestRoot "chakuchuri-api.exe"
  Assert-NotProtectedPath $destBin
  if ($WhatIf) {
    Write-Host "Would copy $backendOut -> $destBin"
  } else {
    Copy-Item -Force $backendOut $destBin
  }
} else {
  Write-Warning "Backend binary not found at $backendOut. Pass -BuildBackend to compile it."
}

# Never place an .env from source into dest.
$sourceEnv = Join-Path $SourceRoot ".env"
if (Test-Path $sourceEnv) {
  Write-Host "Note: source .env was NOT copied. Keep production secrets only on the runtime host."
}

Write-Host ""
Write-Host "Deploy finished (code only)."
Write-Host "Ensure DATABASE_URL, STORAGE_DIR, and BACKUP_OFFSITE_DIR still point at live data."
