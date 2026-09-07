-- App home content shown on the customer mobile dashboard.
ALTER TABLE workflow_state_meta
  ADD COLUMN IF NOT EXISTS customer_notices jsonb NOT NULL DEFAULT '[]'::jsonb;

ALTER TABLE workflow_state_meta
  ADD COLUMN IF NOT EXISTS featured_products jsonb NOT NULL DEFAULT '[]'::jsonb;
