package email

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"chakuchuri/backend/internal/platform/tenantdb"
)

const defaultTenantExternalID = "tenant_chakuchuri"

type PostgresRepository struct {
	db      *sql.DB
	timeout time.Duration
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db, timeout: 5 * time.Second}
}

func (r *PostgresRepository) LoadState() (State, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()
	tx, tenantID, err := r.begin(ctx)
	if err != nil {
		return State{}, "", err
	}
	defer tx.Rollback()

	state := State{Version: 1}
	var raw []byte
	err = tx.QueryRowContext(ctx, "SELECT app_data FROM email_connections WHERE tenant_id = $1", tenantID).Scan(&raw)
	if err == nil {
		if err := json.Unmarshal(raw, &state.Settings); err != nil {
			return State{}, "", fmt.Errorf("decode email settings: %w", err)
		}
	} else if err != sql.ErrNoRows {
		return State{}, "", fmt.Errorf("load email settings: %w", err)
	}
	if state.Templates, err = loadRows[Template](ctx, tx, "email_templates", tenantID, "updated_at DESC"); err != nil {
		return State{}, "", err
	}
	if state.Rules, err = loadRows[AutomationRule](ctx, tx, "email_automation_rules", tenantID, "updated_at DESC"); err != nil {
		return State{}, "", err
	}
	if state.Outbox, err = loadRows[OutboxItem](ctx, tx, "email_outbox", tenantID, "created_at DESC"); err != nil {
		return State{}, "", err
	}
	if state.Deliveries, err = loadRows[Delivery](ctx, tx, "email_deliveries", tenantID, "created_at DESC"); err != nil {
		return State{}, "", err
	}
	if err := tx.Commit(); err != nil {
		return State{}, "", fmt.Errorf("commit email load: %w", err)
	}
	if state.Settings.FromEmail == "" && len(state.Templates) == 0 && len(state.Rules) == 0 {
		return State{}, "", errInvalidEmailSnapshot
	}
	return state, "postgres.email", nil
}

