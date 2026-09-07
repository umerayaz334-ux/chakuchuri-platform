DROP INDEX IF EXISTS idx_calls_customer_initiated;
DROP INDEX IF EXISTS idx_calls_tenant_status_initiated;

ALTER TABLE calls
  DROP COLUMN IF EXISTS answered_by_user_external_id,
  DROP COLUMN IF EXISTS initiator_user_external_id,
  DROP COLUMN IF EXISTS duration_seconds,
  DROP COLUMN IF EXISTS ring_expires_at,
  DROP COLUMN IF EXISTS updated_at,
  ADD COLUMN IF NOT EXISTS priority int NOT NULL DEFAULT 100;

ALTER TABLE calls RENAME COLUMN started_at TO answered_at;
ALTER TABLE calls RENAME COLUMN initiated_at TO requested_at;
ALTER TABLE calls RENAME TO call_queue_entries;

CREATE INDEX IF NOT EXISTS idx_call_queue_status_priority
  ON call_queue_entries (tenant_id, status, priority, requested_at);