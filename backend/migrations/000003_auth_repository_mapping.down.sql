DROP INDEX IF EXISTS idx_users_tenant_role;
DROP INDEX IF EXISTS idx_users_tenant_external_id;

ALTER TABLE users DROP COLUMN IF EXISTS permissions;
ALTER TABLE users DROP COLUMN IF EXISTS salt;
ALTER TABLE users DROP COLUMN IF EXISTS external_id;
