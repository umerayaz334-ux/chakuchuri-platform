package workflow

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"chakuchuri/backend/internal/platform/idgen"
	"chakuchuri/backend/internal/platform/tenantdb"
)

const defaultTenantExternalID = "tenant_chakuchuri"

type PostgresRepository struct {
	mu       sync.Mutex
	revision int64
	db       *sql.DB
	timeout  time.Duration
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db, timeout: 5 * time.Second}
}

func (r *PostgresRepository) LoadState() (State, string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return State{}, "", fmt.Errorf("begin workflow load: %w", err)
	}
	defer tx.Rollback()

	tenantID, err := tenantdb.EnsureTenant(ctx, tx, defaultTenantExternalID, defaultTenantExternalID)
	if err != nil {
		return State{}, "", err
	}
	if err := tenantdb.SetTenant(ctx, tx, tenantID); err != nil {
		return State{}, "", err
	}

	state := State{Version: 1}
	if state.Products, err = loadAppDataRows[Product](ctx, tx, "products", tenantID, "updated_at DESC", "deleted_at IS NULL"); err != nil {
		return State{}, "", err
	}
	if state.Quotations, err = loadAppDataRows[Quotation](ctx, tx, "quotations", tenantID, "created_at DESC", "true"); err != nil {
		return State{}, "", err
	}
	if state.Manufacturing, err = loadAppDataRows[ManufacturingOrder](ctx, tx, "manufacturing_orders", tenantID, "created_at DESC", "true"); err != nil {
		return State{}, "", err
	}
	if state.RateSheets, err = loadAppDataRows[RateSheet](ctx, tx, "shipping_rate_sheets", tenantID, "created_at DESC", "true"); err != nil {
		return State{}, "", err
	}
	if state.Shipping, err = loadAppDataRows[ShippingRequest](ctx, tx, "shipping_requests", tenantID, "created_at DESC", "true"); err != nil {
		return State{}, "", err
	}
	if state.Payments, err = loadAppDataRows[Payment](ctx, tx, "payment_confirmations", tenantID, "created_at DESC", "true"); err != nil {
		return State{}, "", err
	}
	if state.Ledger, err = r.loadLedgerEntries(ctx, tx, tenantID); err != nil {
		return State{}, "", err
	}
	if state.Conversations, err = loadAppDataRows[Conversation](ctx, tx, "conversations", tenantID, "created_at DESC", "true"); err != nil {
		return State{}, "", err
	}
	if state.Calls, err = loadAppDataRows[CallRequest](ctx, tx, "calls", tenantID, "initiated_at DESC", "true"); err != nil {
		return State{}, "", err
	}
	if state.CallSignals, err = loadAppDataRows[CallSignal](ctx, tx, "call_signals", tenantID, "signal_no ASC, created_at ASC", "true"); err != nil {
		return State{}, "", err
	}

	var platformSettingsRaw []byte
	var noticesRaw []byte
	var featuredRaw []byte
	var revision int64
	err = tx.QueryRowContext(ctx, `
SELECT last_sequence_day, sequence, platform_settings, customer_notices, featured_products, revision
FROM workflow_state_meta
WHERE tenant_id = $1`, tenantID).Scan(&state.LastSequenceDay, &state.Sequence, &platformSettingsRaw, &noticesRaw, &featuredRaw, &revision)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return State{}, "", fmt.Errorf("load workflow metadata: %w", err)
	}
	if errors.Is(err, sql.ErrNoRows) && !workflowIsEmpty(state) {
		return State{}, "", errors.New("workflow metadata missing for existing records")
	}
	if err == nil {
		for _, field := range []struct {
			name   string
			raw    []byte
			target any
		}{
			{"settings", platformSettingsRaw, &state.Settings},
			{"notices", noticesRaw, &state.Notices},
			{"featured products", featuredRaw, &state.Featured},
		} {
			if err := json.Unmarshal(field.raw, field.target); err != nil {
				return State{}, "", fmt.Errorf("decode workflow %s: %w", field.name, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return State{}, "", fmt.Errorf("commit workflow load: %w", err)
	}
	r.revision = revision
	return state, "postgres.workflow", nil
}

var ErrStaleWorkflow = errors.New("workflow changed in another writer; reload before retrying")

func (r *PostgresRepository) SaveState(state State) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin workflow save: %w", err)
	}
	defer tx.Rollback()

	tenantID, err := tenantdb.EnsureTenant(ctx, tx, defaultTenantExternalID, defaultTenantExternalID)
	if err != nil {
		return err
	}
	if err := tenantdb.SetTenant(ctx, tx, tenantID); err != nil {
		return err
	}

	customerIDs := map[string]string{}
	var revision int64
	err = tx.QueryRowContext(ctx, "SELECT revision FROM workflow_state_meta WHERE tenant_id = $1 FOR UPDATE", tenantID).Scan(&revision)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if revision != r.revision {
		return ErrStaleWorkflow
	}
	quotationIDs := map[string]string{}
	manufacturingIDs := map[string]string{}
	conversationIDs := map[string]string{}
	callIDs := map[string]string{}

	for _, product := range state.Products {
		customerID, err := r.customerID(ctx, tx, tenantID, customerIDs, product.CustomerID)
		if err != nil {
			return err
		}
		if err := r.saveProduct(ctx, tx, tenantID, customerID, product); err != nil {
			return err
		}
	}
	for _, quote := range state.Quotations {
		customerID, err := r.customerID(ctx, tx, tenantID, customerIDs, quote.CustomerID)
		if err != nil {
			return err
		}
		id, err := r.saveQuotation(ctx, tx, tenantID, customerID, quote)
		if err != nil {
			return err
		}
		quotationIDs[quote.ID] = id
	}
	for _, order := range state.Manufacturing {
		customerID, err := r.customerID(ctx, tx, tenantID, customerIDs, order.CustomerID)
		if err != nil {
			return err
		}
		quotationID, err := r.quotationID(ctx, tx, tenantID, quotationIDs, order.QuotationID)
		if err != nil {
			return err
		}
		id, err := r.saveManufacturing(ctx, tx, tenantID, customerID, quotationID, order)
		if err != nil {
			return err
		}
		manufacturingIDs[order.ID] = id
	}
	for _, rate := range state.RateSheets {
		if err := r.saveRateSheet(ctx, tx, tenantID, rate); err != nil {
			return err
		}
	}
	for _, shipping := range state.Shipping {
		customerID, err := r.customerID(ctx, tx, tenantID, customerIDs, shipping.CustomerID)
		if err != nil {
			return err
		}
		manufacturingID, err := r.manufacturingID(ctx, tx, tenantID, manufacturingIDs, shipping.ManufacturingID)
		if err != nil {
			return err
		}
		if err := r.saveShipping(ctx, tx, tenantID, customerID, manufacturingID, shipping); err != nil {
			return err
		}
	}
	for _, payment := range state.Payments {
		customerID, err := r.customerID(ctx, tx, tenantID, customerIDs, payment.CustomerID)
		if err != nil {
			return err
		}
		if err := r.savePayment(ctx, tx, tenantID, customerID, payment); err != nil {
			return err
		}
	}
	ledgerEntries := mergedLedgerEntries(state.Ledger, state.Manufacturing, state.Shipping, state.Payments)
	for _, entry := range ledgerEntries {
		customerID, err := r.customerID(ctx, tx, tenantID, customerIDs, entry.CustomerID)
		if err != nil {
			return err
		}
		if err := r.saveLedgerEntry(ctx, tx, tenantID, customerID, entry); err != nil {
			return err
		}
	}
	for _, conversation := range state.Conversations {
		customerID, err := r.customerID(ctx, tx, tenantID, customerIDs, conversation.CustomerID)
		if err != nil {
			return err
		}
		id, err := r.saveConversation(ctx, tx, tenantID, customerID, conversation)
		if err != nil {
			return err
		}
		conversationIDs[conversation.ID] = id
		if err := r.saveMessages(ctx, tx, tenantID, id, conversation.Messages); err != nil {
			return err
		}
	}
	for _, call := range state.Calls {
		customerID, err := r.customerID(ctx, tx, tenantID, customerIDs, call.CustomerID)
		if err != nil {
			return err
		}
		id, err := r.saveCall(ctx, tx, tenantID, customerID, call)
		if err != nil {
			return err
		}
		callIDs[call.ID] = id
	}
	for _, signal := range trimCallSignals(state.CallSignals, 1000) {
		callID, err := r.callID(ctx, tx, tenantID, callIDs, signal.CallID)
		if err != nil {
			return err
		}
		if callID == nil {
			continue
		}
		if err := r.saveCallSignal(ctx, tx, tenantID, callID, signal); err != nil {
			return err
		}
	}
	if err := r.saveMeta(ctx, tx, tenantID, state); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit workflow save: %w", err)
	}
	r.revision++
	return nil
}

