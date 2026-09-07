# Roadmap Progress

## Current Position

The current priority is to finish and stabilize the web platform before resuming native mobile work. The public website, customer portal, admin portal and Go API are all usable; the deepest current work is in Phases 3, 5, 6 and 11.

| Phase | Status | Approx. | Notes |
| --- | --- | ---: | --- |
| 1. UI blueprint | Done | 100% | Responsive public, customer and admin design systems are implemented. |
| 2. Project foundation | Done | 100% | Backend, frontend, mobile, migrations, docs and storage structure exists. |
| 3. Core backend | Advanced | 92% | Auth, signup, explicit KYC review access, users, customer identities, permissions, profiles, suspension controls, files, audit/request logs and PostgreSQL repositories exist. Production object storage remains. |
| 4. Website | In progress | 65% | Public entry, services, login and signup exist. Content polish, visitor analytics and production contact/chat delivery remain. |
| 5. Customer portal | Advanced | 85% | Dashboard, quotes, products, manufacturing, shipping, payments, messages, calls and personal settings exist. Final workflow polish and broad QA remain. |
| 6. Admin portal | Advanced | 85% | Customer directory/detail, Users & Access, operational modules, finance, messenger, call history and settings exist. Reports and final workflow polish remain. |
| 7. Quotations | Advanced | 90% | Photo-first request, per-unit pricing, automatic totals, strict newest-first ordering, accept/reject and order conversion work. Final document output remains. |
| 8. Manufacturing | Advanced | 88% | Stages, dates, progress, balances, customer-visible updates and verified edit/cancel actions work. Rich production attachments and final QA remain. |
| 9. Shipping | Advanced | 80% | Outside/manufactured requests, courier rates, CSV/XLSX imports and tracking states work. Carrier integrations remain. |
| 10. Payments/accounting | In progress | 80% | Proofs, confirmation, ledger, balances, statements, exports and immutable order adjustment/cancellation entries exist. Reconciliation and deeper reporting remain. |
| 11. Chat/calls | Advanced | 80% | Modern messenger, five-file/photo attachments, unread markers, presence, ringing, ringtone, WebRTC, WebSocket signaling and call history exist. TURN and production notification delivery remain. |
| 11A. Email automation | Planned | 5% | Owner workflow, outbox architecture, templates, triggers, retries, delivery logs and English/Urdu support are specified. Implementation follows web workflow stabilization. |
| 12. Mobile app | Paused for web | 55% | Flutter shell and core customer screens exist and pass analysis/tests/web build. Native files, WebRTC, push and device QA resume after web completion. |
| 13. Data safety/updates | In progress | 85% | PostgreSQL is required for real runs; offsite backup copy; release checklist and safe deploy script that never overwrite production data. |
| 14. Stability/speed | In progress | 70% | Request IDs, recovery, logs, error boundaries, health checks and scoped WebSocket updates exist. Redis jobs, monitoring and load tuning remain. |

## Latest Completed Work

- Rebuilt KYC as a permissioned state machine with one-shot secure links, hashed tokens, reviewer-only evidence access and mandatory document readiness checks.
- Added a tenant-scoped PostgreSQL KYC request table, strict evidence MIME rules and protected atomic upload endpoints that cannot be bypassed through generic files.
- Added compensating upload rollback across storage and metadata, including an explicit PostgreSQL delete path, plus focused permission, lifecycle and persistence regression tests.
- Redesigned customer and public KYC surfaces around clear lifecycle, readiness, locked-submission and responsive states.
- Replaced repeated whole-workspace polling with scoped WebSocket changes and record-level local mutation patches.
- Quotations are always arranged by customer submission time, newest first, regardless of status.
- Formal quotations use per-unit pricing with an automatic quantity total and no deposit request.
- Accepted orders support owner-verified editing and cancellation using a two-step exact-ID confirmation.
- Order value changes append debit/credit adjustments; cancellations append a full charge reversal without deleting prior accounting history.
- Every role now receives Personal Settings, even for older saved access profiles.
- Users can update their display name/email, password and profile photo.
- Profile photos replace initials across the sidebar, customer directory, Users & Access, messenger and call views.
- Temporarily suspended users can sign in only to the administrator notice/contact screen; all normal APIs and portal pages remain frozen.
- Administrators can set the suspension notice and review the user's contact request in Users & Access.
- The Customers page is now a searchable, filterable, sortable operational directory with reload-safe account routes.
- Messenger uploads now generate collision-safe file IDs and support five distinct attachments in one message.
- New users and signup records now use collision-safe IDs for concurrent requests.

## Next Web Milestone

1. Run KYC against PostgreSQL migration 15 and complete responsive role-permission QA.
2. Complete public website content, contact delivery and visitor analytics.
3. Finish quotation/manufacturing documents and richer production attachments.
4. Build Phase 11A email foundation and owner Email Center, beginning with quote-ready, order-ready and payment reminders.
5. Finish accounting reconciliation, customer reports and polished PDF statements.
6. Productionize calls with TURN, deployment configuration and notification delivery.
7. Run complete recovery and performance QA, then resume native Flutter work.