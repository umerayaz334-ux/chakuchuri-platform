# Phase 6 Storage Boundaries

Phase 6 starts separating business workflows from storage mechanics.

## Completed In This Step

- Added `internal/platform/snapshot`.
- Auth, customers and workflow snapshots now use one shared storage helper.
- Snapshot writes are atomic:
  - write a temporary file
  - move current primary snapshot to `.bak`
  - promote the new snapshot
  - restore backup if promotion fails
- Snapshot loading can fall back to `.bak` when the primary JSON file is damaged.
- Snapshot tests cover backup creation and backup recovery.

## Why This Matters

The current app still runs on local JSON files for fast development, but the business services no longer need custom file-write logic in every module. That makes the next PostgreSQL step cleaner because storage can be replaced without redesigning quotations, manufacturing, shipping or payments.

## Current Storage Flow

```text
API route
  -> domain service
  -> in-memory state
  -> shared snapshot helper
  -> backend/data/*.dev.json + .bak
```

## Next Storage Flow

```text
API route
  -> domain service
  -> repository interface
  -> PostgreSQL implementation
```

## Next Coding Step

Add repository interfaces around each service:

- `auth.Repository`
- `customers.Repository`
- `workflow.Repository`
- `audit.Repository`
- `files.Repository`

Then implement PostgreSQL repositories behind those interfaces.
