# Mobile session and suspension fixes

- API requests bind to the session version and token present before connection setup.
- Results from a prior session are rejected before parsing, mutation publication, upload return or binary return. File fallback cannot continue under another session.
- Sign-out invalidates pending anonymous login attempts.
- Suspension profile fields survive secure-storage serialization and profile copies.
- Identity refresh applies suspension before requesting protected customer data. Suspended users see an isolated account screen instead of the customer portal.
- The screen shows the administrator notice and supports contacting the administrator, checking status and signing out. Operational snapshots are cleared and further workspace loads are blocked while suspended.
- Existing sockets remain available for access-change invalidations. Restored access triggers a fresh snapshot.

Focused tests cover session-switch mutation rejection, sign-out during login, file fallback isolation, profile persistence and suspension contact/check/sign-out controls at 360px width.

No APK rebuild yet: the user requested one rebuild after the fixes are batched.
Still pending: call recovery, background push setup, HTTP response deadlines, notification identities, mobile media caching, and the pre-existing app-level widget test's notification-plugin initialization failure.
