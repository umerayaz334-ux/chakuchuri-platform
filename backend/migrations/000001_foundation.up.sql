CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE tenants (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  slug text NOT NULL UNIQUE,
  name text NOT NULL,
  status text NOT NULL DEFAULT 'active',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE users (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id),
  email citext NOT NULL,
  name text NOT NULL,
  password_hash text NOT NULL,
  role text NOT NULL,
  status text NOT NULL DEFAULT 'active',
  last_online_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, email)
);

CREATE TABLE customers (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id),
  company_name text NOT NULL,
  contact_name text,
  email text,
  phone text,
  country text,
  status text NOT NULL DEFAULT 'active',
  service_flags jsonb NOT NULL DEFAULT '{}',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);

CREATE TABLE customer_users (
  customer_id uuid NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (customer_id, user_id)
);

CREATE TABLE audit_events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid REFERENCES tenants(id),
  actor_user_id uuid REFERENCES users(id),
  entity_type text NOT NULL,
  entity_id uuid,
  action text NOT NULL,
  before_data jsonb,
  after_data jsonb,
  ip_address inet,
  user_agent text,
  request_id text,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE files (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id),
  customer_id uuid REFERENCES customers(id),
  owner_type text NOT NULL,
  owner_id uuid,
  original_name text NOT NULL,
  storage_key text NOT NULL UNIQUE,
  mime_type text NOT NULL,
  byte_size bigint NOT NULL,
  width int,
  height int,
  checksum_sha256 text,
  processing_status text NOT NULL DEFAULT 'pending',
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE products (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id),
  customer_id uuid NOT NULL REFERENCES customers(id),
  sku text NOT NULL,
  name text NOT NULL,
  description text,
  quantity_on_hand int NOT NULL DEFAULT 0,
  reserved_quantity int NOT NULL DEFAULT 0,
  image_file_id uuid REFERENCES files(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  UNIQUE (tenant_id, customer_id, sku)
);

CREATE TABLE quotations (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id),
  customer_id uuid NOT NULL REFERENCES customers(id),
  quote_no text NOT NULL,
  product_name text NOT NULL,
  quantity int NOT NULL DEFAULT 1,
  status text NOT NULL DEFAULT 'requested',
  quoted_total numeric(14,2),
  deposit_required numeric(14,2),
  currency text NOT NULL DEFAULT 'PKR',
  estimated_ready_at date,
  spec_data jsonb NOT NULL DEFAULT '{}',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, quote_no)
);

CREATE TABLE manufacturing_orders (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id),
  customer_id uuid NOT NULL REFERENCES customers(id),
  quotation_id uuid REFERENCES quotations(id),
  order_no text NOT NULL,
  status text NOT NULL DEFAULT 'confirmed',
  progress_percent int NOT NULL DEFAULT 0,
  estimated_ready_at date,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, order_no)
);

CREATE TABLE manufacturing_timeline_events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id),
  manufacturing_order_id uuid NOT NULL REFERENCES manufacturing_orders(id) ON DELETE CASCADE,
  stage text NOT NULL,
  note text,
  event_at timestamptz NOT NULL DEFAULT now(),
  actor_user_id uuid REFERENCES users(id)
);

CREATE TABLE shipping_rate_sheets (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id),
  courier text NOT NULL,
  service_name text NOT NULL,
  version text NOT NULL,
  currency text NOT NULL DEFAULT 'PKR',
  source_file_id uuid REFERENCES files(id),
  is_active boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, courier, service_name, version)
);

CREATE TABLE shipping_requests (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id),
  customer_id uuid NOT NULL REFERENCES customers(id),
  manufacturing_order_id uuid REFERENCES manufacturing_orders(id),
  request_no text NOT NULL,
  request_type text NOT NULL,
  courier text,
  service_name text,
  status text NOT NULL DEFAULT 'pending',
  destination_country text NOT NULL,
  destination_postal_code text,
  package_data jsonb NOT NULL DEFAULT '{}',
  estimated_cost numeric(14,2),
  confirmed_cost numeric(14,2),
  tracking_number text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, request_no)
);

