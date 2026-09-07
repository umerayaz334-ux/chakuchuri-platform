ALTER TABLE products ADD COLUMN IF NOT EXISTS external_id text;
ALTER TABLE products ADD COLUMN IF NOT EXISTS app_data jsonb NOT NULL DEFAULT '{}';

UPDATE products
SET external_id = id::text
WHERE external_id IS NULL OR external_id = '';

ALTER TABLE products ALTER COLUMN external_id SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_products_tenant_external_id
  ON products (tenant_id, external_id);

ALTER TABLE quotations ADD COLUMN IF NOT EXISTS external_id text;
ALTER TABLE quotations ADD COLUMN IF NOT EXISTS app_data jsonb NOT NULL DEFAULT '{}';

UPDATE quotations
SET external_id = quote_no
WHERE external_id IS NULL OR external_id = '';

ALTER TABLE quotations ALTER COLUMN external_id SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_quotations_tenant_external_id
  ON quotations (tenant_id, external_id);

ALTER TABLE manufacturing_orders ADD COLUMN IF NOT EXISTS external_id text;
ALTER TABLE manufacturing_orders ADD COLUMN IF NOT EXISTS app_data jsonb NOT NULL DEFAULT '{}';

UPDATE manufacturing_orders
SET external_id = order_no
WHERE external_id IS NULL OR external_id = '';

ALTER TABLE manufacturing_orders ALTER COLUMN external_id SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_manufacturing_tenant_external_id
  ON manufacturing_orders (tenant_id, external_id);

ALTER TABLE shipping_rate_sheets ADD COLUMN IF NOT EXISTS external_id text;
ALTER TABLE shipping_rate_sheets ADD COLUMN IF NOT EXISTS app_data jsonb NOT NULL DEFAULT '{}';

UPDATE shipping_rate_sheets
SET external_id = id::text
WHERE external_id IS NULL OR external_id = '';

ALTER TABLE shipping_rate_sheets ALTER COLUMN external_id SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_rate_sheets_tenant_external_id
  ON shipping_rate_sheets (tenant_id, external_id);

ALTER TABLE shipping_requests ADD COLUMN IF NOT EXISTS external_id text;
ALTER TABLE shipping_requests ADD COLUMN IF NOT EXISTS app_data jsonb NOT NULL DEFAULT '{}';

UPDATE shipping_requests
SET external_id = request_no
WHERE external_id IS NULL OR external_id = '';

ALTER TABLE shipping_requests ALTER COLUMN external_id SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_shipping_tenant_external_id
  ON shipping_requests (tenant_id, external_id);

ALTER TABLE payment_confirmations ADD COLUMN IF NOT EXISTS external_id text;
ALTER TABLE payment_confirmations ADD COLUMN IF NOT EXISTS app_data jsonb NOT NULL DEFAULT '{}';

UPDATE payment_confirmations
SET external_id = id::text
WHERE external_id IS NULL OR external_id = '';

ALTER TABLE payment_confirmations ALTER COLUMN external_id SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_payments_tenant_external_id
  ON payment_confirmations (tenant_id, external_id);

ALTER TABLE conversations ADD COLUMN IF NOT EXISTS external_id text;
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS app_data jsonb NOT NULL DEFAULT '{}';

UPDATE conversations
SET external_id = id::text
WHERE external_id IS NULL OR external_id = '';

ALTER TABLE conversations ALTER COLUMN external_id SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_conversations_tenant_external_id
  ON conversations (tenant_id, external_id);

ALTER TABLE messages ADD COLUMN IF NOT EXISTS external_id text;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS app_data jsonb NOT NULL DEFAULT '{}';

UPDATE messages
SET external_id = id::text
WHERE external_id IS NULL OR external_id = '';

ALTER TABLE messages ALTER COLUMN external_id SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_messages_conversation_external_id
  ON messages (conversation_id, external_id);

ALTER TABLE call_queue_entries ADD COLUMN IF NOT EXISTS external_id text;
ALTER TABLE call_queue_entries ADD COLUMN IF NOT EXISTS app_data jsonb NOT NULL DEFAULT '{}';

UPDATE call_queue_entries
SET external_id = id::text
WHERE external_id IS NULL OR external_id = '';

ALTER TABLE call_queue_entries ALTER COLUMN external_id SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_calls_tenant_external_id
  ON call_queue_entries (tenant_id, external_id);

CREATE TABLE IF NOT EXISTS workflow_state_meta (
  tenant_id uuid PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
  last_sequence_day text NOT NULL DEFAULT '',
  sequence int NOT NULL DEFAULT 0,
  saved_at timestamptz NOT NULL DEFAULT now()
);
