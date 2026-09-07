CREATE TABLE IF NOT EXISTS customer_kyc_requests (
  tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  request_id text NOT NULL,
  customer_id uuid NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
  token_hash text NOT NULL,
  kinds jsonb NOT NULL DEFAULT '[]'::jsonb,
  status text NOT NULL DEFAULT 'active',
  note text,
  created_by text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL,
  submitted_at timestamptz,
  revoked_at timestamptz,
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, request_id),
  UNIQUE (tenant_id, token_hash),
  CHECK (status IN ('active', 'submitted', 'revoked', 'expired')),
  CHECK (jsonb_typeof(kinds) = 'array')
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_customer_kyc_requests_one_active
  ON customer_kyc_requests (tenant_id, customer_id)
  WHERE status = 'active';

CREATE INDEX IF NOT EXISTS idx_customer_kyc_requests_customer_created
  ON customer_kyc_requests (tenant_id, customer_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_customer_kyc_requests_expiry
  ON customer_kyc_requests (tenant_id, status, expires_at);

ALTER TABLE customer_kyc_requests ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_customer_kyc_requests ON customer_kyc_requests
  USING (tenant_id = current_setting('app.current_tenant_id')::uuid);
