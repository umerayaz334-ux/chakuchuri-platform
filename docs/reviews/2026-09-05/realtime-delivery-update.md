# Filtered realtime delivery

- Empty filtered changes no longer produce socket frames.
- Revisions count delivered events per connection, not global activity. Mobile's existing consecutive-revision checks therefore do not interpret another customer's updates as a lost event.
- Subscribe/ready, changed events and heartbeat acknowledgements are queued under the same hub lock to preserve ordering across concurrent publishers.
- Web reconnects always request a snapshot because connection revisions restart at zero. Web also checks changed-event and heartbeat revision gaps.
- Slow connections with full queues disconnect; healthy connections continue. Reconnect snapshot recovery remains required.
- Existing authorization filtering and notice withdrawal semantics are retained.

Regression coverage: unrelated customers, consecutive local revisions, heartbeat revisions, departed subscribers, 100 concurrent publishers, slow subscribers, and browser reconnect/gap handling.

This reduces transmitted frames, not the hub's O(subscribers) filtering cost. Indexed subscriptions, targeted auth/customer invalidations, snapshot/event reconciliation, distributed delivery, and load testing remain pending. No mobile source changes or APK rebuild.