func (r *PostgresRepository) loadLedgerEntries(ctx context.Context, tx *sql.Tx, tenantID string) ([]LedgerEntry, error) {
	rows, err := tx.QueryContext(ctx, `
SELECT
  COALESCE(NULLIF(le.app_data ->> 'id', ''), le.id::text),
  c.external_id,
  le.entry_no,
  le.source_type,
  COALESCE((le.app_data ->> 'sourceId'), ''),
  le.debit::int,
  le.credit::int,
  le.currency,
  COALESCE(le.note, ''),
  le.posted_at
FROM ledger_entries le
JOIN customers c ON c.id = le.customer_id
WHERE le.tenant_id = $1
ORDER BY le.posted_at DESC`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("query ledger entries: %w", err)
	}
	defer rows.Close()

	entries := []LedgerEntry{}
	for rows.Next() {
		var entry LedgerEntry
		var postedAt time.Time
		if err := rows.Scan(
			&entry.ID,
			&entry.CustomerID,
			&entry.EntryNo,
			&entry.SourceType,
			&entry.SourceID,
			&entry.Debit,
			&entry.Credit,
			&entry.Currency,
			&entry.Note,
			&postedAt,
		); err != nil {
			return nil, fmt.Errorf("scan ledger entry: %w", err)
		}
		entry.PostedAt = postedAt.UTC().Format(time.RFC3339)
		entries = append(entries, normalizeLedgerEntry(entry))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ledger entries: %w", err)
	}
	return entries, nil
}

