CREATE TABLE IF NOT EXISTS call_signals (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  call_id uuid NOT NULL REFERENCES call_queue_entries(id) ON DELETE CASCADE,
  external_id text NOT NULL,
  signal_no int NOT NULL,
  sender_user_external_id text NOT NULL DEFAULT '',
  sender_role text NOT NULL DEFAULT '',
  signal_type text NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}',
  app_data jsonb NOT NULL DEFAULT '{}',
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_call_signals_call_external_id
  ON call_signals (call_id, external_id);

CREATE INDEX IF NOT EXISTS idx_call_signals_call_signal_no
  ON call_signals (tenant_id, call_id, signal_no);

ALTER TABLE call_signals ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS tenant_call_signals ON call_signals;
CREATE POLICY tenant_call_signals ON call_signals USING (tenant_id = current_setting('app.current_tenant_id')::uuid);
