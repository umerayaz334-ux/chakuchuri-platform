CREATE TABLE IF NOT EXISTS directory_state (
  tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  external_id text NOT NULL DEFAULT 'main',
  app_data jsonb NOT NULL DEFAULT '{}',
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, external_id)
);
