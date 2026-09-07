#Requires -Version 5.1
param([Parameter(Mandatory=$true)][ValidatePattern('^chakuchuri_[a-z0-9_]+$')][string]$Database)
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'local-build-env.ps1')
$root = Split-Path $PSScriptRoot -Parent
$pg = Join-Path (Split-Path $root -Parent) 'tools\postgresql-16.15\pgsql\bin'
$runtime = Join-Path $root '.runtime\postgres-rehearsal'
$cred = Import-Clixml -LiteralPath (Join-Path $runtime 'credential.xml')
$previousPassword = $env:PGPASSWORD
$env:PGPASSWORD = $cred.GetNetworkCredential().Password
$suffix = [guid]::NewGuid().ToString('N')
$restore = "cc_restore_$suffix"
$dump = Join-Path $runtime "${Database}_$suffix.dump"
$created = $false
$verified = $false

function Query([string]$Name, [string]$SQL) {
    $result = & (Join-Path $pg 'psql.exe') -X -w -h 127.0.0.1 -p 55432 -U $cred.UserName -d $Name -At -v ON_ERROR_STOP=1 -c $SQL
    if ($LASTEXITCODE -ne 0) { throw "Verification query failed in $Name." }
    return $result
}
try {
    Write-Output 'Restore drill requires a quiescent source database. The dump will be retained.'
    & (Join-Path $pg 'pg_dump.exe') -w -h 127.0.0.1 -p 55432 -U $cred.UserName -d $Database -Fc -f $dump
    if ($LASTEXITCODE) { throw 'pg_dump failed.' }
    & (Join-Path $pg 'createdb.exe') -w -h 127.0.0.1 -p 55432 -U $cred.UserName $restore
    if ($LASTEXITCODE) { throw 'createdb failed.' }
    $created = $true
    & (Join-Path $pg 'pg_restore.exe') -w -h 127.0.0.1 -p 55432 -U $cred.UserName -d $restore --exit-on-error $dump
    if ($LASTEXITCODE) { throw 'pg_restore failed.' }
    $tableQuery = "SELECT tablename FROM pg_tables WHERE schemaname='public' ORDER BY tablename"
    $tables = @(Query $Database $tableQuery)
    $restoredTables = @(Query $restore $tableQuery)
    if ($tables.Count -eq 0 -or (($tables -join ',') -ne ($restoredTables -join ','))) { throw 'Restored table set differs.' }
    $checks = @()
    foreach ($table in $tables) {
        if ($table -notmatch '^[a-z_][a-z0-9_]*$') { throw 'Unexpected table identifier.' }
        $sql = "SELECT count(*)::text || ':' || md5(COALESCE(string_agg(to_jsonb(t)::text, E'\n' ORDER BY to_jsonb(t)::text), '')) FROM public.$table t"
        $expected = Query $Database $sql
        $actual = Query $restore $sql
        if ($expected -ne $actual) { throw "Restored data differs in $table (or source changed during drill)." }
        $checks += [ordered]@{table=$table; rowsAndDigest=$actual; match=$true}
    }
    $expectedMigrations = @(Get-ChildItem -LiteralPath (Join-Path $root 'backend\migrations') -Filter '*.up.sql').Count
    $migrations = Query $restore 'SELECT count(*) FROM schema_migrations'
    if ([int]$migrations -ne $expectedMigrations) { throw 'Restored migration count differs from source tree.' }
    $report = [ordered]@{verified=$true; source=$Database; restoredInto=$restore; verifiedAt=[DateTime]::UtcNow.ToString('o'); dump=$dump; dumpBytes=(Get-Item -LiteralPath $dump).Length; sha256=(Get-FileHash -LiteralPath $dump -Algorithm SHA256).Hash; migrations=[int]$migrations; tables=$checks}
    $report | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath ($dump + '.verified.json') -Encoding UTF8
    $verified = $true
    Write-Output "Recovery verified: $($tables.Count) tables, $migrations migrations; retained $dump"
} finally {
    # Only remove the random database created by this invocation, after full verification.
    if ($created -and $verified -and $restore -match '^cc_restore_[a-f0-9]{32}$') {
        & (Join-Path $pg 'dropdb.exe') -w -h 127.0.0.1 -p 55432 -U $cred.UserName $restore
        if ($LASTEXITCODE) { Write-Warning "Could not remove verified rehearsal database $restore." }
    }
    $env:PGPASSWORD = $previousPassword
}
