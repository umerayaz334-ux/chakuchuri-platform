# Phase 5 Database Migrations

Phase 5 starts the PostgreSQL migration path without removing the local JSON snapshots yet.

## What Is Ready

- `backend/migrations`: versioned SQL migration files.
- `internal/platform/migrations`: migration discovery, status, apply and rollback logic.
- `cmd/migrate`: command line runner for PostgreSQL schema changes.
- `schema_migrations`: PostgreSQL ledger table created automatically by the runner.

## Commands

Set `DATABASE_URL` first:

```powershell
$env:DATABASE_URL="postgres://user:password@localhost:5432/chakuchuri?sslmode=disable"
```

Check status:

```powershell
go run ./cmd/migrate status
```

Apply all pending migrations:

```powershell
go run ./cmd/migrate up
```

Rollback the latest migration:

```powershell
go run ./cmd/migrate down
```

Rollback more than one migration:

```powershell
go run ./cmd/migrate -steps 2 down
```

## Safety Rules

- Never edit an already-applied migration in production.
- Add a new numbered migration for every schema change.
- Every `.up.sql` file must have a matching `.down.sql` file.
- PostgreSQL is the live store; do not point production at JSON files.
- Follow `docs/ops/release-checklist.md`: backup → migrate → deploy → verify.
- Deploy scripts must never overwrite Postgres data, `storage`, backups, or `.env`.

## Next Implementation

Repository interfaces are in place for auth, customers, workflow, email and files. Use `DATABASE_URL` for all real environments.
