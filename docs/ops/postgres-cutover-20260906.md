# Local PostgreSQL cutover: September 6, 2026

## Active runtime

- Web: `http://localhost:5170`; API: `http://localhost:8002`.
- Live database: `chakuchuri_local`, PostgreSQL 16 on `127.0.0.1:55432`.
- `scripts/start-dev-windows.ps1` and `scripts/START-CHAKUCHURI.cmd` use the
  database/binary pointer in `.runtime/local-api.json`. Missing configuration or
  database failure does not start a JSON fallback or silently build other code.
- The API account is `cc_app_local`, without superuser or RLS-bypass privileges.
  `cc_backup_local` can read all rows for backups, but cannot change table data.
  Credentials are DPAPI-protected under `.runtime/postgres-rehearsal`, with access
  restricted to the current Windows user. No database passwords are in argv.
- Source JSON remains as recovery material, not the active store. Never start
  an old binary with `CC_ALLOW_JSON_STORE=1` against it to recover a running
  PostgreSQL system: later SQL changes will not exist in that old snapshot.

The cluster path retains its historical `postgres-rehearsal` name. It now also
hosts the active local database. Do not remove that directory as test output.
This development cluster is not the final production infrastructure.

## Migration evidence

The JSON API was stopped before the final copy. Original files were retained
unchanged in `.runtime/postgres-cutover-20260906/source` and `original-json.zip`.
A separately prepared copy contains 20 recorded repairs:

- Ten displaced duplicate file IDs were assigned unique historical IDs. For each
  collision, the last record keeps the original ID, matching the old JSON lookup.
- Six orphan customer links on file metadata were resolved using unambiguous
  references from existing business records, not customer-name guesses.
- Four active KYC links older than already recorded submissions/verification were
  closed using existing submission timestamps. Customer verification was not
  granted or revoked by the import.

All media bytes were preserved. Import refused nonempty candidates, made a full
safety archive, checked source hashes and reconciled every imported section.
The restricted app account also passed the same reconciliation:

| Records | Count |
| --- | ---: |
| Users / customers | 6 / 6 |
| File metadata | 142 |
| Quotes / orders | 29 / 26 |
| Payments / ledger entries | 36 / 71 |
| Shipments / calls | 8 / 56 |

All 26 sections matched, including email, rates, notices and directory data.
`reconciled.json`, `preparation-report.json` in `prepared`, and
`media-verified.json` contain the local evidence. All 142 source files matched
their recorded lengths/SHA-256, with no missing referenced thumbnails.

## Recovery and live checks

- `scripts/verify-postgres-recovery.ps1 -Database chakuchuri_local` dumped into a
  retained custom-format archive, restored to a uniquely named disposable DB,
  compared the complete public table set and row-content digests, and verified
  all 19 migrations. All 31 tables matched; only the disposable restored DB was
  removed. Run this exact-content drill with the source quiescent.
- The retained dump and `.verified.json` report are under
  `.runtime/postgres-rehearsal/chakuchuri_local_*.dump*`.
- The running API passed the `store=postgres` gate, demo sign-in, paginated
  workspace retrieval, existing photo/thumbnail access and live WebP uploads.
  Payment-proof bytes were preserved. Test uploads were removed after checking.
- A backup requested through `/api/admin/backups` returned `Verified` with source
  `PostgreSQL + file storage`, using the separate read-only backup credential.
- `go test -p 1 ./...` passed with `TEST_POSTGRES_ADMIN_URL` enabled, including
  real concurrent file writes, tenant isolation, single-credit payment retries,
  SQL failure rollback and independent stale-workflow-writer rejection.
- `go test -tags nodynamic ./internal/files` passed as well.

## Disk use

Source `scripts/local-build-env.ps1` before local builds. Go, npm, Flutter,
Gradle and temporary paths are directed to `D:\DevCache`. Both standard launchers
inherit these paths. Existing third-party IDE/OS processes are not reconfigured.

Cleanup removed 280,772,395 bytes (about 268 MiB) of an extracted old APK debug
payload, superseded/duplicate test binaries and completed migration rehearsal
copies. The two candidate databases created for this migration were also removed.
The active APK, live data, original JSON, final migration recovery files and
historical real backups were retained. Older pre-existing databases were not
deleted merely because they are inactive.

## Not production-ready yet

File metadata uses incremental writes, but workflow changes still write a whole
state snapshot. The revision check prevents silent stale overwrites; it does not
provide automatic cross-instance retry/reload or 20,000-user capacity. Some
nonfinancial mutations still ignore persistence errors. Auth, customer and email
repositories also need incremental persistence. Financial email/audit effects
need an atomic durable outbox; sessions need production persistence/security.

Private object storage, durable image jobs/admission, physical deduplication,
independent offsite DB/media recovery and realistic peak-load tests remain open.
ZIP compatibility backups have not been retired. This cutover does not certify
the platform for real-money production traffic or 20,000 concurrent users.
