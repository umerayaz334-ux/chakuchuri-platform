# KYC Architecture

## Purpose

KYC is an isolated, permissioned workflow. Identity data is not part of the general customer-management surface and KYC files cannot use the generic upload endpoint.

## Actors and permissions

- Customers can upload, submit, and view their own editable drafts.
- Internal users need the explicit `kyc.review` permission to see KYC metadata or media, request documents, review documents, or change verification status.
- Owner and Admin roles receive `kyc.review` by default. Other internal roles receive it only when explicitly assigned.
- Customer API responses and non-reviewer staff responses are redacted independently.

## Request lifecycle

Secure document requests use this state machine:

```text
active -> submitted
active -> revoked
active -> expired
```

- Creating a request revokes any earlier active request for that customer.
- Only a SHA-256 token hash is persisted. The raw token is returned once when the link is created.
- Upload and final submission require an active, unexpired request.
- Final submission is one-shot. A submitted link displays a locked receipt and cannot upload again.
- Documents uploaded through a secure link carry the originating request ID.

## Document lifecycle

```text
requested -> draft -> submitted -> accepted
                              \-> rejected -> draft -> submitted
```

- `cnic_front`, `cnic_back`, and `selfie` are mandatory for verification.
- Every extra requested document must also have an accepted upload.
- Verification never auto-accepts a draft or submitted file.
- Submitted and accepted evidence is immutable. Corrections require rejection and a replacement upload.

## API boundaries

Authenticated upload:

```text
POST /api/customers/{customerId}/documents/upload
```

The endpoint validates customer access, KYC permission, requested kind, MIME, size, and ownership. File creation and customer attachment use a compensating transaction: if attachment fails, file metadata and storage are removed.

Public secure-link flow:

```text
GET  /api/public/document-requests/{token}
POST /api/public/document-requests/{token}/upload
POST /api/public/document-requests/{token}/submit
```

Reviewer flow:

```text
POST /api/customers/{customerId}/document-requests
POST /api/customers/{customerId}/documents/{documentId}/review
POST /api/customers/{customerId}/verification
```

Generic `POST /api/files` rejects `ownerType=customer_document` so callers cannot bypass the workflow.

## Persistence

- Migration `000015_customer_kyc_requests` stores token hashes and request lifecycle fields in PostgreSQL with tenant row-level security.
- File rollback has an explicit repository delete operation so a failed attachment cannot reappear after restart.
- Upload success is returned only after file metadata persistence succeeds.
- Development JSON snapshots strip raw request tokens and retain hashes only.

## Upload policy

- Maximum size: 10 MB per file.
- Allowed evidence: JPEG, PNG, WebP, and PDF.
- Both detected MIME and filename extension must match the allowlist.
- Office documents, CSV, archives, and executable formats are rejected.

## Production scale path

The current service boundaries are ready for the next storage implementation, but a 20,000-concurrent-user deployment still requires:

1. Move customer document rows out of customer `portal_data` into normalized PostgreSQL tables with optimistic versioning.
2. Replace local disk with private S3-compatible object storage and short-lived authorized download URLs.
3. Stream uploads directly to object storage instead of buffering whole files in API memory.
4. Add asynchronous malware scanning, image normalization, and quarantine states.
5. Publish KYC mutations through Redis so every API instance invalidates the correct customer view.
6. Add request and upload rate limits, idempotency keys, storage quotas, retention rules, and audit export.
7. Run PostgreSQL integration, multi-instance, recovery, and load tests before production launch.

Until those items are complete, the development JSON/local-storage mode must not be described as production-ready.
