# Operations: PostgreSQL, backups, and safe deploys

## Live store

PostgreSQL is the real database. The API refuses to start without `DATABASE_URL` unless you explicitly set `CC_ALLOW_JSON_STORE=1` for local experiments. Production never allows the JSON store.

Repositories wired when Postgres is connected:

- Auth users and sessions
- Customers
- Workflow (quotes, orders, shipping, payments, messages, calls)
- Email automation
- File metadata (blobs stay on disk under `STORAGE_DIR`)

## Recommended layout

Keep durable data off the deploy folder:

| Path | Purpose |
| --- | --- |
| `POSTGRES_DATA_DIR` on another disk | Postgres files |
| `STORAGE_DIR` outside app | Uploaded images/proofs |
| `BACKUP_DIR` local cache | Recent zip archives for UI download |
| `BACKUP_OFFSITE_DIR` other disk / NAS / cloud sync | Durable recovery copies |

Example (Windows):

```
E:\ChakuChuriData\postgres
E:\ChakuChuriData\storage
E:\ChakuChuriBackups
D:\ChakuChuriRuntime\app          ← deploy code here only
```

## Start Postgres locally

```powershell
copy .env.example .env
# edit POSTGRES_PASSWORD and DATABASE_URL
docker compose up -d
cd backend
go run ./cmd/migrate up
go run ./cmd/api
```

## Backups

Each backup zip contains:

- `database/postgresql.dump` when `DATABASE_URL` is set (`pg_dump` custom format)
- `storage/...` uploaded files
- `manifest.json` with SHA-256 entries

After a verified local zip is written, the manager copies it to `BACKUP_OFFSITE_DIR`. Production requires that directory and rejects an offsite path inside the app folder. On Windows production it also rejects the same drive letter unless `BACKUP_ALLOW_SAME_VOLUME=1`.

## Safe deploy

`scripts/deploy-app.ps1` copies only application artifacts. Protected names are never copied or deleted:

`data`, `storage`, `backups`, `.runtime`, `postgres`, `pgdata`, `.env`, `*.dump`

See `docs/ops/release-checklist.md` for the backup → migrate → deploy → verify order.
