# Phase 2 Foundation

The foundation keeps the project understandable as product features grow.

## Backend Modules

- `internal/platform/config`: environment and port configuration
- `internal/platform/httpx`: response envelopes, errors, JSON parsing, CORS, request logs and panic recovery
- `internal/auth`: login, sessions, roles, permissions and page access
- `internal/customers`: customer accounts, services and online state
- `internal/audit`: append-only audit events
- `internal/foundation`: live module map for the UI
- `internal/blueprint`: UI and data blueprint
- `internal/workflow`: quotations, manufacturing, shipping, payments, products, messages and calls
- `internal/files`: protected uploads, image processing and metadata
- `internal/backups`: snapshots and restore verification

## API Shape

Successful responses use:

```json
{
  "ok": true,
  "requestId": "20260901064133.563987400",
  "data": {}
}
```

Errors use:

```json
{
  "ok": false,
  "requestId": "20260901064133.563987400",
  "error": {
    "code": "invalid_credentials",
    "message": "Email or password is incorrect."
  }
}
```

## Foundation Endpoints

- `GET /health`
- `GET /api/blueprint`
- `GET /api/foundation`
- `POST /api/auth/login`
- `GET /api/auth/me`
- `GET /api/customers`
- `GET /api/audit/recent`

## Data Foundation

Versioned migrations cover tenants, users, customers, audit events, files, products, quotations, manufacturing orders and timeline events, shipping rates and requests, ledger entries, payment confirmations, conversations, messages, calls and call signals.

Tenant indexes and PostgreSQL row-level security policies protect customer-owned data.