func loadAppDataRows[T any](ctx context.Context, tx *sql.Tx, table string, tenantID string, orderBy string, extraWhere string) ([]T, error) {
	query := fmt.Sprintf("SELECT app_data FROM %s WHERE tenant_id = $1 AND %s ORDER BY %s", table, extraWhere, orderBy)
	rows, err := tx.QueryContext(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("query %s app data: %w", table, err)
	}
	defer rows.Close()

	items := []T{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("scan %s app data: %w", table, err)
		}
		var item T
		if !decodeAppData(raw, &item) {
			return nil, fmt.Errorf("invalid %s app data; repair or migrate the record before startup", table)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate %s app data: %w", table, err)
	}
	return items, nil
}

func (r *PostgresRepository) customerID(ctx context.Context, tx *sql.Tx, tenantID string, cache map[string]string, externalID string) (string, error) {
	externalID = tenantdb.FirstNonEmpty(externalID, "cust_abc_export")
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
		return "", fmt.Errorf("ensure workflow customer %s: %w", externalID, err)
	}
	cache[externalID] = customerID
	return customerID, nil
}

func (r *PostgresRepository) quotationID(ctx context.Context, tx *sql.Tx, tenantID string, cache map[string]string, externalID string) (any, error) {
	externalID = strings.TrimSpace(externalID)
	if externalID == "" {
		return nil, nil
	}
	if cached := cache[externalID]; cached != "" {
		return cached, nil
	}
	var id string
	err := tx.QueryRowContext(ctx, "SELECT id::text FROM quotations WHERE tenant_id = $1 AND external_id = $2", tenantID, externalID).Scan(&id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("lookup quotation %s: %w", externalID, err)
	}
	cache[externalID] = id
	return id, nil
}

