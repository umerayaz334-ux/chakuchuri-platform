# Phase 11A: Email Automation and Delivery

## Goal

Give the owner a reliable email operations center for transactional customer communication. The platform must support manual and automatic messages without slowing down quotations, orders, payments or shipping actions.

Initial examples:

- Formal quotation ready
- Order confirmed or amended
- Production delay or milestone update
- Order ready for dispatch
- Payment due or overdue reminder
- Payment proof received or confirmed
- Shipment dispatched, in transit or delivered
- Order cancellation and account-credit notice

## Product Experience

### Owner Email Center

Add an owner-only `Email` page with compact operational tabs:

1. **Overview**: connection health, queued mail, delivery rate, failures and recent activity.
2. **Automations**: enable or disable each trigger, choose timing, audience, quiet hours and reminder limits.
3. **Templates**: edit subject and body, preview with real-looking sample data, send a test and view version history.
4. **Deliveries**: searchable log of scheduled, sent, delivered, bounced, failed, suppressed and cancelled mail.
5. **Settings**: provider connection, sender identity, reply-to address, domain verification and webhook health.

Order, quote, payment and shipment detail views may expose a small `Send email` action. It opens a preview using the correct template instead of sending immediately.

### Customer Experience

- Emails link to the exact portal route, such as `/orders/{id}` or `/payments`.
- English and Urdu templates follow the customer's saved language.
- Optional notifications can be disabled by the customer; essential account and transaction notices remain available.
- The customer portal shows the same event in its activity history, so email and in-app information cannot disagree.

## Architecture

### Provider Boundary

Define a Go `Mailer` interface so SMTP or an API provider can be changed without touching order/payment code. Credentials live in environment variables or encrypted secret storage, never in JSON snapshots, frontend code or logs.

Required operations:

- Verify configuration
- Send a test email
- Send a rendered transactional email
- Parse delivery webhooks
- Normalize provider errors and message IDs

### Transactional Outbox

Business services must not call the email provider directly.

1. A quotation/order/payment/shipping transaction commits its business change.
2. The same transaction adds a uniquely keyed notification event to `email_outbox`.
3. A background worker claims queued events.
4. The worker renders the selected template and sends it.
5. Delivery attempts and provider events are appended to the audit trail.

Every event uses an idempotency key, for example `order:MFG-123:ready:v1`. Retried jobs therefore cannot send accidental duplicates.

### Queue and Retry Rules

- Redis-backed jobs in production; database claiming is an acceptable local-development fallback.
- Exponential retry for temporary failures.
- No retry for permanent invalid-recipient failures.
- Dead-letter state after the configured attempt limit.
- Owner can retry a failed item or cancel a scheduled item.
- Per-customer and per-template rate limits prevent reminder spam.

## Data Model

Create versioned PostgreSQL migrations for:

- `email_connections`: provider type and non-secret configuration metadata.
- `email_sender_identities`: from name, from address, reply-to and verification status.
- `email_templates`: stable template key, language, current version and enabled state.
- `email_template_versions`: immutable subject/body versions and editor audit data.
- `email_automation_rules`: trigger, delay, cadence, audience, quiet hours and enabled state.
- `email_outbox`: event payload, idempotency key, schedule and processing state.
- `email_deliveries`: recipient, rendered template version, provider message ID and attempt state.
- `email_delivery_events`: immutable accepted, delivered, bounced, complained and failed events.
- `notification_preferences`: customer choices by channel and category.

Email addresses are normalized. Provider secrets are not stored in these tables unless encrypted with a deployment-managed key.

## Template System

Start with a restricted variable engine rather than arbitrary executable templates.

Supported variable groups:

- Customer: name, company and portal URL
- Quote: ID, product, quantity, unit price, total and expiry
- Order: ID, product, quantity, stage, expected date, paid amount and balance
- Payment: ID, type, amount, status and due date
- Shipment: ID, courier, tracking number, status and tracking URL
- Company: support email, phone and reply-to address

Unknown or missing required variables block sending and create an actionable failure. HTML is sanitized, links are validated, and every template has a plain-text alternative.

## Initial Automation Rules

| Trigger | Default | Timing | Guardrails |
| --- | --- | --- | --- |
| Quote priced | On | Immediate | One email per quote version |
| Quote accepted | On | Immediate | Includes exact order link |
| Order amended | On | Immediate | Shows before/after summary |
| Order ready | On | Immediate | Suppress if order cancelled |
| Order delayed | Off | Manual approval | Requires new expected date and reason |
| Payment due | On | Configurable | Stop after payment or cancellation |
| Payment overdue | Off | Daily or weekly | Maximum reminder count and quiet hours |
| Payment confirmed | On | Immediate | Uses confirmed amount from ledger |
| Shipment dispatched | On | Immediate | Requires courier/tracking data |
| Order cancelled | On | Immediate | Shows reversal and any customer credit |

## Security and Deliverability

- Owner access requires `email.manage`; read-only delivery access uses `email.read`.
- Sensitive connection changes require current-password confirmation and an audit entry.
- Verify webhook signatures and reject replayed events.
- Configure SPF, DKIM and DMARC before production sending.
- Never log email bodies, passwords, API keys or complete provider webhook secrets.
- Track bounces and complaints; suppress repeatedly invalid recipients.
- Use a dedicated transactional sender and a monitored reply-to inbox.

## Delivery Plan

### 11A.1 Foundation

- Define domain events, provider interface, template keys and PostgreSQL schema.
- Build an in-memory/local mail sink for development and automated tests.
- Add owner permissions and audit actions.

### 11A.2 Owner UI

- Build Email Overview, Automations, Templates, Deliveries and Settings.
- Add preview, test-send, filters and failure detail.
- Keep provider secrets server-only.

### 11A.3 Business Triggers

- Connect quote, order, payment and shipping events to the outbox.
- Begin with quote priced, order ready, payment due, payment confirmed and order cancelled.
- Add manual preview/send actions to detail pages.

### 11A.4 Reliable Delivery

- Add Redis worker, scheduling, retry, idempotency and dead-letter handling.
- Add provider delivery webhooks and live status updates in the Email Center.

### 11A.5 Preferences and Localization

- Add customer preferences, English/Urdu template pairs and quiet hours.
- Add optional digest rules for non-urgent activity.

### 11A.6 Production Readiness

- Verify sending domain and provider webhooks.
- Test duplicate prevention, retries, cancellation races, bounce handling and backup restore.
- Add delivery metrics, alerts and runbooks.

## Acceptance Criteria

- A business action succeeds even if email delivery is temporarily unavailable.
- The same event cannot send duplicate emails when a worker retries.
- Cancelling or paying an order stops scheduled reminders for that obligation.
- Every delivery identifies its trigger, template version, recipient, attempts and final state.
- The owner can preview, test, enable, disable, schedule, cancel and retry without exposing credentials.
- Customer and admin see consistent order/payment values in email and in the portal.
- English and Urdu templates render correctly on desktop and mobile email clients.
