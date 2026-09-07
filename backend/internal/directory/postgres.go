package directory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"chakuchuri/backend/internal/platform/tenantdb"
)

const defaultDirectoryTenant = "tenant_chakuchuri"

type PostgresRepository struct {
	db      *sql.DB
	timeout time.Duration
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db, timeout: 5 * time.Second}
}

func (r *PostgresRepository) begin(ctx context.Context) (*sql.Tx, string, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, "", fmt.Errorf("begin directory tx: %w", err)
	}
	tenantID, err := tenantdb.EnsureTenant(ctx, tx, defaultDirectoryTenant, defaultDirectoryTenant)
	if err != nil {
		_ = tx.Rollback()
		return nil, "", err
	}
	if err := tenantdb.SetTenant(ctx, tx, tenantID); err != nil {
		_ = tx.Rollback()
		return nil, "", err
	}
	return tx, tenantID, nil
}

func (r *PostgresRepository) LoadState() (State, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()
	tx, tenantID, err := r.begin(ctx)
	if err != nil {
		return State{}, "", err
	}
	defer tx.Rollback()

	var raw []byte
	err = tx.QueryRowContext(ctx, `
SELECT app_data
FROM directory_state
WHERE tenant_id = $1 AND external_id = 'main'`, tenantID).Scan(&raw)
	if err == sql.ErrNoRows {
		return State{}, "", errInvalidSnapshot
	}
	if err != nil {
		return State{}, "", fmt.Errorf("load directory state: %w", err)
	}
	var state State
	if err := json.Unmarshal(raw, &state); err != nil {
		return State{}, "", fmt.Errorf("decode directory state: %w", err)
	}
	if state.Version != 1 {
		return State{}, "", errInvalidSnapshot
	}
	if state.Categories == nil {
		state.Categories = []Category{}
	}
	if state.Listings == nil {
		state.Listings = []Listing{}
	}
	if err := tx.Commit(); err != nil {
		return State{}, "", fmt.Errorf("commit directory load: %w", err)
	}
	return state, "postgres.directory_state", nil
}

func (r *PostgresRepository) SaveState(state State) error {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()
	tx, tenantID, err := r.begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	state.Version = 1
	raw, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode directory state: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO directory_state (tenant_id, external_id, app_data, updated_at)
VALUES ($1, 'main', $2::jsonb, now())
ON CONFLICT (tenant_id, external_id) DO UPDATE SET
  app_data = EXCLUDED.app_data,
  updated_at = now()`, tenantID, raw); err != nil {
		return fmt.Errorf("save directory state: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit directory save: %w", err)
	}
	return nil
}
