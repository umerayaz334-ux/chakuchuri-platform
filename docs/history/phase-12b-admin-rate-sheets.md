# Phase 12 Admin Rate Sheets

Admin shipping now has a first real rate sheet management flow.

## Completed

- Added `POST /api/workflow/ratesheets`.
- Added `POST /api/workflow/ratesheets/import`.
- Added editable courier, service, zone, weight and price rows.
- Added optional source sheet upload through the file module.
- Customer shipping rate lookup now matches courier, zone and weight band before falling back.
- Rate rows persist through the workflow repository:
  - JSON in local development
  - PostgreSQL when `DATABASE_URL` is configured

## Next Step

Add stronger accounting ledger links for imported shipping charges and customer statements.
