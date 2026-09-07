package files

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
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

func (r *PostgresRepository) LoadFiles() ([]FileRecord, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, "", fmt.Errorf("begin files load: %w", err)
	}
	defer tx.Rollback()

	tenantID, err := tenantdb.EnsureTenant(ctx, tx, defaultTenantExternalID, defaultTenantExternalID)
	if err != nil {
		return nil, "", err
	}
	if err := tenantdb.SetTenant(ctx, tx, tenantID); err != nil {
		return nil, "", err
	}

	rows, err := tx.QueryContext(ctx, `
SELECT app_data
FROM files
WHERE tenant_id = $1
ORDER BY created_at DESC`, tenantID)
	if err != nil {
		return nil, "", fmt.Errorf("query files: %w", err)
	}
	defer rows.Close()

	records := []FileRecord{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, "", fmt.Errorf("scan file: %w", err)
		}
		var record FileRecord
		if decodeRecord(raw, &record) {
			records = append(records, record)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("iterate files: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, "", fmt.Errorf("commit files load: %w", err)
	}
	return records, "postgres.files", nil
}

func (r *PostgresRepository) SaveFiles(records []FileRecord) error {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin files save: %w", err)
	}
	defer tx.Rollback()

	tenantIDs := map[string]string{}
	customerIDs := map[string]string{}
	for _, record := range records {
		if strings.TrimSpace(record.ID) == "" || strings.TrimSpace(record.StorageKey) == "" {
			continue
		}

		tenantExternalID := tenantdb.FirstNonEmpty(record.TenantID, defaultTenantExternalID)
		tenantID := tenantIDs[tenantExternalID]
		if tenantID == "" {
			tenantID, err = tenantdb.EnsureTenant(ctx, tx, tenantExternalID, defaultTenantExternalID)
			if err != nil {
				return err
			}
			tenantIDs[tenantExternalID] = tenantID
		}
		if err := tenantdb.SetTenant(ctx, tx, tenantID); err != nil {
			return err
		}

		customerID, err := ensureCustomer(ctx, tx, tenantID, customerIDs, record.CustomerID)
		if err != nil {
			return err
		}

		appData, err := json.Marshal(record)
		if err != nil {
			return fmt.Errorf("encode file %s: %w", record.ID, err)
		}

		if _, err := tx.ExecContext(ctx, `
INSERT INTO files (
  tenant_id, customer_id, external_id, owner_type, owner_id, original_name,
  storage_key, thumbnail_storage_key, mime_type, byte_size, width, height,
  checksum_sha256, processing_status, app_data, created_at
) VALUES ($1, $2, $3, $4, NULL, $5, $6, $7, $8, $9, $10, $11, $12, 'ready', $13::jsonb, COALESCE($14, now()))
ON CONFLICT (tenant_id, external_id) DO UPDATE SET
  customer_id = EXCLUDED.customer_id,
  owner_type = EXCLUDED.owner_type,
  original_name = EXCLUDED.original_name,
  storage_key = EXCLUDED.storage_key,
  thumbnail_storage_key = EXCLUDED.thumbnail_storage_key,
  mime_type = EXCLUDED.mime_type,
  byte_size = EXCLUDED.byte_size,
  width = EXCLUDED.width,
  height = EXCLUDED.height,
  checksum_sha256 = EXCLUDED.checksum_sha256,
  processing_status = 'ready',
  app_data = EXCLUDED.app_data`,
			tenantID,
			customerID,
			record.ID,
			tenantdb.FirstNonEmpty(record.OwnerType, "general"),
			tenantdb.FirstNonEmpty(record.OriginalName, record.ID),
			record.StorageKey,
			nullableText(record.ThumbnailKey),
			record.MimeType,
			record.ByteSize,
			nullableInt(record.Width),
			nullableInt(record.Height),
			record.Checksum,
			appData,
			nullableTime(record.CreatedAt),
		); err != nil {
			return fmt.Errorf("save file %s: %w", record.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit files save: %w", err)
	}
	return nil
}

func (r *PostgresRepository) SaveFile(record FileRecord) error {
	if strings.TrimSpace(record.ID) == "" || strings.TrimSpace(record.StorageKey) == "" || strings.TrimSpace(record.TenantID) == "" {
		return fmt.Errorf("file id, tenant and storage key are required")
	}
	return r.SaveFiles([]FileRecord{record})
}

func (r *PostgresRepository) Check() error {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()
	_, err := r.db.ExecContext(ctx, "SELECT tenant_id, external_id, app_data FROM files LIMIT 0")
	return err
}

func (r *PostgresRepository) GetFile(tenantID, id string) (FileRecord, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return FileRecord{}, false, err
	}
	defer tx.Rollback()
	var internalTenant string
	err = tx.QueryRowContext(ctx, "SELECT id FROM tenants WHERE external_id = $1", tenantID).Scan(&internalTenant)
	if err == sql.ErrNoRows {
		return FileRecord{}, false, nil
	}
	if err != nil {
		return FileRecord{}, false, err
	}
	if err := tenantdb.SetTenant(ctx, tx, internalTenant); err != nil {
		return FileRecord{}, false, err
	}
	var raw []byte
	err = tx.QueryRowContext(ctx, "SELECT app_data FROM files WHERE tenant_id = $1 AND external_id = $2", internalTenant, id).Scan(&raw)
	if err == sql.ErrNoRows {
		return FileRecord{}, false, nil
	}
	if err != nil {
		return FileRecord{}, false, err
	}
	var record FileRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return FileRecord{}, false, err
	}
	if record.TenantID != tenantID || record.ID != id {
		return FileRecord{}, false, fmt.Errorf("file metadata identity mismatch")
	}
	if err := tx.Commit(); err != nil {
		return FileRecord{}, false, err
	}
	return record, true, nil
}

