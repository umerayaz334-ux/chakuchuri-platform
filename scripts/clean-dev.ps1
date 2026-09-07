#Requires -Version 5.1
<#
.SYNOPSIS
  Remove regenerable build/cache/log junk. Does not delete source, .env, or uploads/backups/data by default.
#>
param(
  [switch]$IncludeNodeModules,
  [switch]$IncludeRuntimeData
)

$ErrorActionPreference = "Stop"
$root = Resolve-Path (Join-Path $PSScriptRoot "..")

$paths = @(
  ".logs",
  "tmp",
  "frontend\tmp",
  "frontend\dist",
  "frontend\frontend.err.log",
  "frontend\frontend.out.log",
  "frontend\vite-5170.stderr.log",
  "frontend\vite-5170.stdout.log",
  "backend\backend.err.log",
  "backend\backend.out.log",
  "backend\bin",
  "mobile\build",
  "mobile\.dart_tool",
  "mobile\.idea",
  "mobile\.flutter-plugins",
  "mobile\.flutter-plugins-dependencies",
  "mobile\android\.gradle",
  "mobile\android\build",
  "mobile\android\app\build",
  "mobile\android\.kotlin"
)

if ($IncludeNodeModules) {
  $paths += "frontend\node_modules"
}

Write-Host "Cleaning regenerable junk under $root"
foreach ($rel in $paths) {
  $full = Join-Path $root $rel
  if (Test-Path $full) {
    Remove-Item -Recurse -Force $full
    Write-Host "  removed $rel"
  }
}

Get-ChildItem (Join-Path $root "frontend") -Filter "*.log" -ErrorAction SilentlyContinue | ForEach-Object {
  Remove-Item -Force $_.FullName
  Write-Host "  removed frontend\$($_.Name)"
}
Get-ChildItem (Join-Path $root "backend") -Filter "*.log" -ErrorAction SilentlyContinue | ForEach-Object {
  Remove-Item -Force $_.FullName
  Write-Host "  removed backend\$($_.Name)"
}
Get-ChildItem (Join-Path $root "backend\data") -Filter "*.bak" -ErrorAction SilentlyContinue | ForEach-Object {
  Remove-Item -Force $_.FullName
  Write-Host "  removed backend\data\$($_.Name)"
}

if ($IncludeRuntimeData) {
  Write-Host "WARNING: removing local runtime data (uploads, backup zips, JSON snapshots)"
  Get-ChildItem (Join-Path $root "backend\backups") -Filter "chakuchuri-*.zip" -ErrorAction SilentlyContinue | Remove-Item -Force
  Get-ChildItem (Join-Path $root "backend\data") -Filter "*.dev.json" -ErrorAction SilentlyContinue | Remove-Item -Force
  if (Test-Path (Join-Path $root "backend\storage\uploads")) {
    Get-ChildItem (Join-Path $root "backend\storage\uploads") -Recurse -Force -ErrorAction SilentlyContinue | Remove-Item -Recurse -Force
  }
}

Write-Host ""
Write-Host "Done. These come back only when you develop/build again:"
Write-Host "  npm install / npm run build  -> frontend/node_modules, frontend/dist"
Write-Host "  flutter pub get / flutter run -> mobile/.dart_tool, mobile/build"
Write-Host "  go build                     -> backend/bin (if you build into it)"
Write-Host "Logs only reappear if a command redirects output into *.log or .logs/"
