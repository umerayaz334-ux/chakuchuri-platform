package ratesheets

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"chakuchuri/backend/internal/platform/tenantdb"
)

const defaultRateBookTenant = "tenant_chakuchuri"

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

	rows, err := tx.QueryContext(ctx, `
SELECT app_data
FROM shipping_rate_books
WHERE tenant_id = $1
ORDER BY imported_at DESC, created_at DESC`, tenantID)
	if err != nil {
		return State{}, "", fmt.Errorf("load shipping rate books: %w", err)
	}
	defer rows.Close()
	state := State{Version: 1, Books: []RateBook{}}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return State{}, "", fmt.Errorf("scan shipping rate book: %w", err)
		}
		var book RateBook
		if err := json.Unmarshal(raw, &book); err != nil {
			return State{}, "", fmt.Errorf("decode shipping rate book: %w", err)
		}
		state.Books = append(state.Books, book)
	}
	if err := rows.Err(); err != nil {
		return State{}, "", fmt.Errorf("iterate shipping rate books: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return State{}, "", fmt.Errorf("commit shipping rate load: %w", err)
	}
	if len(state.Books) == 0 {
		return State{}, "", errInvalidRateBookSnapshot
	}
	return state, "postgres.shipping_rate_books", nil
}

func (r *PostgresRepository) SaveState(state State) error {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()
	tx, tenantID, err := r.begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, book := range state.Books {
		raw, err := json.Marshal(book)
		if err != nil {
			return fmt.Errorf("encode shipping rate book %s: %w", book.ID, err)
		}
		importedAt := time.Now().UTC()
		if parsed, parseErr := time.Parse(time.RFC3339, book.ImportedAt); parseErr == nil {
			importedAt = parsed
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO shipping_rate_books (
  tenant_id, book_id, name, carrier, country_code, currency, effective_date,
  version, status, source_file_external_id, source_name, imported_by, imported_at, app_data, updated_at
)
VALUES ($1,$2,$3,$4,$5,$6,NULLIF($7,'')::date,$8,$9,NULLIF($10,''),NULLIF($11,''),$12,$13,$14::jsonb,now())
ON CONFLICT (tenant_id, book_id) DO UPDATE SET
  name=EXCLUDED.name, carrier=EXCLUDED.carrier, country_code=EXCLUDED.country_code,
  currency=EXCLUDED.currency, effective_date=EXCLUDED.effective_date, version=EXCLUDED.version,
  status=EXCLUDED.status, source_file_external_id=EXCLUDED.source_file_external_id,
  source_name=EXCLUDED.source_name, imported_by=EXCLUDED.imported_by,
  imported_at=EXCLUDED.imported_at, app_data=EXCLUDED.app_data, updated_at=now()`,
			tenantID, book.ID, book.Name, book.Carrier, book.Country, book.Currency, book.EffectiveDate,
			book.Version, book.Status, book.SourceFileID, book.SourceName, book.ImportedBy, importedAt, raw); err != nil {
			return fmt.Errorf("save shipping rate book %s: %w", book.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit shipping rate save: %w", err)
	}
	return nil
}

func (r *PostgresRepository) begin(ctx context.Context) (*sql.Tx, string, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, "", fmt.Errorf("begin shipping rate transaction: %w", err)
	}
	tenantID, err := tenantdb.EnsureTenant(ctx, tx, defaultRateBookTenant, defaultRateBookTenant)
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
