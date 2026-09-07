# Release checklist — backup → migrate → deploy → verify
#
# Goal: ship a new build without ever replacing production Postgres data,
# uploaded files, or backup archives.

## Before you start

- [ ] Confirm `APP_ENV=production`
- [ ] Confirm `DATABASE_URL` points at the live Postgres instance
- [ ] Confirm `BACKUP_OFFSITE_DIR` is on **another disk / NAS / cloud-synced path**
- [ ] Confirm `STORAGE_DIR` and Postgres data are **outside** the folder you deploy into
- [ ] Confirm `pg_dump` is on PATH (used by the backup manager)

## 1. Backup

Create a verified archive and confirm the offsite copy exists:

```powershell
cd backend
$env:DATABASE_URL = "postgres://..."
# Prefer creating from the running API (Settings → Backups), or:
# curl -X POST -H "Authorization: Bearer <token>" http://127.0.0.1:8002/api/admin/backups
```

Checklist:

- [ ] Backup status is **Verified**
- [ ] Archive appears under `BACKUP_DIR`
- [ ] Same file appears under `BACKUP_OFFSITE_DIR`
- [ ] Offsite disk is not the laptop/app drive (or cloud sync is confirmed)

## 2. Migrate

Apply only pending schema migrations. Never edit old migration files.

```powershell
cd backend
go run ./cmd/migrate status
go run ./cmd/migrate up
go run ./cmd/migrate status
```

Checklist:

- [ ] `status` shows expected versions applied
- [ ] No manual SQL against production unless reviewed and backed up

## 3. Deploy (code only)

Use the safe deploy script. It copies binaries/static assets and **refuses** to overwrite data folders.

```powershell
.\scripts\deploy-app.ps1 `
  -SourceRoot "D:\Factory Software\ChakuChuri Platform" `
  -DestRoot "D:\ChakuChuriRuntime\app" `
  -BuildFrontend
```

Checklist:

- [ ] Deploy target does **not** contain Postgres data, `storage`, or `backups`
- [ ] Script reported skipped protected paths
- [ ] Old `STORAGE_DIR` / `DATABASE_URL` / `BACKUP_OFFSITE_DIR` env vars still point at the live data

## 4. Verify

```powershell
.\scripts\verify-release.ps1 -ApiBase "http://127.0.0.1:8002"
```

Checklist:

- [ ] `/health` returns `"status":"ok"` and `"store":"postgres"`
- [ ] Admin and customer can sign in
- [ ] Open orders / quotes still present (no empty DB)
- [ ] Upload a small file and confirm it lands under the existing `STORAGE_DIR`
- [ ] Create a backup and confirm offsite copy

## Never do

- Do not copy `backend/data`, `.runtime`, Postgres volumes, `storage`, or `backups` from a laptop build over production
- Do not replace production `.env` with a development `.env`
- Do not run `migrate down` on production unless recovering from a known bad migration with a fresh backup
- Do not set `CC_ALLOW_JSON_STORE=1` in production
