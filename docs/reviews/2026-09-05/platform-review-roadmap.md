# Platform review and implementation roadmap

Date: 2026-09-05
Scope: Cursor handoff, current Go/Vite/Flutter source, available local logs, and APK identity.
Status: Review complete; roadmap proposed. No production capacity certification.

## Implementation update: first safety milestone

Completed after the review:

- Fixed the undefined KYC count references; TypeScript and the production frontend build now pass.
- Notice socket payloads now follow audience and active-state checks. Unauthorized recipients receive an ID-only withdrawal, never notice text or recipient lists. Snapshot responses redact recipient lists for non-managers.
- Non-manager staff visibility now follows customer scope for selected notices. Inactive featured items are withdrawn from non-manager clients.
- Notice and featured create/update/delete now save before publishing. Failed saves restore those collections in memory and return HTTP 503 storage_unavailable.
- Added normal backend regression tests for all six failed-write/retry paths, recipient privacy, withdrawal, and HTTP error handling. Added two frontend tests for existing App Home event handlers.
- Full backend suite and frontend event tests pass. Frontend build retains its existing large-bundle warning.
- Rebuilt the API and started it on 8002. Health reports development JSON mode; the frontend at 5170 reaches the backend successfully.
- No APK rebuild or production database migration.

Remaining: save-failure propagation for other workflow modules, startup recovery safety, live socket revocation, presence fetch reduction, fanout/cursor design, caches, transactional repositories, and load testing. These are not marked complete by this milestone.

The historical findings below describe the reviewed baseline. The review overlay's selected-notice check now allows ID-only removal events; the global fanout probe remains expected to fail until the cursor/fanout milestone.

## What was verified

- Existing Go tests pass for workflow, files, auth, and customers.
- Four isolated review probes fail as expected, demonstrating selected-notice leakage, inactive-notice leakage, success after storage failure, and unnecessary global event fanout.
- Web TypeScript check fails: customer-documents.tsx lines 603 and 614 reference undefined acceptedCount; acceptedDocsCount is defined at line 388.
- Flutter analyze finds 14 informational lint/deprecation findings, with no analyzer errors or warnings. Command exits nonzero because of informational findings.
- APK files in mobile/dist/ChakuChuri-0.1.8+9-release.apk and frontend/public/downloads/chakuchuri-android.apk have identical SHA-256:
  86369a14fe87d1ed6a874d3c52bac01b26de3939535cdecb099403a592d70528.
- apksigner verifies the artifact and reports certificate subject C=US, O=Android, CN=Android Debug.
- API health at http://127.0.0.1:8002/health was unreachable during review. Source inspection cannot establish which binary was previously running.
- Available .logs/backend-8002.err.log ends at 08:20:10 on September 5. It shows repeated customer requests, but lacks response status, response bytes, and client identity.
- No APK rebuild, API restart, deployment, or real customer-data mutation was performed for this review.

## Findings, by priority

### P0: Selected and inactive notices leak through realtime

backend/internal/workflow/notices.go:145 and :204 broadcast notices with an empty CustomerID. workspace_realtime.go:191 filters only the change's customer ID and the connection's saved user status, not the notice's audience or Active flag.

The snapshot endpoint correctly filters customer audiences. The socket path does not. Mobile renders all active notices received, checking Active but not CustomerIDs. Hiding content in the UI is not an authorization fix.

Reproduction: TestReviewSelectedNoticeIsolation and TestReviewInactiveNoticeIsolation.
Required outcome: customer B never receives customer A's private notice or any inactive notice body. Audience changes must remove an old notice from clients that lose access. Restricted staff access must also be checked.

### P0: Persistence failure is reported as success

backend/internal/workflow/persistence.go:139 logs SaveState failure but cannot return it to the mutation caller. Notice mutations modify memory and publish before deferred persistence completes.

Reproduction: TestReviewNoticePersistenceFailure uses a failing synthetic repository. Create returns nil error, leaves the new notice in memory, and already queued a customer-visible event.

Required outcome: durable commit precedes success and event publication; failures leave no success message, phantom record, or notification. Treat this as a shared workflow architecture issue, not only an App Home bug. Coordinate accounting changes with its current owner.

Also review startup recovery at persistence.go:93: repository load errors currently lead to persistLocked against existing in-memory state. Missing stores, corrupt stores, migration errors, and transient database failures must have distinct handling. Do not seed or overwrite automatically after an unexpected load failure.

### P1: Web build is broken

frontend/src/platform/customer-documents.tsx:603 and :614 use acceptedCount without a declaration. Fix the intended count and run TypeScript plus a KYC page render test; do not mask the error.

### P1: PostgreSQL mode still saves complete in-memory snapshots

backend/internal/workflow/postgres.go:97 walks all products, quotes, orders, shipments, payments, ledgers, conversations/messages, and calls on SaveState. Service holds shared in-memory state under a process mutex; workflow.go:3185 generates IDs from a process-local daily counter.