CREATE TABLE ledger_entries (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id),
  customer_id uuid NOT NULL REFERENCES customers(id),
  entry_no text NOT NULL,
  source_type text NOT NULL,
  source_id uuid,
  debit numeric(14,2) NOT NULL DEFAULT 0,
  credit numeric(14,2) NOT NULL DEFAULT 0,
  currency text NOT NULL DEFAULT 'PKR',
  note text,
  posted_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, entry_no)
);

CREATE TABLE payment_confirmations (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id),
  customer_id uuid NOT NULL REFERENCES customers(id),
  amount numeric(14,2) NOT NULL,
  currency text NOT NULL DEFAULT 'PKR',
  status text NOT NULL DEFAULT 'pending',
  proof_file_id uuid REFERENCES files(id),
  confirmed_by uuid REFERENCES users(id),
  confirmed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE conversations (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id),
  customer_id uuid REFERENCES customers(id),
  visitor_fingerprint text,
  status text NOT NULL DEFAULT 'open',
  last_message_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE messages (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id),
  conversation_id uuid NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
  sender_type text NOT NULL,
  sender_user_id uuid REFERENCES users(id),
  body text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE call_queue_entries (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL REFERENCES tenants(id),
  customer_id uuid REFERENCES customers(id),
  conversation_id uuid REFERENCES conversations(id),
  status text NOT NULL DEFAULT 'waiting',
  priority int NOT NULL DEFAULT 100,
  requested_at timestamptz NOT NULL DEFAULT now(),
  answered_at timestamptz,
  ended_at timestamptz
);

CREATE INDEX idx_customers_tenant_status ON customers (tenant_id, status);
CREATE INDEX idx_products_customer_sku ON products (tenant_id, customer_id, sku);
CREATE INDEX idx_quotations_customer_status ON quotations (tenant_id, customer_id, status);
CREATE INDEX idx_manufacturing_customer_status ON manufacturing_orders (tenant_id, customer_id, status);
CREATE INDEX idx_shipping_customer_status ON shipping_requests (tenant_id, customer_id, status);
CREATE INDEX idx_ledger_customer_posted ON ledger_entries (tenant_id, customer_id, posted_at DESC);
CREATE INDEX idx_messages_conversation_created ON messages (conversation_id, created_at);
CREATE INDEX idx_call_queue_status_priority ON call_queue_entries (tenant_id, status, priority, requested_at);
CREATE INDEX idx_audit_tenant_created ON audit_events (tenant_id, created_at DESC);

ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE customers ENABLE ROW LEVEL SECURITY;
ALTER TABLE products ENABLE ROW LEVEL SECURITY;
ALTER TABLE quotations ENABLE ROW LEVEL SECURITY;
ALTER TABLE manufacturing_orders ENABLE ROW LEVEL SECURITY;
ALTER TABLE shipping_requests ENABLE ROW LEVEL SECURITY;
ALTER TABLE ledger_entries ENABLE ROW LEVEL SECURITY;
ALTER TABLE conversations ENABLE ROW LEVEL SECURITY;
ALTER TABLE messages ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_users ON users USING (tenant_id = current_setting('app.current_tenant_id')::uuid);
CREATE POLICY tenant_customers ON customers USING (tenant_id = current_setting('app.current_tenant_id')::uuid);
CREATE POLICY tenant_products ON products USING (tenant_id = current_setting('app.current_tenant_id')::uuid);
CREATE POLICY tenant_quotations ON quotations USING (tenant_id = current_setting('app.current_tenant_id')::uuid);
CREATE POLICY tenant_manufacturing ON manufacturing_orders USING (tenant_id = current_setting('app.current_tenant_id')::uuid);
CREATE POLICY tenant_shipping ON shipping_requests USING (tenant_id = current_setting('app.current_tenant_id')::uuid);
CREATE POLICY tenant_ledger ON ledger_entries USING (tenant_id = current_setting('app.current_tenant_id')::uuid);
CREATE POLICY tenant_conversations ON conversations USING (tenant_id = current_setting('app.current_tenant_id')::uuid);
CREATE POLICY tenant_messages ON messages USING (tenant_id = current_setting('app.current_tenant_id')::uuid);

