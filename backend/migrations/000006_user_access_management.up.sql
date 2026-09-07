ALTER TABLE users ADD COLUMN IF NOT EXISTS page_access jsonb NOT NULL DEFAULT '[]';
ALTER TABLE users ADD COLUMN IF NOT EXISTS assigned_customer_ids jsonb NOT NULL DEFAULT '[]';
ALTER TABLE users ADD COLUMN IF NOT EXISTS failed_login_count integer NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN IF NOT EXISTS login_activity jsonb NOT NULL DEFAULT '[]';
ALTER TABLE users ADD COLUMN IF NOT EXISTS last_login_at timestamptz;

CREATE INDEX IF NOT EXISTS idx_users_tenant_status_role
  ON users (tenant_id, status, role);