Switching a connection string to PostgreSQL does not make this safe for multiple API processes. Independently loaded snapshots can overwrite newer records, IDs can conflict, and individual mutations grow more expensive with history.

Required outcome: scoped repository reads/writes, database-backed unique identifiers, bounded queries, transactional updates, conflict detection, and atomic outbox records.

### P1: Realtime causes customer reloads and global fanout

frontend/src/PlatformAppNext.tsx:251 calls loadCustomers for every presence event. Heartbeats are sent every 15 seconds; auth/access.go:207 permits presence publication every 12 seconds. This is event-triggered repeated fetching, not simply an old setInterval polling loop.

workspace_realtime.go:170 loops over every connected socket and queues a frame even when no changes are visible. TestReviewUnrelatedEventFanout reproduces the empty frame.

Required outcome: apply presence by user/customer ID without fetching customer profiles. Coalesce genuine profile invalidations, deduplicate in-flight requests, and use customer/tenant subscription groups.

Do not simply drop empty frames while retaining global consecutive-revision checks: coordinate cursor semantics with both clients or they may repeatedly resync when unrelated revisions are skipped.

### P1: Socket authorization becomes stale

workspace_realtime.go stores the authenticated User at connection time and uses that copy when filtering later events. Ping refreshes presence, not session validity or current permissions.

Required outcome: logout, expiry, suspension, reassignment, and role changes revoke/re-scope existing connections server-side. A modified client must not be able to keep receiving data by ignoring a users event.

Web workspace/call sockets and mobile call sockets still put bearer tokens in URL queries. Mobile workspace now uses an Authorization header. Use headers for native clients; evaluate same-origin HttpOnly session cookies with origin validation or short-lived single-use socket tickets for browsers. Redact secrets from all logging layers.

### P1: Calls and background notifications remain incomplete

mobile/lib/src/direct_call.dart:667 accepts an answer only when there is no existing remote description. A later ICE restart answer is ignored. _restartIce can remain marked in progress if negotiation does not connect and does not throw.

Required outcome: serialized signaling, valid renegotiation transitions, bounded retries with timeout, ICE-generation handling, reconnecting signaling, and tests on real Wi-Fi/mobile-network transitions.

Local notifications are generated from workspace updates. The app closes the workspace socket on pause when no call is active. This is not reliable background incoming-call/message delivery. Add FCM/APNs with device registration, deep links, appropriate platform call integration, expiry, and delivery deduplication.

### P2: Media delivery has no explicit shared cache strategy

Featured 168x168 logical tiles fetch full originals at customer_shell.dart:1519. ApiClient.fetchFileBytes downloads bytes on each load without a shared bounded byte cache or in-flight deduplication. Feature-image responses also have no request generation check if file IDs change while downloads are in flight.

Web apiBlob performs independent fetches. files.go:314 delegates to http.ServeFile without an explicit per-visibility Cache-Control policy. ServeFile already supports Last-Modified conditional behavior; repeated log lines alone do not prove full image bytes were transferred.

Required outcome: optimized image variants suited to device pixel density, safe bounded caches, in-flight deduplication, stale-response protection, explicit validators/cache directives, and original downloads only when needed. Keep selected featured tiles square with names below.

Public catalog media can use a CDN. KYC, payment proofs, and customer-only attachments must remain private; do not place all files behind unrestricted public cache rules.

### P1: Release and operational evidence is insufficient

The downloadable APK is debug-signed. Gradle explicitly falls back to debug signing when key.properties is absent. Replace that fallback with a separate demo variant and make production release fail without the production key. Verify package ID, signing continuity, HTTPS environment, checksum, install/upgrade behavior, and version metadata before release.

Request logger httpx.go:89 logs method, path, request ID, and duration only. A POST log does not establish successful publishing, durable saving, audience correctness, or customer rendering. Long WebSocket request duration is normally connection lifetime, not slow HTTP work.

## Corrections to Cursor's explanation

1. Correct: a few local clients can create many log entries; log volume is not a user count.
2. Incomplete: /customers traffic here is directly triggered by presence events even though WebSocket is already enabled. Adding WebSocket alone will not fix it.
3. Unsupported: seeing POST /notices or /featured does not prove the feature works. Status, durable data, permissions, and recipient rendering must be verified.
4. Too weak: cache tuning is performance work, but audience leakage, false save success, source build failures, and debug signing are release blockers.
5. Premature: sharding/regions should follow measurements and requirements. Begin with sound transactions, bounded APIs, indexed PostgreSQL, reliable jobs, media delivery, and observability.
6. Misleading framing: a Go process is not inherently unsuitable for high concurrency. This application's shared snapshots, fanout and workload determine capacity. Neither a language choice nor a load balancer proves support for 20,000 users.
7. Unknown: without client identity, status/byte counts and representative tests, the exact source/cost of all observed traffic is not established.

## Implementation order and acceptance checks

### 1. Stabilize and close privacy/data-loss gaps

