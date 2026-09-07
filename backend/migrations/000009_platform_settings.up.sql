ALTER TABLE workflow_state_meta
  ADD COLUMN IF NOT EXISTS platform_settings jsonb NOT NULL DEFAULT '{}';
