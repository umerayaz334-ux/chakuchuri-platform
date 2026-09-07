# Phase 7 Repository Boundaries

Phase 7 makes the current storage replaceable without changing portal workflows.

## Completed

- `auth.Repository`
  - Loads and saves `auth.UserRecord` rows.
  - Current implementation: file-backed JSON snapshot.
- `customers.Repository`
  - Loads and saves customer account rows.
  - Current implementation: file-backed JSON snapshot.
- `workflow.Repository`
  - Loads and saves `workflow.State`.
  - Current implementation: file-backed JSON snapshot.

## Why This Matters

The services now talk to a storage contract instead of directly knowing file paths.

```text
Before:
service -> direct JSON path -> file writes

Now:
service -> Repository interface -> file-backed repository

Next:
service -> Repository interface -> PostgreSQL repository
```

This means quotations, manufacturing, shipping, payments, products, messages and calls can continue using the same service methods while the storage layer moves to PostgreSQL.

## Next Coding Step

Build the PostgreSQL repository implementations module by module:

1. Customers
2. Auth users
3. Workflow read model
4. Workflow mutations
5. Audit log
6. Files and image metadata

The API routes should remain unchanged during that migration.