func (r *PostgresRepository) manufacturingID(ctx context.Context, tx *sql.Tx, tenantID string, cache map[string]string, externalID string) (any, error) {
	externalID = strings.TrimSpace(externalID)
	if externalID == "" {
		return nil, nil
	}
	if cached := cache[externalID]; cached != "" {
		return cached, nil
	}
	var id string
	err := tx.QueryRowContext(ctx, "SELECT id::text FROM manufacturing_orders WHERE tenant_id = $1 AND external_id = $2", tenantID, externalID).Scan(&id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("lookup manufacturing order %s: %w", externalID, err)
	}
	cache[externalID] = id
	return id, nil
}

func (r *PostgresRepository) callID(ctx context.Context, tx *sql.Tx, tenantID string, cache map[string]string, externalID string) (any, error) {
	externalID = strings.TrimSpace(externalID)
	if externalID == "" {
		return nil, nil
	}
	if cached := cache[externalID]; cached != "" {
		return cached, nil
	}
	var id string
	err := tx.QueryRowContext(ctx, "SELECT id::text FROM calls WHERE tenant_id = $1 AND external_id = $2", tenantID, externalID).Scan(&id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("lookup call %s: %w", externalID, err)
	}
	cache[externalID] = id
	return id, nil
}

func (r *PostgresRepository) saveProduct(ctx context.Context, tx *sql.Tx, tenantID string, customerID string, product Product) error {
	appData, err := encodeAppData(product)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO products (
  tenant_id, customer_id, external_id, sku, name, quantity_on_hand,
  reserved_quantity, description, app_data, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb, now())
ON CONFLICT (tenant_id, external_id) DO UPDATE SET
  customer_id = EXCLUDED.customer_id,
  sku = EXCLUDED.sku,
  name = EXCLUDED.name,
  quantity_on_hand = EXCLUDED.quantity_on_hand,
  reserved_quantity = EXCLUDED.reserved_quantity,
  description = EXCLUDED.description,
  app_data = EXCLUDED.app_data,
  updated_at = now(),
  deleted_at = NULL`,
		tenantID,
		customerID,
		requiredExternalID(product.ID, "prod"),
		tenantdb.FirstNonEmpty(product.SKU, product.ID),
		tenantdb.FirstNonEmpty(product.Name, product.SKU, "Product"),
		product.Stock,
		product.Reserved,
		product.Image,
		appData,
	); err != nil {
		return fmt.Errorf("save product %s: %w", product.ID, err)
	}
	return nil
}

func (r *PostgresRepository) saveQuotation(ctx context.Context, tx *sql.Tx, tenantID string, customerID string, quote Quotation) (string, error) {
	appData, err := encodeAppData(quote)
	if err != nil {
		return "", err
	}

	specData, err := json.Marshal(map[string]string{
		"productId":      quote.ProductID,
		"imageName":      quote.ImageName,
		"imageFileId":    quote.ImageFileID,
		"notes":          quote.Notes,
		"steel":          quote.Steel,
		"tang":           quote.Tang,
		"bladeThickness": quote.BladeThickness,
		"handleMaterial": quote.HandleMaterial,
		"sheath":         quote.Sheath,
		"finish":         quote.Finish,
		"adminNote":      quote.AdminNote,
	})
	if err != nil {
		return "", err
	}

	var id string
	if err := tx.QueryRowContext(ctx, `
INSERT INTO quotations (
  tenant_id, customer_id, external_id, quote_no, product_name, quantity, status,
  quoted_total, deposit_required, estimated_ready_at, spec_data, app_data, created_at, updated_at
) VALUES ($1, $2, $3, $3, $4, $5, $6, $7, $8, $9, $10::jsonb, $11::jsonb, COALESCE($12, now()), now())
ON CONFLICT (tenant_id, external_id) DO UPDATE SET
  customer_id = EXCLUDED.customer_id,
  product_name = EXCLUDED.product_name,
  quantity = EXCLUDED.quantity,
  status = EXCLUDED.status,
  quoted_total = EXCLUDED.quoted_total,
  deposit_required = EXCLUDED.deposit_required,
  estimated_ready_at = EXCLUDED.estimated_ready_at,
  spec_data = EXCLUDED.spec_data,
  app_data = EXCLUDED.app_data,
  updated_at = now()
RETURNING id::text`,
		tenantID,
		customerID,
		requiredExternalID(quote.ID, "Q"),
		tenantdb.FirstNonEmpty(quote.ProductName, "Custom product"),
		positiveOrOne(quote.Quantity),
		tenantdb.FirstNonEmpty(quote.Status, "Requested"),
		quote.TotalAmount,
		quote.DepositRequired,
		nullableDate(quote.ExpectedDate),
		specData,
		appData,
		nullableTime(quote.CreatedAt),
	).Scan(&id); err != nil {
		return "", fmt.Errorf("save quotation %s: %w", quote.ID, err)
	}
	return id, nil
}

func (r *PostgresRepository) saveManufacturing(ctx context.Context, tx *sql.Tx, tenantID string, customerID string, quotationID any, order ManufacturingOrder) (string, error) {
	appData, err := encodeAppData(order)
	if err != nil {
		return "", err
	}

	var id string
	if err := tx.QueryRowContext(ctx, `
INSERT INTO manufacturing_orders (
  tenant_id, customer_id, quotation_id, external_id, order_no, status,
  progress_percent, estimated_ready_at, app_data, created_at, updated_at
) VALUES ($1, $2, $3, $4, $4, $5, $6, $7, $8::jsonb, COALESCE($9, now()), now())
ON CONFLICT (tenant_id, external_id) DO UPDATE SET
  customer_id = EXCLUDED.customer_id,
  quotation_id = EXCLUDED.quotation_id,
  status = EXCLUDED.status,
  progress_percent = EXCLUDED.progress_percent,
  estimated_ready_at = EXCLUDED.estimated_ready_at,
  app_data = EXCLUDED.app_data,
  updated_at = now()
RETURNING id::text`,
		tenantID,
		customerID,
		quotationID,
		requiredExternalID(order.ID, "MFG"),
		tenantdb.FirstNonEmpty(order.Status, "Confirmed"),
		order.Progress,
		nullableDate(order.ExpectedDate),
		appData,
		nullableTime(order.CreatedAt),
	).Scan(&id); err != nil {
		return "", fmt.Errorf("save manufacturing order %s: %w", order.ID, err)
	}
	return id, nil
}

func (r *PostgresRepository) saveRateSheet(ctx context.Context, tx *sql.Tx, tenantID string, rate RateSheet) error {
	appData, err := encodeAppData(rate)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO shipping_rate_sheets (
  tenant_id, external_id, courier, service_name, version, currency, is_active, app_data
) VALUES ($1, $2, $3, $4, $2, 'PKR', $5, $6::jsonb)
ON CONFLICT (tenant_id, external_id) DO UPDATE SET
  courier = EXCLUDED.courier,
  service_name = EXCLUDED.service_name,
  currency = EXCLUDED.currency,
  is_active = EXCLUDED.is_active,
  app_data = EXCLUDED.app_data`,
		tenantID,
		requiredExternalID(rate.ID, "rate"),
		tenantdb.FirstNonEmpty(rate.Courier, "Custom"),
		tenantdb.FirstNonEmpty(rate.Service, "Rate service"),
		strings.EqualFold(rate.Status, "Active"),
		appData,
	); err != nil {
		return fmt.Errorf("save rate sheet %s: %w", rate.ID, err)
	}
	return nil
}

