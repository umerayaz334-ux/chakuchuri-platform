DROP TABLE IF EXISTS workflow_state_meta;

DROP INDEX IF EXISTS idx_calls_tenant_external_id;
ALTER TABLE call_queue_entries DROP COLUMN IF EXISTS app_data;
ALTER TABLE call_queue_entries DROP COLUMN IF EXISTS external_id;

DROP INDEX IF EXISTS idx_messages_conversation_external_id;
ALTER TABLE messages DROP COLUMN IF EXISTS app_data;
ALTER TABLE messages DROP COLUMN IF EXISTS external_id;

DROP INDEX IF EXISTS idx_conversations_tenant_external_id;
ALTER TABLE conversations DROP COLUMN IF EXISTS app_data;
ALTER TABLE conversations DROP COLUMN IF EXISTS external_id;

DROP INDEX IF EXISTS idx_payments_tenant_external_id;
ALTER TABLE payment_confirmations DROP COLUMN IF EXISTS app_data;
ALTER TABLE payment_confirmations DROP COLUMN IF EXISTS external_id;

DROP INDEX IF EXISTS idx_shipping_tenant_external_id;
ALTER TABLE shipping_requests DROP COLUMN IF EXISTS app_data;
ALTER TABLE shipping_requests DROP COLUMN IF EXISTS external_id;

DROP INDEX IF EXISTS idx_rate_sheets_tenant_external_id;
ALTER TABLE shipping_rate_sheets DROP COLUMN IF EXISTS app_data;
ALTER TABLE shipping_rate_sheets DROP COLUMN IF EXISTS external_id;

DROP INDEX IF EXISTS idx_manufacturing_tenant_external_id;
ALTER TABLE manufacturing_orders DROP COLUMN IF EXISTS app_data;
ALTER TABLE manufacturing_orders DROP COLUMN IF EXISTS external_id;

DROP INDEX IF EXISTS idx_quotations_tenant_external_id;
ALTER TABLE quotations DROP COLUMN IF EXISTS app_data;
ALTER TABLE quotations DROP COLUMN IF EXISTS external_id;

DROP INDEX IF EXISTS idx_products_tenant_external_id;
ALTER TABLE products DROP COLUMN IF EXISTS app_data;
ALTER TABLE products DROP COLUMN IF EXISTS external_id;
