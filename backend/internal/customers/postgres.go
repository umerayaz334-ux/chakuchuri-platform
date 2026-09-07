package customers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"chakuchuri/backend/internal/platform/tenantdb"
)

const defaultTenantExternalID = "tenant_chakuchuri"

var orderedServices = []string{"Manufacturing", "Shipping", "Chat", "Calls"}

type PostgresRepository struct {
	db      *sql.DB
	timeout time.Duration
}

type portalData struct {
	Online                 bool                `json:"online"`
	LastOnline             string              `json:"lastOnline"`
	BalanceDue             string              `json:"balanceDue"`
	OpenOrders             int                 `json:"openOrders"`
	OpenShipments          int                 `json:"openShipments"`
	VerificationStatus     string              `json:"verificationStatus,omitempty"`
	VerificationStage      string              `json:"verificationStage,omitempty"`
	VerifiedAt             string              `json:"verifiedAt,omitempty"`
	VerifiedByUserID       string              `json:"verifiedByUserId,omitempty"`
	IdentityNote           string              `json:"identityNote,omitempty"`
	CNIC                   string              `json:"cnic,omitempty"`
	Documents              []CustomerDocument  `json:"documents,omitempty"`
	RequestedDocuments     []RequestedDocument `json:"requestedDocuments,omitempty"`
	DocumentsSubmittedAt   string              `json:"documentsSubmittedAt,omitempty"`
	VerificationInviteOpen bool                `json:"verificationInviteOpen,omitempty"`
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db, timeout: 5 * time.Second}
}

