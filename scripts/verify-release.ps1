#Requires -Version 5.1
param(
  [Parameter(Mandatory = $true)]
  [string]$ApiBase
)

$ErrorActionPreference = "Stop"
$ApiBase = $ApiBase.TrimEnd("/")

Write-Host "Checking $ApiBase/health ..."
$response = Invoke-RestMethod -Uri "$ApiBase/health" -Method Get
$response | ConvertTo-Json -Depth 6

$health = $response.data
if ($response.ok -ne $true -or $null -eq $health -or $health.status -ne "ok") {
  throw "Health status is not ok."
}
if ($health.store -ne "postgres") {
  throw "Expected store=postgres, got '$($health.store)'. Missing store or JSON is not accepted."
}

Write-Host "Verify passed."
