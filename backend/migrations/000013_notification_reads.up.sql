ALTER TABLE users ADD COLUMN IF NOT EXISTS notification_read_ids jsonb NOT NULL DEFAULT '[]'::jsonb;
