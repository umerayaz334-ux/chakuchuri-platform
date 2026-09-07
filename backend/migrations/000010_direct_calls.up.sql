ALTER TABLE call_queue_entries RENAME TO calls;

ALTER TABLE calls RENAME COLUMN requested_at TO initiated_at;
ALTER TABLE calls RENAME COLUMN answered_at TO started_at;

DROP INDEX IF EXISTS idx_call_queue_status_priority;

ALTER TABLE calls
  DROP COLUMN IF EXISTS priority,
  ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now(),
  ADD COLUMN IF NOT EXISTS ring_expires_at timestamptz,
  ADD COLUMN IF NOT EXISTS duration_seconds int NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS initiator_user_external_id text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS answered_by_user_external_id text NOT NULL DEFAULT '';

UPDATE calls
SET status = 'missed',
    ended_at = COALESCE(ended_at, initiated_at),
    updated_at = COALESCE(ended_at, initiated_at)
WHERE lower(status) IN ('waiting', 'queued', 'queue');

CREATE INDEX IF NOT EXISTS idx_calls_tenant_status_initiated
  ON calls (tenant_id, status, initiated_at DESC);

CREATE INDEX IF NOT EXISTS idx_calls_customer_initiated
  ON calls (tenant_id, customer_id, initiated_at DESC);