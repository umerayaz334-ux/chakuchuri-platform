DROP INDEX IF EXISTS idx_files_tenant_customer_created;
DROP INDEX IF EXISTS idx_files_tenant_external_id;

ALTER TABLE files DROP COLUMN IF EXISTS app_data;
ALTER TABLE files DROP COLUMN IF EXISTS thumbnail_storage_key;
ALTER TABLE files DROP COLUMN IF EXISTS external_id;
