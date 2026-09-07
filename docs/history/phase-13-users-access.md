# Users And Access

Users and access are shared work across Phase 3 (core backend) and Phase 6 (admin portal).

## Completed

- Customer logins open the customer portal and are bound to their own customer account.
- Owner, Admin, Manager, Accountant, Shipping Staff, Production Staff and Support Agent logins open the admin portal with page and feature permissions.
- Every role has Personal Settings for display name/email, password, profile picture and login activity.
- Administrators can create, edit, suspend, reactivate and permanently delete managed users.
- Temporarily suspended users can still authenticate, but only see a custom administrator notice and contact form; all normal page and API actions are blocked.
- Contact requests from suspended users appear in the Users & Access inspector.
- Profile pictures are served through authenticated file endpoints and appear anywhere the user's initials previously appeared.
- Page permissions, feature permissions, assigned-customer scope, failed login counts and session invalidation are enforced by the backend.

## Active Auth Endpoints

- `GET /api/auth/me`
- `POST /api/auth/profile`
- `POST /api/auth/suspension/contact`
- `GET /api/auth/users/options`
- `GET /api/auth/users`
- `POST /api/auth/users`
- `POST /api/auth/users/{id}`
- `POST /api/auth/users/{id}/delete`

## Remaining Production Work

- Invitation and password-reset email delivery.
- Optional multi-factor authentication.
- Device/session management and administrator session revocation UI.
- Expanded access-change reporting and security alerts.