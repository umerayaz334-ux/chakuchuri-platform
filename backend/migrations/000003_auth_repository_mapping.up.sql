ALTER TABLE users ADD COLUMN IF NOT EXISTS external_id text;
ALTER TABLE users ADD COLUMN IF NOT EXISTS salt text NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS permissions jsonb NOT NULL DEFAULT '[]';

UPDATE users
SET external_id = id::text
WHERE external_id IS NULL OR external_id = '';

ALTER TABLE users ALTER COLUMN external_id SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_tenant_external_id
  ON users (tenant_id, external_id);

CREATE INDEX IF NOT EXISTS idx_users_tenant_role
  ON users (tenant_id, role, status);