func (r *PostgresRepository) saveShipping(ctx context.Context, tx *sql.Tx, tenantID string, customerID string, manufacturingID any, shipping ShippingRequest) error {
	appData, err := encodeAppData(shipping)
	if err != nil {
		return err
	}
	packageData, err := json.Marshal(map[string]string{
		"zone":    shipping.Zone,
		"weight":  shipping.Weight,
		"history": "stored_in_app_data",
	})
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO shipping_requests (
  tenant_id, customer_id, manufacturing_order_id, external_id, request_no,
  request_type, courier, service_name, status, destination_country, package_data,
  estimated_cost, tracking_number, app_data, created_at, updated_at
) VALUES ($1, $2, $3, $4, $4, $5, $6, $7, $8, $9, $10::jsonb, $11, $12, $13::jsonb, COALESCE($14, now()), now())
ON CONFLICT (tenant_id, external_id) DO UPDATE SET
  customer_id = EXCLUDED.customer_id,
  manufacturing_order_id = EXCLUDED.manufacturing_order_id,
  request_type = EXCLUDED.request_type,
  courier = EXCLUDED.courier,
  service_name = EXCLUDED.service_name,
  status = EXCLUDED.status,
  destination_country = EXCLUDED.destination_country,
  package_data = EXCLUDED.package_data,
  estimated_cost = EXCLUDED.estimated_cost,
  tracking_number = EXCLUDED.tracking_number,
  app_data = EXCLUDED.app_data,
  updated_at = now()`,
		tenantID,
		customerID,
		manufacturingID,
		requiredExternalID(shipping.ID, "SHIP"),
		tenantdb.FirstNonEmpty(shipping.Type, "Outside product"),
		tenantdb.FirstNonEmpty(shipping.Courier, "FedEx"),
		tenantdb.FirstNonEmpty(shipping.Service, "Duty paid premium"),
		tenantdb.FirstNonEmpty(shipping.Status, "Pending"),
		tenantdb.FirstNonEmpty(shipping.Destination, "Not set"),
		packageData,
		shipping.QuotedAmount,
		nullableText(shipping.Tracking),
		appData,
		nullableTime(shipping.CreatedAt),
	); err != nil {
		return fmt.Errorf("save shipping %s: %w", shipping.ID, err)
	}
	return nil
}

func (r *PostgresRepository) savePayment(ctx context.Context, tx *sql.Tx, tenantID string, customerID string, payment Payment) error {
	appData, err := encodeAppData(payment)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO payment_confirmations (
  tenant_id, customer_id, external_id, amount, currency, status, app_data, created_at, confirmed_at
) VALUES ($1, $2, $3, $4, 'PKR', $5, $6::jsonb, COALESCE($7, now()), $8)
ON CONFLICT (tenant_id, external_id) DO UPDATE SET
  customer_id = EXCLUDED.customer_id,
  amount = EXCLUDED.amount,
  currency = EXCLUDED.currency,
  status = EXCLUDED.status,
  app_data = EXCLUDED.app_data,
  confirmed_at = EXCLUDED.confirmed_at`,
		tenantID,
		customerID,
		requiredExternalID(payment.ID, "PAY"),
		payment.Amount,
		tenantdb.FirstNonEmpty(payment.Status, "Waiting confirmation"),
		appData,
		nullableTime(payment.CreatedAt),
		nullableTime(payment.ConfirmedAt),
	); err != nil {
		return fmt.Errorf("save payment %s: %w", payment.ID, err)
	}
	return nil
}

