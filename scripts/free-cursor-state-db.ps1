# Run this AFTER fully quitting Cursor (check Task Manager: no Cursor.exe).
# Right-click -> Run with PowerShell, or paste into a new PowerShell window.

$ErrorActionPreference = 'SilentlyContinue'
function FreeGB { [math]::Round((Get-PSDrive C).Free/1GB, 2) }
Write-Host "Free BEFORE: $(FreeGB) GB"

# Make sure Cursor is not running
$cursor = Get-Process -Name 'Cursor','cursor' -ErrorAction SilentlyContinue
if ($cursor) {
  Write-Host "Cursor is still running. Closing it..."
  $cursor | Stop-Process -Force
  Start-Sleep -Seconds 3
}

$dbDir = Join-Path $env:APPDATA 'Cursor\User\globalStorage'
foreach ($name in @(
  'state.vscdb',
  'state.vscdb-wal',
  'state.vscdb-shm',
  'state.vscdb.backup'
)) {
  $path = Join-Path $dbDir $name
  if (Test-Path $path) {
    $mb = [math]::Round((Get-Item $path).Length/1MB, 1)
    Remove-Item -LiteralPath $path -Force
    Write-Host "Deleted $name ($mb MB)"
  }
}

Remove-Item -LiteralPath (Join-Path $env:APPDATA 'Cursor\snapshots') -Recurse -Force -ErrorAction SilentlyContinue
Remove-Item -LiteralPath (Join-Path $env:LOCALAPPDATA 'Temp') -Recurse -Force -ErrorAction SilentlyContinue
Get-ChildItem (Join-Path $env:LOCALAPPDATA 'Temp') -Force -ErrorAction SilentlyContinue | Remove-Item -Recurse -Force

Clear-RecycleBin -DriveLetter C -Force -ErrorAction SilentlyContinue
Write-Host "Free AFTER: $(FreeGB) GB"
Write-Host "Done. You can open Cursor again."
Pause
