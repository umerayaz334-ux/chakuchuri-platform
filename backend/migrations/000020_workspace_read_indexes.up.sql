-- Covers tenant-scoped workspace reads and newest-first customer history.
CREATE INDEX IF NOT EXISTS idx_products_tenant_updated_active
  ON products (tenant_id, updated_at DESC)
  WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_quotations_tenant_created
  ON quotations (tenant_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_manufacturing_tenant_created
  ON manufacturing_orders (tenant_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_rate_sheets_tenant_created
  ON shipping_rate_sheets (tenant_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_shipping_tenant_created
  ON shipping_requests (tenant_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_payments_tenant_created
  ON payment_confirmations (tenant_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_ledger_tenant_posted
  ON ledger_entries (tenant_id, posted_at DESC);

CREATE INDEX IF NOT EXISTS idx_conversations_tenant_created
  ON conversations (tenant_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_calls_tenant_initiated
  ON calls (tenant_id, initiated_at DESC);

CREATE INDEX IF NOT EXISTS idx_call_signals_tenant_sequence
  ON call_signals (tenant_id, signal_no ASC, created_at ASC);