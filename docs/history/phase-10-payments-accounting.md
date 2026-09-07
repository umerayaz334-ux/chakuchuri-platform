# Phase 10 Payments And Accounting

This phase adds the first real customer ledger model for ChakuChuri.pk.

Implemented:

- Manufacturing orders post debit ledger entries when accepted quotes become orders.
- Shipping requests post debit ledger entries for the quoted courier/service cost.
- Confirmed payments post credit ledger entries.
- Customer and admin workspace responses include `ledger`.
- Dashboard metrics include `ledgerBalance`.
- Customer Payments page shows balance summary and statement rows.
- Admin Payments page shows pending proofs and the customer ledger statement.
- Local JSON snapshots persist ledger entries.
- PostgreSQL repository saves and loads ledger entries.

Current scope:

- Ledger entries are source-linked to manufacturing orders, shipping requests and confirmed payments.
- Payment proofs remain pending until admin confirmation.
- The UI now answers charges, confirmed paid, pending proof and balance due in one place.

Next accounting improvements:

- Customer-wise filters in admin ledger views.
- Downloadable customer statement PDF.
- Manual adjustment entries with approval notes.
- Monthly reports and export files.
