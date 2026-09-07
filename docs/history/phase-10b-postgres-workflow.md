# Phase 10 PostgreSQL Workflow

Workflow data has a PostgreSQL repository behind the existing `workflow.Repository` interface.

## Completed

- Added `workflow.NewPostgresRepository`.
- Added versioned workflow repository migrations.
- Added stable `external_id` mapping for products, quotations, manufacturing orders, rate sheets, shipping requests, payments, conversations, messages, calls and call signals.
- Retained `app_data` JSON per workflow table so current UI fields round-trip safely while normalized reporting fields remain queryable.
- Added `workflow_state_meta` for sequence continuity across restarts.
- Backend startup selects storage by environment:
  - without `DATABASE_URL`: local JSON repository
  - with `DATABASE_URL`: PostgreSQL repository
- Direct-call migration renames legacy call queue records to `calls` while preserving history.

## Current Storage Split

```text
Auth:       JSON repository or PostgreSQL repository
Customers: JSON repository or PostgreSQL repository
Workflow:  JSON repository or PostgreSQL repository
Audit:     in-memory recorder
Files:     JSON repository or PostgreSQL metadata repository
```

## Data Safety

All schema changes are versioned. Existing external IDs and JSON payloads remain stable so future web and mobile versions can migrate without deleting customer records.
