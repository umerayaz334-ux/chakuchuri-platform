# Phase 11 File Uploads

The backend now has a dedicated file upload module for customer/admin assets.

## Completed

- Added `internal/files`.
- Added `POST /api/files` multipart upload endpoint.
- Added authenticated file metadata and content routes:
  - `GET /api/files/{id}`
  - `GET /api/files/{id}/content`
  - `GET /api/files/{id}/thumbnail`
- Added local storage under `storage/uploads`.
- Added JSON metadata storage for local development.
- Added PostgreSQL metadata storage with `files.external_id`, `thumbnail_storage_key` and `app_data`.
- Added JPEG/PNG thumbnail generation at upload time.
- Wired customer quote drawings, product photos and payment screenshots to `POST /api/files`.
- Wired admin payment proof thumbnails through authenticated blob loading.
- Kept originals while creating compressed previews.

## Intended Uses

- Product photos
- Quotation drawings
- Payment proof screenshots
- Shipping documents
- Rate sheet uploads

## Next Step

Add admin rate sheet management next, including uploaded source file metadata and editable courier/zone/weight rows.
