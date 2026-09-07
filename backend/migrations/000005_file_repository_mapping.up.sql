ALTER TABLE files ADD COLUMN IF NOT EXISTS external_id text;
ALTER TABLE files ADD COLUMN IF NOT EXISTS thumbnail_storage_key text;
ALTER TABLE files ADD COLUMN IF NOT EXISTS app_data jsonb NOT NULL DEFAULT '{}';

UPDATE files
SET external_id = id::text
WHERE external_id IS NULL OR external_id = '';

ALTER TABLE files ALTER COLUMN external_id SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_files_tenant_external_id
  ON files (tenant_id, external_id);

CREATE INDEX IF NOT EXISTS idx_files_tenant_customer_created
  ON files (tenant_id, customer_id, created_at DESC);
