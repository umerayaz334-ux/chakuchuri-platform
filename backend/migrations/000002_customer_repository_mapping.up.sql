ALTER TABLE tenants ADD COLUMN IF NOT EXISTS external_id text;

UPDATE tenants
SET external_id = CASE
  WHEN slug IS NOT NULL AND slug <> '' THEN slug
  ELSE id::text
END
WHERE external_id IS NULL OR external_id = '';

ALTER TABLE tenants ALTER COLUMN external_id SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_tenants_external_id
  ON tenants (external_id);

ALTER TABLE customers ADD COLUMN IF NOT EXISTS external_id text;
ALTER TABLE customers ADD COLUMN IF NOT EXISTS portal_data jsonb NOT NULL DEFAULT '{}';

UPDATE customers
SET external_id = id::text
WHERE external_id IS NULL OR external_id = '';

ALTER TABLE customers ALTER COLUMN external_id SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_customers_tenant_external_id
  ON customers (tenant_id, external_id);

CREATE INDEX IF NOT EXISTS idx_customers_tenant_created
  ON customers (tenant_id, created_at DESC);
