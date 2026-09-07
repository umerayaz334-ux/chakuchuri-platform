# Realtime access and presence update

## Implemented

- Web customer presence is derived from workspace presence events. Presence-only events no longer request the full customer list.
- Multiple customer users are aggregated: a customer remains online while any reported customer user is online. Staff presence does not change customer status.
- Workspace sockets revalidate their session and access scope before writing events and every five seconds while idle.
- Logout, expiry, suspension changes, role changes, page/permission changes and customer assignment changes invalidate the existing connection.
- Invalid connections receive only a users invalidation notice before closure, not their queued operational data. Existing web and mobile handlers use that notice to refresh identity.
- Socket pings check authorization before updating presence.
- Authorization checks run outside the publisher to avoid reentering auth locks.

## Verification

- Backend: go test ./... passed.
- Frontend: four Node regression tests passed.
- Frontend production build passed; existing large-bundle warning remains.
- API restarted on 8002; frontend API proxy on 5170 returned HTTP 200.
- APK was not rebuilt.

## Remaining

- Exercise logout, suspension and reassignment on physical devices; current regression coverage tests access keys and outgoing-event sanitization, not a full device session.
- Scope event routing and revisions to avoid empty global broadcasts.
- Remove browser socket bearer tokens from URLs using a dedicated authentication design.
- Deduplicate/cache private thumbnails without exposing protected documents.
- Replace whole-state persistence with transactional, scoped database writes and test restore/failure behavior.
- Measure request budgets and run realistic concurrency/load tests before making capacity claims.
- This change rechecks access before a write; it does not make authorization changes atomic with an already-running network write.