func (r *PostgresRepository) saveLedgerEntry(ctx context.Context, tx *sql.Tx, tenantID string, customerID string, entry LedgerEntry) error {
	entry = normalizeLedgerEntry(entry)
	appData, err := json.Marshal(map[string]string{"id": entry.ID, "sourceId": entry.SourceID})
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO ledger_entries (
  tenant_id, customer_id, entry_no, source_type, source_id, debit, credit,
  currency, note, posted_at, app_data
) VALUES ($1, $2, $3, $4, NULL, $5, $6, $7, $8, COALESCE($9, now()), $10::jsonb)
ON CONFLICT (tenant_id, entry_no) DO UPDATE SET
  customer_id = EXCLUDED.customer_id,
  source_type = EXCLUDED.source_type,
  debit = EXCLUDED.debit,
  credit = EXCLUDED.credit,
  currency = EXCLUDED.currency,
  note = EXCLUDED.note,
  posted_at = EXCLUDED.posted_at,
  app_data = EXCLUDED.app_data`,
		tenantID,
		customerID,
		tenantdb.FirstNonEmpty(entry.EntryNo, entry.ID),
		tenantdb.FirstNonEmpty(entry.SourceType, "manual"),
		entry.Debit,
		entry.Credit,
		tenantdb.FirstNonEmpty(entry.Currency, "PKR"),
		nullableText(entry.Note),
		nullableTime(entry.PostedAt),
		appData,
	); err != nil {
		return fmt.Errorf("save ledger entry %s: %w", entry.EntryNo, err)
	}
	return nil
}
func (r *PostgresRepository) saveConversation(ctx context.Context, tx *sql.Tx, tenantID string, customerID string, conversation Conversation) (string, error) {
	appData, err := encodeAppData(conversation)
	if err != nil {
		return "", err
	}
	var id string
	if err := tx.QueryRowContext(ctx, `
INSERT INTO conversations (
  tenant_id, customer_id, external_id, status, last_message_at, app_data
) VALUES ($1, $2, $3, $4, now(), $5::jsonb)
ON CONFLICT (tenant_id, external_id) DO UPDATE SET
  customer_id = EXCLUDED.customer_id,
  status = EXCLUDED.status,
  last_message_at = EXCLUDED.last_message_at,
  app_data = EXCLUDED.app_data
RETURNING id::text`,
		tenantID,
		customerID,
		requiredExternalID(conversation.ID, "CONV"),
		tenantdb.FirstNonEmpty(conversation.Status, "Open"),
		appData,
	).Scan(&id); err != nil {
		return "", fmt.Errorf("save conversation %s: %w", conversation.ID, err)
	}
	return id, nil
}

func (r *PostgresRepository) saveMessages(ctx context.Context, tx *sql.Tx, tenantID string, conversationID string, messages []Message) error {
	for _, message := range messages {
		appData, err := encodeAppData(message)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO messages (
  tenant_id, conversation_id, external_id, sender_type, body, created_at, app_data
) VALUES ($1, $2, $3, $4, $5, COALESCE($6, now()), $7::jsonb)
ON CONFLICT (conversation_id, external_id) DO UPDATE SET
  sender_type = EXCLUDED.sender_type,
  body = EXCLUDED.body,
  created_at = EXCLUDED.created_at,
  app_data = EXCLUDED.app_data`,
			tenantID,
			conversationID,
			requiredExternalID(message.ID, "MSG"),
			tenantdb.FirstNonEmpty(message.Author, "Customer"),
			message.Body,
			nullableTime(message.CreatedAt),
			appData,
		); err != nil {
			return fmt.Errorf("save message %s: %w", message.ID, err)
		}
	}
	return nil
}

