# Phase 8 PostgreSQL Customers

Phase 8 starts moving live modules from file-backed repositories toward PostgreSQL.

## Completed

- Added `customers.NewPostgresRepository`.
- Added `000002_customer_repository_mapping` migration.
- Added `external_id` mapping for tenants and customers.
- Added `portal_data` for customer portal display fields.
- Backend startup now selects customer storage by environment:
  - no `DATABASE_URL`: local JSON repository
  - with `DATABASE_URL`: PostgreSQL customer repository

## Why External IDs Exist

The app already has stable readable IDs such as:

- `tenant_chakuchuri`
- `cust_abc_export`
- `cust_signup_...`

PostgreSQL keeps internal UUID primary keys for foreign keys and indexes. `external_id` preserves the readable API/data migration IDs so future imports, exports and mobile app sync do not break.

## Current Storage Split

```text
Auth:       JSON repository or PostgreSQL repository
Customers: JSON repository or PostgreSQL repository
Workflow:   JSON repository
Audit:      in-memory recorder
Files:      designed, not implemented yet
```

## Next Step

Auth users now have a PostgreSQL repository too. Move workflow data next because it touches quotations, manufacturing, shipping, payments, messages and calls together.
