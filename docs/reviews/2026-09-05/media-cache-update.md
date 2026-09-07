# Private media delivery

- Ordinary file responses use private, no-cache and a weak ETag based on storage key, size and modification time.
- Reuse requires server revalidation after the existing session and file-access checks. Shared caches must not store these responses.
- KYC originals/thumbnails and authorization errors use no-store.
- Simultaneous web blob requests share one pending download per session and path. Completed blobs are not retained by an application cache.
- Pending request bookkeeping is capped at 64 entries; additional requests remain independent.
- Browser-managed revalidation can reduce transferred bytes, but does not eliminate HTTP requests.

Tests cover 304 responses, changed-file validators, KYC policy, unauthorized conditional requests, session separation, deduplication and failure retry.

Still pending: mobile thumbnail sizing/caching, actual device request measurements, scoped realtime fanout, transactional database storage and production load testing. No APK rebuild.
