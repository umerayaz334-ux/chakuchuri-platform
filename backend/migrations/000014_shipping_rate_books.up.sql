CREATE TABLE IF NOT EXISTS shipping_rate_books (
  tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  book_id text NOT NULL,
  name text NOT NULL,
  carrier text NOT NULL,
  country_code text NOT NULL DEFAULT 'US',
  currency text NOT NULL DEFAULT 'PKR',
  effective_date date,
  version int NOT NULL CHECK (version > 0),
  status text NOT NULL DEFAULT 'Paused',
  source_file_external_id text,
  source_name text,
  imported_by text NOT NULL,
  imported_at timestamptz NOT NULL,
  app_data jsonb NOT NULL DEFAULT '{}',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, book_id),
  UNIQUE (tenant_id, carrier, country_code, version)
);

CREATE INDEX IF NOT EXISTS idx_shipping_rate_books_active
  ON shipping_rate_books (tenant_id, country_code, carrier, effective_date DESC)
  WHERE status = 'Active';

CREATE INDEX IF NOT EXISTS idx_shipping_rate_books_imported
  ON shipping_rate_books (tenant_id, imported_at DESC);

ALTER TABLE shipping_rate_books ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_shipping_rate_books ON shipping_rate_books
  USING (tenant_id = current_setting('app.current_tenant_id')::uuid);
