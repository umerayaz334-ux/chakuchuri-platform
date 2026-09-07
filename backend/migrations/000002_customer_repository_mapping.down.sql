DROP INDEX IF EXISTS idx_customers_tenant_created;
DROP INDEX IF EXISTS idx_customers_tenant_external_id;
DROP INDEX IF EXISTS idx_tenants_external_id;

ALTER TABLE customers DROP COLUMN IF EXISTS portal_data;
ALTER TABLE customers DROP COLUMN IF EXISTS external_id;
ALTER TABLE tenants DROP COLUMN IF EXISTS external_id;
