ALTER TABLE users ADD COLUMN IF NOT EXISTS profile_image_file_id text NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS suspension_notice text NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS suspension_contact_at text NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS suspension_contact_message text NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_users_suspension_contact
  ON users (tenant_id, status)
  WHERE suspension_contact_at <> '';
