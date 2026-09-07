DROP INDEX IF EXISTS idx_users_tenant_status_role;

ALTER TABLE users DROP COLUMN IF EXISTS last_login_at;
ALTER TABLE users DROP COLUMN IF EXISTS login_activity;
ALTER TABLE users DROP COLUMN IF EXISTS failed_login_count;
ALTER TABLE users DROP COLUMN IF EXISTS assigned_customer_ids;
ALTER TABLE users DROP COLUMN IF EXISTS page_access;