func (r *PostgresRepository) saveCall(ctx context.Context, tx *sql.Tx, tenantID string, customerID string, call CallRequest) (string, error) {
	appData, err := encodeAppData(call)
	if err != nil {
		return "", err
	}
	var id string
	if err := tx.QueryRowContext(ctx, `
INSERT INTO calls (
  tenant_id, customer_id, external_id, status, initiated_at, started_at, ended_at,
  updated_at, ring_expires_at, duration_seconds, initiator_user_external_id,
  answered_by_user_external_id, app_data
) VALUES ($1, $2, $3, $4, COALESCE($5, now()), $6, $7, COALESCE($8, now()), $9, $10, $11, $12, $13::jsonb)
ON CONFLICT (tenant_id, external_id) DO UPDATE SET
  customer_id = EXCLUDED.customer_id,
  status = EXCLUDED.status,
  initiated_at = EXCLUDED.initiated_at,
  started_at = EXCLUDED.started_at,
  ended_at = EXCLUDED.ended_at,
  updated_at = EXCLUDED.updated_at,
  ring_expires_at = EXCLUDED.ring_expires_at,
  duration_seconds = EXCLUDED.duration_seconds,
  initiator_user_external_id = EXCLUDED.initiator_user_external_id,
  answered_by_user_external_id = EXCLUDED.answered_by_user_external_id,
  app_data = EXCLUDED.app_data
RETURNING id::text`,
		tenantID,
		customerID,
		requiredExternalID(call.ID, "CALL"),
		tenantdb.FirstNonEmpty(call.Status, "Missed"),
		nullableTime(call.CreatedAt),
		nullableTime(call.StartedAt),
		nullableTime(call.EndedAt),
		nullableTime(call.UpdatedAt),
		nullableTime(call.RingExpiresAt),
		call.DurationSeconds,
		call.InitiatorUserID,
		call.AnsweredByUserID,
		appData,
	).Scan(&id); err != nil {
		return "", fmt.Errorf("save call %s: %w", call.ID, err)
	}
	return id, nil
}

