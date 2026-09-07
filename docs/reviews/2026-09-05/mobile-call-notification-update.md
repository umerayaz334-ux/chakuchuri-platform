# Mobile calls and notifications

## Changes

- Call signaling uses an Authorization header through IOWebSocketChannel, not a bearer token in its URL.
- Dropped/silent sockets retry with capped backoff. HTTP fallback has one polling operation at a time and stops when the socket recovers.
- Native signal operations are serialized. Successful signal IDs are deduplicated; failed operations remain retryable.
- Only catch-up batches advance the receive cursor, not outgoing acknowledgements or individual live signals.
- Answers are applied while a local offer is pending, including ICE restart offers.
- ICE recovery has timed retries and a manual retry control after the automatic attempt limit.
- Setup checks whether the call is still alive before retaining newly created peers/media. Disposal cancels reconnect and recovery timers.
- Chat notification identities use incoming message IDs instead of unread counts.
- Call heartbeats do not create new alert identities; outgoing ringing calls are not labeled incoming.
- Notification work is serialized across account resets, old notifications are cancelled, and lock-screen visibility is private.
- Notification initialization failures do not block sign-in.
- Updated the auth smoke test for the welcome screen and fixed the connection-settings Material wrapper.

## Verification Limits

Unit/widget tests cover serialized signals, duplicate/failed signals, disposal during configuration fetch, notification identities, session isolation, suspension UI and sign-in navigation.
Native audio, ICE renegotiation, Bluetooth routing and Wi-Fi handoff still require two-device testing. Background push/wake-up is not implemented by this batch.
No APK rebuilt: one combined rebuild remains planned after the remaining mobile fixes.
