ALTER TABLE ledger_entries
ADD COLUMN IF NOT EXISTS app_data jsonb NOT NULL DEFAULT '{}';