func (r *PostgresRepository) saveCallSignal(ctx context.Context, tx *sql.Tx, tenantID string, callID any, signal CallSignal) error {
	appData, err := encodeAppData(signal)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(signal.Payload)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO call_signals (
  tenant_id, call_id, external_id, signal_no, sender_user_external_id,
  sender_role, signal_type, payload, app_data, created_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9::jsonb, COALESCE($10, now()))
ON CONFLICT (call_id, external_id) DO UPDATE SET
  signal_no = EXCLUDED.signal_no,
  sender_user_external_id = EXCLUDED.sender_user_external_id,
  sender_role = EXCLUDED.sender_role,
  signal_type = EXCLUDED.signal_type,
  payload = EXCLUDED.payload,
  app_data = EXCLUDED.app_data,
  created_at = EXCLUDED.created_at`,
		tenantID,
		callID,
		requiredExternalID(signal.ID, "SIG"),
		signal.SignalNo,
		signal.SenderID,
		tenantdb.FirstNonEmpty(signal.SenderRole, "Support"),
		tenantdb.FirstNonEmpty(signal.SignalType, "media-state"),
		payload,
		appData,
		nullableTime(signal.CreatedAt),
	); err != nil {
		return fmt.Errorf("save call signal %s: %w", signal.ID, err)
	}
	return nil
}

func (r *PostgresRepository) saveMeta(ctx context.Context, tx *sql.Tx, tenantID string, state State) error {
	settingsData, err := encodeAppData(state.Settings)
	if err != nil {
		return fmt.Errorf("encode platform settings: %w", err)
	}
	noticesData, err := encodeAppData(state.Notices)
	if err != nil {
		return fmt.Errorf("encode customer notices: %w", err)
	}
	featuredData, err := encodeAppData(state.Featured)
	if err != nil {
		return fmt.Errorf("encode featured products: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO workflow_state_meta (tenant_id, last_sequence_day, sequence, platform_settings, customer_notices, featured_products, revision, saved_at)
VALUES ($1, $2, $3, $4::jsonb, $5::jsonb, $6::jsonb, $7, now())
ON CONFLICT (tenant_id) DO UPDATE SET
  last_sequence_day = EXCLUDED.last_sequence_day,
  sequence = EXCLUDED.sequence,
  platform_settings = EXCLUDED.platform_settings,
  customer_notices = EXCLUDED.customer_notices,
  featured_products = EXCLUDED.featured_products,
  revision = EXCLUDED.revision,
  saved_at = now()`,
		tenantID,
		state.LastSequenceDay,
		state.Sequence,
		settingsData,
		noticesData,
		featuredData,
		r.revision+1,
	); err != nil {
		return fmt.Errorf("save workflow meta: %w", err)
	}
	return nil
}

func encodeAppData(value any) ([]byte, error) {
	return json.Marshal(value)
}

func decodeAppData(raw []byte, target any) bool {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "{}" || trimmed == "null" {
		return false
	}
	return json.Unmarshal(raw, target) == nil
}

func workflowIsEmpty(state State) bool {
	return len(state.Products) == 0 &&
		len(state.Quotations) == 0 &&
		len(state.Manufacturing) == 0 &&
		len(state.RateSheets) == 0 &&
		len(state.Shipping) == 0 &&
		len(state.Payments) == 0 &&
		len(state.Ledger) == 0 &&
		len(state.Conversations) == 0 &&
		len(state.Calls) == 0 &&
		len(state.CallSignals) == 0
}

func requiredExternalID(value string, fallbackPrefix string) string {
	value = strings.TrimSpace(value)
	if value != "" {
		return value
	}
	return idgen.New(fallbackPrefix)
}

func nullableText(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
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

func nullableDate(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil
	}
	return parsed
}

func positiveOrOne(value int) int {
	if value < 1 {
		return 1
	}
	return value
}