func (r *PostgresRepository) SaveState(state State) error {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()
	tx, tenantID, err := r.begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	settingsRaw, err := json.Marshal(state.Settings)
	if err != nil {
		return fmt.Errorf("encode email settings: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO email_connections (tenant_id, provider, mode, from_email, reply_to, configured, app_data, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, now())
ON CONFLICT (tenant_id) DO UPDATE SET provider=EXCLUDED.provider, mode=EXCLUDED.mode,
from_email=EXCLUDED.from_email, reply_to=EXCLUDED.reply_to, configured=EXCLUDED.configured,
app_data=EXCLUDED.app_data, updated_at=now()`,
		tenantID, state.Settings.Provider, state.Settings.Mode, state.Settings.FromEmail,
		state.Settings.ReplyTo, state.Settings.Configured, settingsRaw); err != nil {
		return fmt.Errorf("save email settings: %w", err)
	}

	for _, row := range state.Templates {
		raw, err := json.Marshal(row)
		if err != nil {
			return fmt.Errorf("encode email template %s: %w", row.ID, err)
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO email_templates (tenant_id, template_id, template_key, language, category, enabled, version, app_data, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8::jsonb,now())
ON CONFLICT (tenant_id, template_id) DO UPDATE SET template_key=EXCLUDED.template_key,
language=EXCLUDED.language, category=EXCLUDED.category, enabled=EXCLUDED.enabled,
version=EXCLUDED.version, app_data=EXCLUDED.app_data, updated_at=now()`,
			tenantID, row.ID, row.Key, row.Language, row.Category, row.Enabled, row.Version, raw); err != nil {
			return fmt.Errorf("save email template %s: %w", row.ID, err)
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO email_template_versions (tenant_id, template_id, version, app_data)
VALUES ($1,$2,$3,$4::jsonb)
ON CONFLICT (tenant_id, template_id, version) DO NOTHING`,
			tenantID, row.ID, row.Version, raw); err != nil {
			return fmt.Errorf("save email template version %s: %w", row.ID, err)
		}
	}
	for _, row := range state.Rules {
		raw, err := json.Marshal(row)
		if err != nil {
			return fmt.Errorf("encode email rule %s: %w", row.ID, err)
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO email_automation_rules (tenant_id, rule_id, trigger_key, enabled, timing, delay_minutes, max_reminders, app_data, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8::jsonb,now())
ON CONFLICT (tenant_id, rule_id) DO UPDATE SET trigger_key=EXCLUDED.trigger_key,
enabled=EXCLUDED.enabled, timing=EXCLUDED.timing, delay_minutes=EXCLUDED.delay_minutes,
max_reminders=EXCLUDED.max_reminders, app_data=EXCLUDED.app_data, updated_at=now()`,
			tenantID, row.ID, row.Trigger, row.Enabled, row.Timing, row.DelayMinutes, row.MaxReminders, raw); err != nil {
			return fmt.Errorf("save email rule %s: %w", row.ID, err)
		}
	}
	for _, row := range state.Outbox {
		raw, err := json.Marshal(row)
		if err != nil {
			return fmt.Errorf("encode email outbox %s: %w", row.ID, err)
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO email_outbox (tenant_id,outbox_id,event_key,trigger_key,customer_external_id,
entity_external_id,recipient,status,attempts,scheduled_at,app_data,created_at,updated_at)
VALUES ($1,$2,$3,$4,NULLIF($5,''),NULLIF($6,''),$7,$8,$9,$10,$11::jsonb,$12,now())
ON CONFLICT (tenant_id, outbox_id) DO UPDATE SET status=EXCLUDED.status,
attempts=EXCLUDED.attempts, scheduled_at=EXCLUDED.scheduled_at, app_data=EXCLUDED.app_data, updated_at=now()`,
			tenantID, row.ID, row.EventKey, row.Trigger, row.CustomerID, row.EntityID, row.Recipient,
			row.Status, row.Attempts, parseTime(row.ScheduledAt), raw, parseTime(row.CreatedAt)); err != nil {
			return fmt.Errorf("save email outbox %s: %w", row.ID, err)
		}
	}
	for _, row := range state.Deliveries {
		raw, err := json.Marshal(row)
		if err != nil {
			return fmt.Errorf("encode email delivery %s: %w", row.ID, err)
		}
		var sentAt interface{}
		if row.SentAt != "" {
			sentAt = parseTime(row.SentAt)
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO email_deliveries (tenant_id,delivery_id,outbox_id,event_key,trigger_key,
customer_external_id,entity_external_id,recipient,status,provider,provider_message_id,
attempts,app_data,created_at,sent_at)
VALUES ($1,$2,$3,$4,$5,NULLIF($6,''),NULLIF($7,''),$8,$9,$10,NULLIF($11,''),$12,$13::jsonb,$14,$15)
ON CONFLICT (tenant_id, delivery_id) DO UPDATE SET status=EXCLUDED.status,
provider=EXCLUDED.provider, provider_message_id=EXCLUDED.provider_message_id,
attempts=EXCLUDED.attempts, app_data=EXCLUDED.app_data, sent_at=EXCLUDED.sent_at`,
			tenantID, row.ID, row.OutboxID, row.EventKey, row.Trigger, row.CustomerID, row.EntityID,
			row.Recipient, row.Status, row.Provider, row.ProviderMessageID, row.Attempts, raw,
			parseTime(row.CreatedAt), sentAt); err != nil {
			return fmt.Errorf("save email delivery %s: %w", row.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit email save: %w", err)
	}
	return nil
}

func (r *PostgresRepository) begin(ctx context.Context) (*sql.Tx, string, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, "", fmt.Errorf("begin email repository transaction: %w", err)
	}
	tenantID, err := tenantdb.EnsureTenant(ctx, tx, defaultTenantExternalID, defaultTenantExternalID)
	if err != nil {
		tx.Rollback()
		return nil, "", err
	}
	if err := tenantdb.SetTenant(ctx, tx, tenantID); err != nil {
		tx.Rollback()
		return nil, "", err
	}
	return tx, tenantID, nil
}

func loadRows[T any](ctx context.Context, tx *sql.Tx, table string, tenantID string, orderBy string) ([]T, error) {
	query := fmt.Sprintf("SELECT app_data FROM %s WHERE tenant_id = $1 ORDER BY %s", table, orderBy)
	rows, err := tx.QueryContext(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("load %s: %w", table, err)
	}
	defer rows.Close()
	result := []T{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("scan %s: %w", table, err)
		}
		var row T
		if err := json.Unmarshal(raw, &row); err != nil {
			return nil, fmt.Errorf("decode %s: %w", table, err)
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate %s: %w", table, err)
	}
	return result, nil
}

func parseTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Now().UTC()
	}
	return parsed
}