func (r *PostgresRepository) LoadCustomers() ([]Customer, []DocumentRequest, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, "", fmt.Errorf("begin customers load: %w", err)
	}
	defer tx.Rollback()

	tenantID, err := tenantdb.EnsureTenant(ctx, tx, defaultTenantExternalID, defaultTenantExternalID)
	if err != nil {
		return nil, nil, "", err
	}
	if err := tenantdb.SetTenant(ctx, tx, tenantID); err != nil {
		return nil, nil, "", err
	}

	rows, err := tx.QueryContext(ctx, `
SELECT
  c.external_id,
  t.external_id,
  c.company_name,
  COALESCE(c.contact_name, ''),
  COALESCE(c.email, ''),
  COALESCE(c.phone, ''),
  COALESCE(c.country, ''),
  c.service_flags,
  c.portal_data
FROM customers c
JOIN tenants t ON t.id = c.tenant_id
WHERE c.deleted_at IS NULL AND c.tenant_id = $1
ORDER BY c.created_at DESC`, tenantID)
	if err != nil {
		return nil, nil, "", fmt.Errorf("query customers: %w", err)
	}
	defer rows.Close()

	customers := make([]Customer, 0)
	for rows.Next() {
		var customer Customer
		var serviceFlags []byte
		var portal []byte
		if err := rows.Scan(
			&customer.ID,
			&customer.TenantID,
			&customer.CompanyName,
			&customer.ContactName,
			&customer.Email,
			&customer.Phone,
			&customer.Country,
			&serviceFlags,
			&portal,
		); err != nil {
			return nil, nil, "", fmt.Errorf("scan customer: %w", err)
		}
		customer.Services = decodeServiceFlags(serviceFlags)
		applyPortalData(&customer, portal)
		customers = append(customers, customer)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, "", fmt.Errorf("iterate customers: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, nil, "", fmt.Errorf("close customers rows: %w", err)
	}

	requestRows, err := tx.QueryContext(ctx, `
SELECT
  r.request_id,
  t.external_id,
  c.external_id,
  r.token_hash,
  r.kinds,
  r.status,
  COALESCE(r.note, ''),
  r.expires_at,
  r.created_at,
  r.created_by,
  r.submitted_at,
  r.revoked_at,
  c.company_name
FROM customer_kyc_requests r
JOIN tenants t ON t.id = r.tenant_id
JOIN customers c ON c.id = r.customer_id
WHERE r.tenant_id = $1
ORDER BY r.created_at DESC`, tenantID)
	if err != nil {
		return nil, nil, "", fmt.Errorf("query KYC requests: %w", err)
	}
	defer requestRows.Close()

	requests := make([]DocumentRequest, 0)
	for requestRows.Next() {
		var request DocumentRequest
		var kinds []byte
		var expiresAt time.Time
		var createdAt time.Time
		var submittedAt sql.NullTime
		var revokedAt sql.NullTime
		if err := requestRows.Scan(
			&request.ID,
			&request.TenantID,
			&request.CustomerID,
			&request.TokenHash,
			&kinds,
			&request.Status,
			&request.Note,
			&expiresAt,
			&createdAt,
			&request.CreatedBy,
			&submittedAt,
			&revokedAt,
			&request.CompanyName,
		); err != nil {
			return nil, nil, "", fmt.Errorf("scan KYC request: %w", err)
		}
		if err := json.Unmarshal(kinds, &request.Kinds); err != nil {
			return nil, nil, "", fmt.Errorf("decode KYC request kinds: %w", err)
		}
		request.ExpiresAt = expiresAt.UTC().Format(time.RFC3339)
		request.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		if submittedAt.Valid {
			request.SubmittedAt = submittedAt.Time.UTC().Format(time.RFC3339)
			request.UsedAt = request.SubmittedAt
		}
		if revokedAt.Valid {
			request.RevokedAt = revokedAt.Time.UTC().Format(time.RFC3339)
		}
		requests = append(requests, request)
	}
	if err := requestRows.Err(); err != nil {
		return nil, nil, "", fmt.Errorf("iterate KYC requests: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, nil, "", fmt.Errorf("commit customers load: %w", err)
	}
	if len(customers) == 0 {
		return nil, nil, "", errInvalidCustomersSnapshot
	}
	return customers, requests, "postgres.customers", nil
}

func (r *PostgresRepository) SaveCustomers(customers []Customer, requests []DocumentRequest) error {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin customers save: %w", err)
	}
	defer tx.Rollback()

	tenantIDs := map[string]string{}
	customerTenants := map[string]string{}
	for _, customer := range customers {
		customerTenants[customer.ID] = tenantdb.FirstNonEmpty(customer.TenantID, defaultTenantExternalID)
		if strings.TrimSpace(customer.ID) == "" {
			continue
		}

		tenantExternalID := tenantdb.FirstNonEmpty(customer.TenantID, defaultTenantExternalID)
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

		serviceFlags, err := encodeServiceFlags(customer.Services)
		if err != nil {
			return err
		}
		portal, err := encodePortalData(customer)
		if err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx, `
INSERT INTO customers (
  tenant_id, external_id, company_name, contact_name, email, phone, country,
  status, service_flags, portal_data, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, 'active', $8::jsonb, $9::jsonb, now())
ON CONFLICT (tenant_id, external_id) DO UPDATE SET
  company_name = EXCLUDED.company_name,
  contact_name = EXCLUDED.contact_name,
  email = EXCLUDED.email,
  phone = EXCLUDED.phone,
  country = EXCLUDED.country,
  status = 'active',
  service_flags = EXCLUDED.service_flags,
  portal_data = EXCLUDED.portal_data,
  updated_at = now(),
  deleted_at = NULL`,
			tenantID,
			customer.ID,
			customer.CompanyName,
			customer.ContactName,
			customer.Email,
			customer.Phone,
			customer.Country,
			serviceFlags,
			portal,
		); err != nil {
			return fmt.Errorf("upsert customer %s: %w", customer.ID, err)
		}
	}

	for _, request := range requests {
		if strings.TrimSpace(request.CustomerID) == "" {
			continue
		}
		tenantExternalID := tenantdb.FirstNonEmpty(request.TenantID, customerTenants[request.CustomerID], defaultTenantExternalID)
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

		var customerID string
		if err := tx.QueryRowContext(ctx, `
SELECT id::text
FROM customers
WHERE tenant_id = $1 AND external_id = $2 AND deleted_at IS NULL`, tenantID, request.CustomerID).Scan(&customerID); err != nil {
			return fmt.Errorf("resolve KYC customer %s: %w", request.CustomerID, err)
		}
		tokenHash := strings.TrimSpace(request.TokenHash)
		if tokenHash == "" && strings.TrimSpace(request.Token) != "" {
			tokenHash = hashRequestToken(request.Token)
		}
		if tokenHash == "" {
			return fmt.Errorf("KYC request %s has no token hash", request.ID)
		}
		requestID := strings.TrimSpace(request.ID)
		if requestID == "" {
			prefix := tokenHash
			if len(prefix) > 16 {
				prefix = prefix[:16]
			}
			requestID = "kyc_req_" + prefix
		}
		status := documentRequestStatus(request)
		if status == "active" {
			if _, err := tx.ExecContext(ctx, `
UPDATE customer_kyc_requests
SET status = CASE WHEN expires_at <= now() THEN 'expired' ELSE 'revoked' END,
    revoked_at = CASE WHEN expires_at > now() THEN COALESCE(revoked_at, now()) ELSE revoked_at END,
    updated_at = now()
WHERE tenant_id = $1 AND customer_id = $2 AND status = 'active' AND request_id <> $3`, tenantID, customerID, requestID); err != nil {
				return fmt.Errorf("revoke prior KYC requests for %s: %w", request.CustomerID, err)
			}
		}
		kinds, err := json.Marshal(request.Kinds)
		if err != nil {
			return fmt.Errorf("encode KYC request kinds: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO customer_kyc_requests (
  tenant_id, request_id, customer_id, token_hash, kinds, status, note,
  created_by, created_at, expires_at, submitted_at, revoked_at, updated_at
) VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7, $8, COALESCE($9, now()), $10, $11, $12, now())
ON CONFLICT (tenant_id, request_id) DO UPDATE SET
  customer_id = EXCLUDED.customer_id,
  token_hash = EXCLUDED.token_hash,
  kinds = EXCLUDED.kinds,
  status = EXCLUDED.status,
  note = EXCLUDED.note,
  expires_at = EXCLUDED.expires_at,
  submitted_at = EXCLUDED.submitted_at,
  revoked_at = EXCLUDED.revoked_at,
  updated_at = now()`,
			tenantID,
			requestID,
			customerID,
			tokenHash,
			kinds,
			status,
			nullableText(request.Note),
			tenantdb.FirstNonEmpty(request.CreatedBy, "system"),
			nullableTime(request.CreatedAt),
			nullableTime(request.ExpiresAt),
			nullableTime(request.SubmittedAt),
			nullableTime(request.RevokedAt),
		); err != nil {
			return fmt.Errorf("save KYC request %s: %w", requestID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit customers save: %w", err)
	}
	return nil
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

func encodeServiceFlags(services []string) ([]byte, error) {
	flags := map[string]bool{}
	for _, service := range services {
		service = strings.TrimSpace(service)
		if service != "" {
			flags[service] = true
		}
	}
	if len(flags) == 0 {
		for _, service := range orderedServices {
			flags[service] = true
		}
	}
	return json.Marshal(flags)
}

func decodeServiceFlags(raw []byte) []string {
	flags := map[string]bool{}
	if err := json.Unmarshal(raw, &flags); err == nil {
		return servicesFromMap(flags)
	}

	var list []string
	if err := json.Unmarshal(raw, &list); err == nil {
		return cleanServices(list)
	}
	return append([]string{}, orderedServices...)
}

func servicesFromMap(flags map[string]bool) []string {
	services := make([]string, 0, len(flags))
	for _, service := range orderedServices {
		if flags[service] {
			services = append(services, service)
			delete(flags, service)
		}
	}

	extra := make([]string, 0, len(flags))
	for service, enabled := range flags {
		if enabled && !strings.HasPrefix(service, "_") {
			extra = append(extra, service)
		}
	}
	sort.Strings(extra)
	services = append(services, extra...)
	if len(services) == 0 {
		return append([]string{}, orderedServices...)
	}
	return services
}

func cleanServices(values []string) []string {
	seen := map[string]bool{}
	services := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		services = append(services, value)
	}
	if len(services) == 0 {
		return append([]string{}, orderedServices...)
	}
	return services
}

func encodePortalData(customer Customer) ([]byte, error) {
	return json.Marshal(portalData{
		Online:                 customer.Online,
		LastOnline:             tenantdb.FirstNonEmpty(customer.LastOnline, "Not tracked yet"),
		BalanceDue:             tenantdb.FirstNonEmpty(customer.BalanceDue, "Rs 0"),
		OpenOrders:             customer.OpenOrders,
		OpenShipments:          customer.OpenShipments,
		VerificationStatus:     firstNonEmpty(normalizeVerificationStatus(customer.VerificationStatus), "unverified"),
		VerificationStage:      customer.VerificationStage,
		VerifiedAt:             customer.VerifiedAt,
		VerifiedByUserID:       customer.VerifiedByUserID,
		IdentityNote:           customer.IdentityNote,
		CNIC:                   customer.CNIC,
		Documents:              customer.Documents,
		RequestedDocuments:     customer.RequestedDocuments,
		DocumentsSubmittedAt:   customer.DocumentsSubmittedAt,
		VerificationInviteOpen: customer.VerificationInviteOpen,
	})
}

func applyPortalData(customer *Customer, raw []byte) {
	data := portalData{
		LastOnline:         "Not tracked yet",
		BalanceDue:         "Rs 0",
		VerificationStatus: "unverified",
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &data)
	}
	customer.Online = data.Online
	customer.LastOnline = tenantdb.FirstNonEmpty(data.LastOnline, "Not tracked yet")
	customer.BalanceDue = tenantdb.FirstNonEmpty(data.BalanceDue, "Rs 0")
	customer.OpenOrders = data.OpenOrders
	customer.OpenShipments = data.OpenShipments
	customer.VerificationStatus = firstNonEmpty(normalizeVerificationStatus(data.VerificationStatus), "unverified")
	customer.VerificationStage = data.VerificationStage
	customer.VerifiedAt = data.VerifiedAt
	customer.VerifiedByUserID = data.VerifiedByUserID
	customer.IdentityNote = data.IdentityNote
	customer.CNIC = data.CNIC
	customer.Documents = data.Documents
	customer.RequestedDocuments = data.RequestedDocuments
	customer.DocumentsSubmittedAt = data.DocumentsSubmittedAt
	customer.VerificationInviteOpen = data.VerificationInviteOpen
	if customer.Documents == nil {
		customer.Documents = []CustomerDocument{}
	}
	if customer.RequestedDocuments == nil {
		customer.RequestedDocuments = []RequestedDocument{}
	}
}
