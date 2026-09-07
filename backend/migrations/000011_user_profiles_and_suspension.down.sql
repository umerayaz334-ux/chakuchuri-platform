DROP INDEX IF EXISTS idx_users_suspension_contact;
ALTER TABLE users DROP COLUMN IF EXISTS suspension_contact_message;
ALTER TABLE users DROP COLUMN IF EXISTS suspension_contact_at;
ALTER TABLE users DROP COLUMN IF EXISTS suspension_notice;
ALTER TABLE users DROP COLUMN IF EXISTS profile_image_file_id;
