# High-volume media and backup plan

## Capacity assumptions

20,000 daily users sending 10 photos each means 200,000 new images per day.
Illustrative decimal units, before thumbnails, versions, replicas and backups:

| Average stored photo | Per day | Per 30 days |
| --- | --- | --- |
| 2 MB | 400 GB | 12 TB |
| 200 KB | 40 GB | 1.2 TB |

200 KB is a planning example, not a promised output size. Measure representative
customer photos and peak upload bursts. The daily average of 2.31 images/second
is not a peak-load target. At 200 KB, retaining 14 separate full daily snapshots
would multiply a large portion of that growing media library fourteen times.

## Implemented foundation

- Category-based WebP conversion, dimension limits and smaller previews for new
  uploads, while preserving incoming KYC/proof/drawing bytes.
- Incoming/stored/preview byte metrics on new file records.
- Read-only disk inventory, exact duplicate measurements and sample conversion.
- The local API now runs on PostgreSQL, with per-record file writes and reads,
  tenant-scoped lookup, owner/checksum indexes and concurrent upload tests.
  PostgreSQL startup no longer loads the complete file catalog into memory.
- JSON import reconciled all 26 sections. A real restore matched all 31 tables;
  original uploads passed a 142-record checksum audit. Financial persistence
  failures roll back the covered operations, and stale workflow writers fail
  instead of replacing a newer snapshot. See [cutover evidence](postgres-cutover-20260906.md).

See [photo storage](photo-storage.md) for policy details and verified examples.
These changes do not migrate existing images or establish production capacity.

## Next work, in order

1. Complete incremental persistence beyond files. File uploads already use
   one-record PostgreSQL operations; the bulk file method remains for migration.
   Workflow, auth, customers and email still use whole-state persistence in places.
   Move business mutations to row-level transactions, with durable outbox events
   and idempotent retries; do not treat the transitional workflow revision guard
   as sufficient for many concurrent API replicas.
2. Put private immutable media in S3-compatible object storage. Keep only
   metadata/references in PostgreSQL. Use authorized short-lived downloads or
   signed CDN access; never expose KYC/proof storage as a public image bucket.
   Verify mobile/web thumbnail caching without bypassing tenant permissions.
3. Add durable image-processing jobs and admission limits. Current conversion is
   synchronous, with two local processing slots; busy workers preserve originals
   and do not enqueue a retry. Stream to staging, apply per-user byte/concurrency
   budgets, validate files, persist retryable jobs, and publish optimized content
   atomically. Budget HTTP memory separately from decoded image memory. Keep a
   usable original until the replacement has been verified and committed.
4. Deduplicate physical blobs without merging logical attachments. Use a
   tenant-scoped checksum plus an image-policy version as the deduplication key,
   keeping unique file IDs, names and access rules. Transactionally manage blob
   references and a delayed cleanup process. Test simultaneous identical uploads,
   attachment removal, interrupted writes and cross-tenant access. Do not delete
   duplicate paths from the audit output directly.
5. Back up database and media separately, with a coordinated recovery point.
   Replace daily full-media ZIP copies with an encrypted deduplicated backup
   repository and retained snapshots, plus database backups appropriate to the
   recovery target. Restic is one candidate: it reuses content across snapshots
   instead of adding another complete copy of unchanged files. See its official
   [backup documentation](https://restic.readthedocs.io/en/stable/040_backup.html).
   Use independent offsite storage and protected credentials. Do not point a file
   backup tool at live PostgreSQL data files and assume that is a recoverable DB
   backup. Object-storage versioning alone is not an independent recovery copy.
6. Make retention an explicit business policy. Separate chat/product-photo
   retention from identity, payment and other records. Preview expiry can be
   independent because previews are regenerable. Do not auto-delete customer
   originals, documents, old backups or legally retained records by default.
7. Verify recovery and realistic load. Restore the DB and referenced media into
   an isolated environment, check every restored reference/checksum, and exercise
   login, chat, photos and documents. Test upload bursts, slow connections, retry
   storms, worker crashes and object-storage failures. Record memory, queue age,
   upload/thumbnail latency, error rate, restore time and daily storage cost.

## Release gates

- The active deployment must report `store=postgres`; the local API passed this
  gate after the September 6, 2026 cutover. This is not a production deployment.
- No acknowledged upload or attachment may disappear after a process failure.
- Authorized photos must render in existing web and Flutter clients, and private
  files must remain inaccessible to another customer.
- A complete offsite restore must succeed within the agreed recovery targets.
- Peak-load evidence must cover the expected traffic mix and media sizes, with
  measured resource headroom. Passing unit tests alone cannot certify 20,000 users.

Current full ZIP backups remain unchanged for compatibility. Existing archives
are recovery points, not disposable duplicates. Do not retire them until the new
backup mechanism has passed a restore drill and its retention is approved.

## Private storage choice

No configured cloud bucket or S3 credentials were found locally. Use a private
Cloudflare R2 bucket as the proposed photo/media target, subject to the deployment
region and data-residency decision. Its [S3-compatible API](https://developers.cloudflare.com/r2/api/s3/)
and [short-lived signed URLs](https://developers.cloudflare.com/r2/api/s3/presigned-urls/)
fit the existing authorized file/thumbnail endpoints without public customer files.
This is a proposed integration, not an active bucket; no cloud account was opened
and no customer files were uploaded externally. Location hints are best effort,
not residency guarantees; check [data-location controls](https://developers.cloudflare.com/r2/reference/data-location/)
before choosing storage for identity/payment records. Keep recovery storage
independent of the live bucket and its credentials.
