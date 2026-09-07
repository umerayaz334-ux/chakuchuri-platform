param(
    [ValidateSet('Start', 'Stop', 'Status')][string]$Action = 'Status'
)

$ErrorActionPreference = 'Stop'
$project = Split-Path $PSScriptRoot -Parent
$bin = Join-Path (Split-Path $project -Parent) 'tools\postgresql-16.15\pgsql\bin'
$runtime = Join-Path $project '.runtime\postgres-rehearsal'
$data = Join-Path $runtime 'data'
$credentialPath = Join-Path $runtime 'credential.xml'
$pgCtl = Join-Path $bin 'pg_ctl.exe'
if (-not (Test-Path -LiteralPath $pgCtl)) { throw "PostgreSQL binaries missing: $bin" }

if ($Action -eq 'Status' -or $Action -eq 'Stop') {
    if (-not (Test-Path -LiteralPath (Join-Path $data 'PG_VERSION'))) {
        Write-Output 'Local rehearsal cluster is not initialized.'
        exit 0
    }
    if ($Action -eq 'Stop') { & $pgCtl stop -D $data -m fast -w }
    else { & $pgCtl status -D $data }
    exit $LASTEXITCODE
}

New-Item -ItemType Directory -Path $runtime -Force | Out-Null
# Keep the generated credential and cluster accessible only to this Windows user.
$sid = [Security.Principal.WindowsIdentity]::GetCurrent().User
$acl = [IO.Directory]::GetAccessControl($runtime, [Security.AccessControl.AccessControlSections]::Access)
$acl.SetAccessRuleProtection($true, $false)
foreach ($rule in @($acl.Access)) { $acl.RemoveAccessRuleSpecific($rule) }
$acl.AddAccessRule([Security.AccessControl.FileSystemAccessRule]::new(
    $sid, 'FullControl', 'ContainerInherit,ObjectInherit', 'None', 'Allow'))
[IO.Directory]::SetAccessControl($runtime, $acl)

if (-not (Test-Path -LiteralPath (Join-Path $data 'PG_VERSION'))) {
    if (Test-Path -LiteralPath $data) { throw 'Incomplete cluster exists; inspect it before retrying. No data was deleted.' }
    if (-not (Test-Path -LiteralPath $credentialPath)) {
        $random = New-Object byte[] 32
        $rng = [Security.Cryptography.RandomNumberGenerator]::Create()
        try { $rng.GetBytes($random) } finally { $rng.Dispose() }
        $secure = ConvertTo-SecureString ([Convert]::ToBase64String($random)) -AsPlainText -Force
        [Management.Automation.PSCredential]::new('postgres', $secure) | Export-Clixml -LiteralPath $credentialPath
    }
    $credential = Import-Clixml -LiteralPath $credentialPath
    $pwFile = Join-Path $runtime 'init-password.tmp'
    try {
        [IO.File]::WriteAllText($pwFile, $credential.GetNetworkCredential().Password, [Text.Encoding]::ASCII)
        & (Join-Path $bin 'initdb.exe') -D $data -U postgres --auth=scram-sha-256 --encoding=UTF8 --locale=C "--pwfile=$pwFile"
        if ($LASTEXITCODE -ne 0) { throw 'PostgreSQL initialization failed.' }
    } finally {
        if (Test-Path -LiteralPath $pwFile) { Remove-Item -LiteralPath $pwFile }
    }
}

& $pgCtl status -D $data *> $null
if ($LASTEXITCODE -eq 0) {
    Write-Output 'Local PostgreSQL is already running.'
    exit 0
}
$options = '-h 127.0.0.1 -p 55432 -c max_connections=50 -c shared_buffers=128MB'
# Detach the persistent server so it survives the command runner without opening a window.
$arguments = "start -D `"$data`" -l `"$(Join-Path $runtime 'server.log')`" -o `"$options`" -w"
$process = Start-Process -FilePath $pgCtl -ArgumentList $arguments -WindowStyle Hidden -PassThru
$process.WaitForExit()
if ($process.ExitCode -ne 0) { throw "PostgreSQL start failed; check $runtime\server.log" }
Write-Output 'Local PostgreSQL is running at 127.0.0.1:55432.'