func (r *PostgresRepository) DeleteFile(record FileRecord) error {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin file delete: %w", err)
	}
	defer tx.Rollback()

	tenantExternalID := tenantdb.FirstNonEmpty(record.TenantID, defaultTenantExternalID)
	tenantID, err := tenantdb.EnsureTenant(ctx, tx, tenantExternalID, defaultTenantExternalID)
	if err != nil {
		return err
	}
	if err := tenantdb.SetTenant(ctx, tx, tenantID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM files WHERE tenant_id = $1 AND external_id = $2`, tenantID, record.ID); err != nil {
		return fmt.Errorf("delete file %s: %w", record.ID, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit file delete: %w", err)
	}
	return nil
}
func ensureCustomer(ctx context.Context, tx *sql.Tx, tenantID string, cache map[string]string, externalID string) (any, error) {
	externalID = strings.TrimSpace(externalID)
	if externalID == "" {
		return nil, nil
	}
	if cached := cache[externalID]; cached != "" {
		return cached, nil
	}

	name := strings.ReplaceAll(strings.TrimPrefix(externalID, "cust_"), "_", " ")
	var customerID string
	if err := tx.QueryRowContext(ctx, `
INSERT INTO customers (
  tenant_id, external_id, company_name, status, service_flags, portal_data, updated_at
) VALUES ($1, $2, $3, 'active', '{}', '{}', now())
ON CONFLICT (tenant_id, external_id) DO UPDATE SET
  updated_at = now(),
  deleted_at = NULL
RETURNING id::text`, tenantID, externalID, tenantdb.TitleLike(name)).Scan(&customerID); err != nil {
		return nil, fmt.Errorf("ensure file customer %s: %w", externalID, err)
	}
	cache[externalID] = customerID
	return customerID, nil
}

func decodeRecord(raw []byte, target *FileRecord) bool {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "{}" || trimmed == "null" {
		return false
	}
	return json.Unmarshal(raw, target) == nil
}

func nullableText(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func nullableInt(value int) any {
	if value <= 0 {
		return nil
	}
	return value
}

func nullableTime(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil
	}
	return parsed.UTC()
}
