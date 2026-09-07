# Phase 9 PostgreSQL Auth

Auth users now have a PostgreSQL repository path behind the same `auth.Repository` interface used by local JSON development.

## Completed

- Added `auth.NewPostgresRepository`.
- Added `000003_auth_repository_mapping` migration.
- Added stable `external_id` mapping for auth users.
- Added persisted password salt and permissions fields.
- Preserved customer portal links through `customer_users`.
- Backend startup selects auth storage by environment:
  - no `DATABASE_URL`: local JSON repository
  - with `DATABASE_URL`: PostgreSQL auth repository

## Current Storage Split

```text
Auth:       JSON repository or PostgreSQL repository
Customers: JSON repository or PostgreSQL repository
Workflow:   JSON repository or PostgreSQL repository
Audit:      in-memory recorder
Files:      designed, not implemented yet
```

## Next Step

Workflow now has a PostgreSQL repository too. Build file upload/compression next so large images and proofs stay out of inline workflow payloads.
