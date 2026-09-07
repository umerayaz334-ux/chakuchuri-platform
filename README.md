# ChakuChuri Platform

Modern B2B service platform for ChakuChuri.pk.

## Tree

```text
├── backend/          Go API, migrations, runtime data folders
├── frontend/         React web portal (Vite)
├── mobile/           Flutter customer app
├── docs/
│   ├── roadmap-progress.md
│   ├── ops/          release + Postgres/backup runbooks
│   └── history/      completed phase notes
├── scripts/          deploy / migrate / verify helpers
├── docker-compose.yml
└── .env.example
```

Runtime data (`backend/data`, `backend/storage`, `backend/backups`) stays local and gitignored. Prefer Postgres + offsite backups for real use.

### Regenerable folders (normal)

These are **not** source. They reappear when you develop/build, and are ignored by git:

| Path | Comes back when |
| --- | --- |
| `frontend/node_modules/` | `npm install` |
| `frontend/dist/` | `npm run build` |
| `mobile/.dart_tool/`, `mobile/build/` | `flutter pub get` / `flutter run` / `flutter build` |

`npm run dev` and `go run ./cmd/api` do **not** need to write `.logs/` or `*.log` files. Those only return if a terminal command redirects output into the repo.

Wipe regenerable junk anytime (keeps uploads/backups/data):

```powershell
.\scripts\clean-dev.ps1
```

## Local Ports

- Frontend: http://localhost:5170
- Backend: http://localhost:8002
- Backend health: http://localhost:8002/health

## Current Phase

The platform is in active build across Phases 3-12. The Go backend, React web portal, workflow APIs, file metadata/storage layer, WebSocket call signaling and the first Flutter mobile source foundation are in place.

Available now:

- Go backend module structure
- Shared API response and error format
- Request logging and panic recovery
- Auth/session login and signup
- User and access management for admin, manager, staff and customer roles
- Customer account onboarding
- Audit event recorder
- PostgreSQL foundation schema migration
- PostgreSQL migration runner with `up`, `down` and `status`
- React public entry, customer portal and admin portal
- Frontend portal code split into clean platform modules
- Workflow APIs for quotations, manufacturing, shipping, payments, products, messages and calls
- Messenger-style chat and one-active-call queue behavior
- WebSocket call signaling with HTTP polling fallback
- Local durable snapshots for auth users, customers, workflow data and file metadata
- File upload UI wiring for quote drawings, product images and payment screenshots
- Repository boundaries for auth, customers, workflow and file storage
- PostgreSQL auth, customer, workflow and file metadata repositories selected automatically when `DATABASE_URL` is set
- Auth/customer/workflow/file migration mapping with stable `external_id` fields and portal display data
- Flutter mobile folder with Dart API models/client and customer screens for dashboard, quotes, orders, shipping, payments and messages/calls

## Active Auth Endpoints

- `POST /api/auth/login`
- `GET /api/auth/me`
- `POST /api/auth/signup`
- `GET /api/auth/users/options`
- `GET /api/auth/users`
- `POST /api/auth/users`
- `POST /api/auth/users/{id}`
- `POST /api/auth/users/{id}/delete`

## Active File Endpoints

- `POST /api/files`
- `GET /api/files/{id}`
- `GET /api/files/{id}/content`
- `GET /api/files/{id}/thumbnail`

## Active Workflow Endpoints

- `GET /api/workspace`
- `POST /api/workflow/quotes`
- `POST /api/workflow/ratesheets`
- `POST /api/workflow/ratesheets/import`
- `POST /api/workflow/quotes/{id}/price`
- `POST /api/workflow/quotes/{id}/accept`
- `POST /api/workflow/quotes/{id}/reject`
- `POST /api/workflow/manufacturing/{id}/stage`
- `POST /api/workflow/shipping`
- `POST /api/workflow/payments`
- `POST /api/workflow/payments/{id}/confirm`
- `POST /api/workflow/products`
- `POST /api/workflow/messages`
- `POST /api/workflow/calls`
- `GET /api/workflow/calls/config`
- `GET /api/workflow/calls/{id}/signals`
- `POST /api/workflow/calls/{id}/signals`
- `POST /api/workflow/calls/{id}/heartbeat`
- `POST /api/workflow/calls/{id}/end`
- `POST /api/workflow/calls/{id}/status`
- `GET /api/realtime/calls/{id}`

## Local Data

PostgreSQL is the real store. Set `DATABASE_URL` (see `.env.example` and `docker compose up -d`).

JSON files under `backend/data` are only for explicit local experiments with `CC_ALLOW_JSON_STORE=1`. Production refuses that mode.

Durable paths should live outside the deploy folder:

- Postgres data (`POSTGRES_DATA_DIR`)
- Uploaded files (`STORAGE_DIR`)
- Offsite backups (`BACKUP_OFFSITE_DIR` on another disk / NAS / cloud sync)

See `docs/ops/postgres-backups.md` and `docs/ops/release-checklist.md`.

## Database Migrations

Production migrations use an administrator connection, never the restricted API account:

```powershell
$env:DATABASE_URL="postgres://migration-admin:password@localhost:5432/chakuchuri?sslmode=disable"
Set-Location backend
go run ./cmd/migrate status
go run ./cmd/migrate up
```

For the D: local PostgreSQL setup, use the dedicated script. It starts the local server and uses its administrator only for the schema operation:

```powershell
.\scripts\migrate-local.ps1 -Action status
.\scripts\migrate-local.ps1 -Action up
```
## Safe release

Always: **backup → migrate → deploy → verify**. Never overwrite production data when copying a new build.

```powershell
.\scripts\release.ps1 -Step checklist
.\scripts\release.ps1 -Step all -DestRoot "D:\ChakuChuriRuntime\app" -BuildFrontend -BuildBackend
```

`scripts/deploy-app.ps1` copies code only and refuses to touch `data`, `storage`, `backups`, Postgres folders, or `.env`.

See `docs/history/phase-05-database-migrations.md` for rollback and safety rules.

## Development

Backend:

```powershell
cd backend
go run ./cmd/api
```

Frontend:

```powershell
cd frontend
npm run dev
```

Mobile source:

```powershell
cd mobile
"D:\Factory Software\tools\flutter\bin\flutter.bat" pub get
"D:\Factory Software\tools\flutter\bin\flutter.bat" run
```

Verification:

```powershell
cd backend
go test ./...

cd ../frontend
npm run build
```
