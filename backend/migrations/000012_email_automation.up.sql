CREATE TABLE IF NOT EXISTS email_connections (
  tenant_id uuid PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
  provider text NOT NULL DEFAULT 'development',
  mode text NOT NULL DEFAULT 'Development',
  from_email citext NOT NULL,
  reply_to citext NOT NULL,
  configured boolean NOT NULL DEFAULT false,
  app_data jsonb NOT NULL DEFAULT '{}',
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS email_templates (
  tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  template_id text NOT NULL,
  template_key text NOT NULL,
  language text NOT NULL,
  category text NOT NULL,
  enabled boolean NOT NULL DEFAULT true,
  version int NOT NULL DEFAULT 1,
  app_data jsonb NOT NULL DEFAULT '{}',
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, template_id),
  UNIQUE (tenant_id, template_key, language)
);

CREATE TABLE IF NOT EXISTS email_template_versions (
  tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  template_id text NOT NULL,
  version int NOT NULL,
  app_data jsonb NOT NULL DEFAULT '{}',
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, template_id, version)
);

CREATE TABLE IF NOT EXISTS email_automation_rules (
  tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  rule_id text NOT NULL,
  trigger_key text NOT NULL,
  enabled boolean NOT NULL DEFAULT true,
  timing text NOT NULL DEFAULT 'Immediate',
  delay_minutes int NOT NULL DEFAULT 0 CHECK (delay_minutes >= 0),
  max_reminders int NOT NULL DEFAULT 0 CHECK (max_reminders >= 0),
  app_data jsonb NOT NULL DEFAULT '{}',
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, rule_id),
  UNIQUE (tenant_id, trigger_key)
);

CREATE TABLE IF NOT EXISTS email_outbox (
  tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  outbox_id text NOT NULL,
  event_key text NOT NULL,
  trigger_key text NOT NULL,
  customer_external_id text,
  entity_external_id text,
  recipient citext NOT NULL,
  status text NOT NULL,
  attempts int NOT NULL DEFAULT 0 CHECK (attempts >= 0),
  scheduled_at timestamptz NOT NULL,
  app_data jsonb NOT NULL DEFAULT '{}',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, outbox_id),
  UNIQUE (tenant_id, event_key)
);

CREATE TABLE IF NOT EXISTS email_deliveries (
  tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  delivery_id text NOT NULL,
  outbox_id text NOT NULL,
  event_key text NOT NULL,
  trigger_key text NOT NULL,
  customer_external_id text,
  entity_external_id text,
  recipient citext NOT NULL,
  status text NOT NULL,
  provider text NOT NULL,
  provider_message_id text,
  attempts int NOT NULL DEFAULT 0 CHECK (attempts >= 0),
  app_data jsonb NOT NULL DEFAULT '{}',
  created_at timestamptz NOT NULL DEFAULT now(),
  sent_at timestamptz,
  PRIMARY KEY (tenant_id, delivery_id),
  UNIQUE (tenant_id, outbox_id)
);

CREATE TABLE IF NOT EXISTS email_delivery_events (
  id bigserial PRIMARY KEY,
  tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  delivery_id text NOT NULL,
  event_type text NOT NULL,
  provider_event_id text,
  event_data jsonb NOT NULL DEFAULT '{}',
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS notification_preferences (
  tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  customer_external_id text NOT NULL,
  category text NOT NULL,
  email_enabled boolean NOT NULL DEFAULT true,
  language text NOT NULL DEFAULT 'en',
  quiet_hours jsonb NOT NULL DEFAULT '{}',
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, customer_external_id, category)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_email_provider_event
  ON email_delivery_events (tenant_id, provider_event_id)
  WHERE provider_event_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_email_outbox_claim
  ON email_outbox (tenant_id, status, scheduled_at, created_at);
CREATE INDEX IF NOT EXISTS idx_email_outbox_customer
  ON email_outbox (tenant_id, customer_external_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_email_deliveries_status_created
  ON email_deliveries (tenant_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_email_deliveries_recipient
  ON email_deliveries (tenant_id, recipient, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_email_delivery_events_delivery
  ON email_delivery_events (tenant_id, delivery_id, created_at DESC);

ALTER TABLE email_connections ENABLE ROW LEVEL SECURITY;
ALTER TABLE email_templates ENABLE ROW LEVEL SECURITY;
ALTER TABLE email_template_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE email_automation_rules ENABLE ROW LEVEL SECURITY;
ALTER TABLE email_outbox ENABLE ROW LEVEL SECURITY;
ALTER TABLE email_deliveries ENABLE ROW LEVEL SECURITY;
ALTER TABLE email_delivery_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE notification_preferences ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_email_connections ON email_connections USING (tenant_id = current_setting('app.current_tenant_id')::uuid);
CREATE POLICY tenant_email_templates ON email_templates USING (tenant_id = current_setting('app.current_tenant_id')::uuid);
CREATE POLICY tenant_email_template_versions ON email_template_versions USING (tenant_id = current_setting('app.current_tenant_id')::uuid);
CREATE POLICY tenant_email_automation_rules ON email_automation_rules USING (tenant_id = current_setting('app.current_tenant_id')::uuid);
CREATE POLICY tenant_email_outbox ON email_outbox USING (tenant_id = current_setting('app.current_tenant_id')::uuid);
CREATE POLICY tenant_email_deliveries ON email_deliveries USING (tenant_id = current_setting('app.current_tenant_id')::uuid);
CREATE POLICY tenant_email_delivery_events ON email_delivery_events USING (tenant_id = current_setting('app.current_tenant_id')::uuid);
CREATE POLICY tenant_notification_preferences ON notification_preferences USING (tenant_id = current_setting('app.current_tenant_id')::uuid);
