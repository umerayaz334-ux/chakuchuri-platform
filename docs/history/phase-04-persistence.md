# Phase 4 Persistence

Phase 4 adds durable local development storage before the PostgreSQL repository layer.

## Snapshot Files

The backend writes versioned JSON snapshots under `backend/data`:

- `auth.users.dev.json`: users, password salts and password hashes
- `customers.dev.json`: customer accounts and service access
- `workflow.dev.json`: products, quotations, manufacturing orders, rate sheets, shipping, payments, conversations and calls

These files are for local development durability. They are ignored by git because they contain live customer/account data.

## Safety Rules

- Every workflow mutation saves immediately after the action.
- Auth and customer creation save immediately after signup.
- Login updates `lastOnline` and persists it.
- The workspace endpoint requires a valid session.
- Snapshot files use a `version` field so future migrations can transform them safely.

## Next Step

Replace the file-backed snapshots with PostgreSQL repositories while keeping the same service APIs:

- `auth.Repository`
- `customers.Repository`
- `workflow.Repository`
- background file/image compression jobs
- database-backed audit log
- data export/import command for version upgrades
