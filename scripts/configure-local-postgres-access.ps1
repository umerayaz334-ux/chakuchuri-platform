#Requires -Version 5.1
param([Parameter(Mandatory=$true)][ValidatePattern('^chakuchuri_[a-z0-9_]+$')][string]$Database)
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'local-build-env.ps1')
$root = Split-Path $PSScriptRoot -Parent
$runtime = Join-Path $root '.runtime\postgres-rehearsal'
$psql = Join-Path (Split-Path $root -Parent) 'tools\postgresql-16.15\pgsql\bin\psql.exe'
$admin = Import-Clixml -LiteralPath (Join-Path $runtime 'credential.xml')
$previousPassword = $env:PGPASSWORD
$env:PGPASSWORD = $admin.GetNetworkCredential().Password
try {
    foreach ($entry in @(@{role='cc_app_local'; file='app-credential.xml'; options='NOBYPASSRLS'}, @{role='cc_backup_local'; file='backup-credential.xml'; options='BYPASSRLS'})) {
        $path = Join-Path $runtime $entry.file
        $exists = & $psql -X -w -h 127.0.0.1 -p 55432 -U $admin.UserName -d $Database -At -v ON_ERROR_STOP=1 -c "SELECT 1 FROM pg_roles WHERE rolname='$($entry.role)'"
        if ($LASTEXITCODE) { throw 'Role lookup failed.' }
        if ($exists -eq '1' -and -not (Test-Path -LiteralPath $path)) { throw 'Existing role has no local credential; refusing to reset it.' }
        if (-not (Test-Path -LiteralPath $path)) {
            $bytes = New-Object byte[] 32
            $rng = [Security.Cryptography.RandomNumberGenerator]::Create()
            try {$rng.GetBytes($bytes)} finally {$rng.Dispose()}
            $secure = ConvertTo-SecureString ([Convert]::ToBase64String($bytes)) -AsPlainText -Force
            [Management.Automation.PSCredential]::new($entry.role,$secure) | Export-Clixml -LiteralPath $path
        }
        $credential = Import-Clixml -LiteralPath $path
        if ($exists -ne '1') {
            $password = $credential.GetNetworkCredential().Password.Replace("'","''")
            # SQL goes through stdin, never through command-line arguments.
            "CREATE ROLE $($entry.role) LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION $($entry.options) PASSWORD '$password';" | & $psql -X -w -h 127.0.0.1 -p 55432 -U $admin.UserName -d $Database -v ON_ERROR_STOP=1
            if ($LASTEXITCODE) { throw 'Role creation failed.' }
        }
    }
    @"
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
GRANT CONNECT ON DATABASE $Database TO cc_app_local, cc_backup_local;
GRANT USAGE ON SCHEMA public TO cc_app_local, cc_backup_local;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO cc_app_local;
REVOKE INSERT, UPDATE, DELETE ON schema_migrations FROM cc_app_local;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO cc_app_local;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO cc_backup_local;
GRANT SELECT ON ALL SEQUENCES IN SCHEMA public TO cc_backup_local;
ALTER DEFAULT PRIVILEGES FOR ROLE postgres IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO cc_app_local;
ALTER DEFAULT PRIVILEGES FOR ROLE postgres IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO cc_app_local;
ALTER DEFAULT PRIVILEGES FOR ROLE postgres IN SCHEMA public GRANT SELECT ON TABLES TO cc_backup_local;
ALTER DEFAULT PRIVILEGES FOR ROLE postgres IN SCHEMA public GRANT SELECT ON SEQUENCES TO cc_backup_local;
"@ | & $psql -X -w -h 127.0.0.1 -p 55432 -U $admin.UserName -d $Database -v ON_ERROR_STOP=1
    if ($LASTEXITCODE) { throw 'Database grants failed.' }
    Write-Output 'App role: no superuser or RLS bypass. Backup role: read-only, all rows. Credentials stay DPAPI-protected on D:.'
} finally {$env:PGPASSWORD=$previousPassword}
