# Photo storage

New JPEG/PNG photo uploads are eligible for WebP optimization before persistence.
Existing uploads and backup archives are never rewritten by this feature.

| Upload category | Longest side | WebP quality |
| --- | --- | --- |
| Product, quotation photo, directory photo | 2048 px | 82 |
| Chat image | 1600 px | 78 |
| Profile photo | 768 px | 80 |

Owner types: `product_image`, `quotation_photo`, `directory_listing`, `message`,
and `user_profile`. Customer product-photo quote forms on web and Flutter now
use `quotation_photo`; technical drawings retain their separate category.

- Correct JPEG EXIF orientation before removing metadata.
- Resize within the category limit without upscaling or cropping.
- Replace the incoming bytes only when the encoded WebP is smaller.
- Update the downloadable filename, MIME type, dimensions, size and checksum.
- Preserve transparency. Skip animated PNG and already encoded WebP originals.
- Preserve KYC documents, payment proofs, quotation references/drawings, other
  owner types, PDFs and non-image files byte-for-byte.
- Generate a JPEG preview only when it is smaller than the stored image.
  For a converted photo, also require main file plus preview to remain smaller
  than the incoming photo; otherwise omit the preview.
  Without a preview, the existing thumbnail endpoint serves the main file.
- Record incoming `sourceBytes`, stored `byteSize`, and `previewBytes` in file
  metadata. Older records do not have these measurements. These fields survive
  JSON persistence and are included in PostgreSQL's existing `app_data` JSON.

Full decoding is limited to 16 million pixels. Two image-processing operations
may run concurrently per API process. When busy, unsupported, oversized, or
unable to encode, uploads retain their incoming bytes; previews may be absent.
This bounds conversion work, not total HTTP upload concurrency. Optimization
runs synchronously and is not yet a distributed background job system.

Savings vary by source image and upload load. No original plus optimized duplicate
is retained for successfully converted photos. Converted photos cannot be restored
to their original quality; use document upload categories when original fidelity
is required. These category rules are not a substitute for KYC authorization.
The backend preserves the bytes it receives for protected categories; any earlier
phone-side resizing is outside that guarantee. Server conversion saves durable
storage and subsequent downloads, not the initial upload bandwidth.

## Read-only measurements

Run from `backend`:

```powershell
go run ./cmd/storage-audit -uploads-dir storage/uploads
go run ./cmd/storage-audit -uploads-dir storage/uploads -sample-photo "path/to/photo.png" -owner-type product_image
```

The command streams file hashes, reports physical bytes by extension and exact
duplicate content, and can simulate one new photo upload entirely in memory.
It does not delete, rewrite, upload, or change file records. It does not follow
symlink entries. A missing/unreadable file fails the report instead of silently
undercounting. The directory scan is not a transactionally consistent snapshot;
run during an idle period or against an immutable copy for a stable inventory.
It keeps one small hash entry per unique content item, not all image bytes.
Sample images have the same 10 MiB input and 16-million-pixel decode limits as
the upload path. Negative sample `savedBytes` means preview overhead exceeded
savings for an unconverted original.

On September 6, 2026, the local upload directory contained 265 files totalling
69,391,135 bytes. There were 95 unique byte sequences totalling 14,826,826 bytes;
54,564,309 bytes were repeated content. This is not a list of safe-to-delete files:
separate records, permissions, thumbnails and attachments may reference each copy.
It is a newer inventory than the earlier 63.6 MiB backup archive.

A read-only product PNG trial measured 3,245,533 source bytes versus 387,308 WebP
bytes plus 33,423 thumbnail bytes: about 87% less storage for that sample. This
does not predict savings for already-compressed JPEGs or every customer upload.
Duplicate-byte estimates and conversion savings must not simply be added together.

Verification: `go test ./internal/files ./cmd/storage-audit -count=1` covers
stored/served metadata, category limits, preview budgets, persistence of size
metrics, protected categories, orientation, alpha, animated PNG, resource bounds,
upload persistence rollback, and read-only duplicate reporting.
`go test -tags nodynamic ./internal/files ./cmd/storage-audit`
checks the encoder without requiring a system libwebp or C compiler.

Deployment requires a rebuilt API binary and controlled API restart. The new
Flutter quotation-photo category also requires a later APK release; existing
APKs continue to use their old category. Existing chat/product categories benefit
from the new backend without an APK update. No existing upload migration or live
API restart is performed by these tests.

The local API was separately rebuilt and restarted on September 6, 2026, with
PostgreSQL active. Live quotation/chat/product uploads returned WebP and working
authorized thumbnails. A 292,749-byte JPEG became 141,768 bytes plus a 15,262-byte
preview for chat (about 46% less combined storage). The same input used as a
payment proof was downloaded byte-for-byte unchanged. Temporary smoke uploads
were removed afterward. KYC-original preservation also passed the real PostgreSQL
integration tests. See [database cutover](postgres-cutover-20260906.md).

API workflow load testing and database write-safety work remain separate readiness
requirements for 20,000 users. See [high-volume media plan](high-volume-media.md).