- Preserve Cursor's Home layout, copy, and published artifacts; establish current source and build provenance.
- Fix KYC TypeScript failure and add a focused render regression.
- Enforce notice audience/active permissions in snapshot and socket paths, including removal on audience change.
- Revalidate/revoke active sockets after access changes.
- Make persistence failures visible and fail startup safely on unexpected repository errors.
- Convert the review probes into ordinary regression tests alongside each fix.
- Pass criteria: wrong-recipient payload tests pass; forced save failure produces no success/event; build and KYC checks pass; live data is unchanged by tests.

### 2. Reduce unnecessary client work

- Patch presence locally and refresh customer profiles only on actual profile changes.
- Deduplicate concurrent reads and coalesce bursts; stop nonessential work on hidden/background screens.
- Keep initial/recovery snapshots; apply confirmed mutations and events at record level.
- Add request-generation/session guards so stale requests cannot overwrite newer account state.
- Establish per-screen request/byte budgets and count foreground requests separately from heartbeat frames.
- Pass criteria: healthy idle clients issue no periodic /workspace or /customers reads solely for presence; an unrelated event preserves form drafts and does not refetch images.

### 3. Make media efficient and private

- Generate responsive thumbnail variants and retain originals for explicit preview/download.
- Add bounded account/server-scoped caches and concurrent-download deduplication; clear private caches on logout/revocation.
- Configure public versus private cache policies, validators, and object-storage/CDN delivery appropriately.
- Add upload progress, retry behavior, bounded concurrency, file validation, and orphan cleanup.
- Pass criteria: repeated views reuse cached variants within policy; private documents cannot be fetched or exposed across accounts; old image responses never replace new ones.

### 4. Replace snapshot persistence with transactional repositories

- Take and verify backups before migration; dry-run imports and reconcile records/attachments.
- Apply all migrations, including 000017; keep JSON mode explicitly demo-only.
- Convert one owned module at a time to scoped SQL reads and writes, stable IDs, optimistic concurrency where needed, and idempotency keys for retryable mutations.
- Save committed events in an outbox in the same transaction; dispatch with retries and consumer deduplication.
- Add cursor pagination and per-resource detail endpoints, especially chat messages.
- Pass criteria: concurrent edits and two API instances preserve both updates; failed transactions roll back; retry tests do not create duplicate records or messages; restore test matches reconciled totals.

### 5. Make realtime and background delivery production-ready

- Scope subscriptions by tenant/customer/conversation and route only relevant events.
- Define ordered per-stream cursors, replay/resync behavior, reconnect jitter, backpressure, and token expiry.
- Add Redis-backed presence expiry and cross-instance event distribution only after durable writes and event contracts are correct.
- Complete FCM/APNs, TURN credentials, mobile call state/ICE recovery, notification tap routing, and real-device tests.
- Pass criteria: customers on different API instances receive authorized updates; offline/reconnect paths converge; calls survive supported network changes; late/duplicate events do not revive ended calls.

### 6. Validate capacity and release

- Instrument HTTP status/bytes/latencies, DB pool/lock waits, socket counts/queue depth, upload bandwidth, memory, and event delivery lag. Preserve WebSocket upgrade support when wrapping response writers.
- Publish a workload definition: idle sockets, simultaneously active users, mutation/read rates, file sizes, chat rate, and concurrent calls. Test media/TURN separately from signaling.
- Ramp an isolated environment through 100, 1,000, 5,000, then 20,000 connections plus the agreed active workload. Run spike, reconnect-storm, soak, and dependency-failure tests.
- Initial targets to refine against real usage: p95 interactive API latency below 500 ms excluding transfers; p95 visible event delivery below 1 second; request error rate below 1%; no cross-customer data exposure or lost/duplicate business operations.
- Set CPU/memory/headroom and bandwidth limits from measured tests; add capacity when a measured bottleneck requires it.
- Build only after source checks and user release direction; require real signing for production, include build identity in health/version endpoints, then verify upgrade and rollback.

No phase percentage or 20,000-user readiness claim should replace these pass criteria.

## Reproduce the review checks

From backend:

~~~powershell
go test ./internal/workflow ./internal/files ./internal/auth ./internal/customers
go test -overlay ../docs/reviews/2026-09-05/review-overlay.json ./internal/workflow -run TestReview -count=1 -v
~~~

The second command intentionally fails on the reviewed source. It overlays synthetic tests into the workflow package and does not alter the running service or application source. Overlay paths are absolute for this workspace.

From frontend: run node_modules/typescript/bin/tsc --noEmit -p tsconfig.json with Node.
From mobile: flutter analyze --no-pub. No APK build is needed for either check.

## Reference documentation

- Private versus shared HTTP caching: https://developer.mozilla.org/en-US/docs/Web/HTTP/Guides/Caching
- Flutter FCM background/terminated handling and platform limitations: https://firebase.google.com/docs/cloud-messaging/flutter/receive-messages
- PostgreSQL transaction isolation: https://www.postgresql.org/docs/18/sql-set-transaction.html
