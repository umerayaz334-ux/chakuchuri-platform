package auth

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"chakuchuri/backend/internal/platform/idgen"
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

func (r *PostgresRepository) LoadUsers() ([]UserRecord, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, "", fmt.Errorf("begin auth load: %w", err)
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
SELECT
  u.external_id,
  t.external_id,
  COALESCE(c.external_id, ''),
  u.name,
  u.email::text,
  u.role,
  u.status,
  u.permissions,
  u.page_access,
  u.assigned_customer_ids,
  COALESCE(u.salt, ''),
  u.password_hash,
  u.last_online_at,
  u.last_login_at,
  u.failed_login_count,
  u.login_activity,
  u.profile_image_file_id,
  u.suspension_notice,
  u.suspension_contact_at,
  u.suspension_contact_message,
  COALESCE(u.notification_read_ids, '[]'::jsonb)
FROM users u
JOIN tenants t ON t.id = u.tenant_id
LEFT JOIN customer_users cu ON cu.user_id = u.id
LEFT JOIN customers c ON c.id = cu.customer_id
WHERE u.status <> 'deleted' AND u.tenant_id = $1
ORDER BY u.created_at`, tenantID)
	if err != nil {
		return nil, "", fmt.Errorf("query users: %w", err)
	}
	defer rows.Close()

	records := make([]UserRecord, 0)
	for rows.Next() {
		var row UserRecord
		var permissions []byte
		var pageAccess []byte
		var assignedCustomers []byte
		var loginActivity []byte
		var notificationReads []byte
		var lastOnline sql.NullTime
		var lastLogin sql.NullTime
		if err := rows.Scan(
			&row.User.ID,
			&row.User.TenantID,
			&row.User.CustomerID,
			&row.User.Name,
			&row.User.Email,
			&row.User.Role,
			&row.User.Status,
			&permissions,
			&pageAccess,
			&assignedCustomers,
			&row.Salt,
			&row.PasswordHash,
			&lastOnline,
			&lastLogin,
			&row.User.FailedLoginCount,
			&loginActivity,
			&row.User.ProfileImageFileID,
			&row.User.SuspensionNotice,
			&row.User.SuspensionContactAt,
			&row.User.SuspensionMessage,
			&notificationReads,
		); err != nil {
			return nil, "", fmt.Errorf("scan user: %w", err)
		}
		row.User.Email = strings.ToLower(strings.TrimSpace(row.User.Email))
		row.User.Permissions = decodePermissions(permissions, row.User.Role)
		row.User.PageAccess = decodeStrings(pageAccess, defaultPageAccessForRole(row.User.Role))
		row.User.AssignedCustomerIDs = decodeStrings(assignedCustomers, nil)
		row.User.LoginActivity, row.User.KnownIPs = decodeLoginHistory(loginActivity)
		if err := json.Unmarshal(notificationReads, &row.User.NotificationReadIDs); err != nil {
			return nil, "", fmt.Errorf("decode notification reads: %w", err)
		}
		row.User.NotificationReadIDs = sanitizeNotificationReads(row.User.NotificationReadIDs)
		row.User.LastOnline = "Never"
		if lastOnline.Valid {
			row.User.LastOnline = lastOnline.Time.UTC().Format(time.RFC3339)
		}
		row.User.LastLogin = row.User.LastOnline
		if lastLogin.Valid {
			row.User.LastLogin = lastLogin.Time.UTC().Format(time.RFC3339)
		}
		row.User = normalizeLoadedUser(row.User)
		records = append(records, row)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("iterate users: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, "", fmt.Errorf("commit auth load: %w", err)
	}
	if len(records) == 0 {
		return nil, "", errInvalidAuthSnapshot
	}
	return records, "postgres.auth_users", nil
}

func (r *PostgresRepository) SaveUsers(users []UserRecord) error {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin auth save: %w", err)
	}
	defer tx.Rollback()

	tenantIDs := map[string]string{}
	for _, row := range users {
		row.User = normalizeLoadedUser(row.User)
		email := strings.ToLower(strings.TrimSpace(row.User.Email))
		if email == "" {
			continue
		}

		tenantExternalID := tenantdb.FirstNonEmpty(row.User.TenantID, defaultTenantExternalID)
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

		userExternalID := tenantdb.FirstNonEmpty(row.User.ID, idgen.New("usr"))
		permissions, err := encodePermissions(row.User.Permissions)
		if err != nil {
			return err
		}
		pageAccess, err := encodeStrings(row.User.PageAccess)
		if err != nil {
			return err
		}
		assignedCustomers, err := encodeStrings(row.User.AssignedCustomerIDs)
		if err != nil {
			return err
		}
		loginActivity, err := encodeLoginHistory(row.User.LoginActivity, row.User.KnownIPs)
		if err != nil {
			return err
		}
		notificationReads, err := json.Marshal(sanitizeNotificationReads(row.User.NotificationReadIDs))
		if err != nil {
			return err
		}
		lastOnlineAt := nullableLastOnline(row.User.LastOnline)
		lastLoginAt := nullableLastOnline(row.User.LastLogin)

		var userID string
		if err := tx.QueryRowContext(ctx, `
INSERT INTO users (
  tenant_id, external_id, email, name, password_hash, salt, role, status,
  permissions, page_access, assigned_customer_ids, last_online_at, last_login_at,
  failed_login_count, login_activity, profile_image_file_id, suspension_notice,
  suspension_contact_at, suspension_contact_message, notification_read_ids, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb, $10::jsonb, $11::jsonb, $12, $13, $14, $15::jsonb, $16, $17, $18, $19, $20::jsonb, now())
ON CONFLICT (tenant_id, external_id) DO UPDATE SET
  email = EXCLUDED.email,
  name = EXCLUDED.name,
  password_hash = EXCLUDED.password_hash,
  salt = EXCLUDED.salt,
  role = EXCLUDED.role,
  status = EXCLUDED.status,
  permissions = EXCLUDED.permissions,
  page_access = EXCLUDED.page_access,
  assigned_customer_ids = EXCLUDED.assigned_customer_ids,
  last_online_at = EXCLUDED.last_online_at,
  last_login_at = EXCLUDED.last_login_at,
  failed_login_count = EXCLUDED.failed_login_count,
  login_activity = EXCLUDED.login_activity,
  profile_image_file_id = EXCLUDED.profile_image_file_id,
  suspension_notice = EXCLUDED.suspension_notice,
  suspension_contact_at = EXCLUDED.suspension_contact_at,
  suspension_contact_message = EXCLUDED.suspension_contact_message,
  notification_read_ids = EXCLUDED.notification_read_ids,
  updated_at = now()
RETURNING id::text`,
			tenantID,
			userExternalID,
			email,
			row.User.Name,
			row.PasswordHash,
			row.Salt,
			tenantdb.FirstNonEmpty(row.User.Role, "Customer"),
			normalizeStatus(row.User.Status),
			permissions,
			pageAccess,
			assignedCustomers,
			lastOnlineAt,
			lastLoginAt,
			row.User.FailedLoginCount,
			loginActivity,
			row.User.ProfileImageFileID,
			row.User.SuspensionNotice,
			row.User.SuspensionContactAt,
			row.User.SuspensionMessage,
			notificationReads,
		).Scan(&userID); err != nil {
			return fmt.Errorf("upsert user %s: %w", email, err)
		}

		if _, err := tx.ExecContext(ctx, "DELETE FROM customer_users WHERE user_id = $1", userID); err != nil {
			return fmt.Errorf("clear customer link for %s: %w", email, err)
		}
		customerExternalID := strings.TrimSpace(row.User.CustomerID)
		if customerExternalID != "" {
			customerID, err := r.ensureCustomer(ctx, tx, tenantID, customerExternalID)
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
INSERT INTO customer_users (customer_id, user_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING`, customerID, userID); err != nil {
				return fmt.Errorf("link customer user %s: %w", email, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit auth save: %w", err)
	}
	return nil
}

func (r *PostgresRepository) DeleteUser(user User) error {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin auth delete: %w", err)
	}
	defer tx.Rollback()

	tenantID, err := tenantdb.EnsureTenant(ctx, tx, tenantdb.FirstNonEmpty(user.TenantID, defaultTenantExternalID), defaultTenantExternalID)
	if err != nil {
		return err
	}
	if err := tenantdb.SetTenant(ctx, tx, tenantID); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, "DELETE FROM users WHERE tenant_id = $1 AND external_id = $2", tenantID, user.ID)
	if err != nil {
		return fmt.Errorf("delete user %s: %w", user.ID, err)
	}
	count, err := result.RowsAffected()
	if err == nil && count == 0 {
		return ErrUserNotFound
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit auth delete: %w", err)
	}
	return nil
}

func (r *PostgresRepository) ensureCustomer(ctx context.Context, tx *sql.Tx, tenantID string, externalID string) (string, error) {
	name := strings.ReplaceAll(strings.TrimPrefix(strings.TrimSpace(externalID), "cust_"), "_", " ")
	if name == "" {
		name = "Customer"
	}

	var customerID string
	if err := tx.QueryRowContext(ctx, `
INSERT INTO customers (
  tenant_id, external_id, company_name, status, service_flags, portal_data, updated_at
) VALUES ($1, $2, $3, 'active', '{}', '{}', now())
ON CONFLICT (tenant_id, external_id) DO UPDATE SET
  updated_at = now(),
  deleted_at = NULL
RETURNING id::text`, tenantID, externalID, tenantdb.TitleLike(name)).Scan(&customerID); err != nil {
		return "", fmt.Errorf("ensure customer %s: %w", externalID, err)
	}
	return customerID, nil
}

func encodePermissions(values []string) ([]byte, error) {
	return json.Marshal(cleanPermissions(values))
}

func encodeStrings(values []string) ([]byte, error) {
	return json.Marshal(cleanStringSlice(values))
}

func decodeStrings(raw []byte, fallback []string) []string {
	var values []string
	if err := json.Unmarshal(raw, &values); err == nil {
		clean := cleanStringSlice(values)
		if len(clean) > 0 || fallback == nil {
			return clean
		}
	}
	return append([]string(nil), fallback...)
}

func encodeLoginActivity(values []LoginActivity) ([]byte, error) {
	return json.Marshal(trimLoginActivity(values))
}

func decodeLoginActivity(raw []byte) []LoginActivity {
	var values []LoginActivity
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil
	}
	return trimLoginActivity(values)
}

type loginHistoryPayload struct {
	Events   []LoginActivity `json:"events"`
	KnownIPs []KnownIP       `json:"knownIps,omitempty"`
}

func encodeLoginHistory(activity []LoginActivity, known []KnownIP) ([]byte, error) {
	return json.Marshal(loginHistoryPayload{
		Events:   trimLoginActivity(activity),
		KnownIPs: trimKnownIPs(known),
	})
}

func decodeLoginHistory(raw []byte) ([]LoginActivity, []KnownIP) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return []LoginActivity{}, []KnownIP{}
	}
	if raw[0] == '[' {
		activity := decodeLoginActivity(raw)
		return activity, rebuildKnownIPsFromActivity(activity, nil)
	}
	var payload loginHistoryPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		activity := decodeLoginActivity(raw)
		return activity, rebuildKnownIPsFromActivity(activity, nil)
	}
	activity := trimLoginActivity(payload.Events)
	known := trimKnownIPs(payload.KnownIPs)
	if len(known) == 0 && len(activity) > 0 {
		known = rebuildKnownIPsFromActivity(activity, nil)
	}
	return activity, known
}

func decodePermissions(raw []byte, role string) []string {
	var permissions []string
	if err := json.Unmarshal(raw, &permissions); err == nil {
		return permissionsOrDefault(permissions, role)
	}

	flags := map[string]bool{}
	if err := json.Unmarshal(raw, &flags); err == nil {
		values := make([]string, 0, len(flags))
		for permission, enabled := range flags {
			if enabled {
				values = append(values, permission)
			}
		}
		return permissionsOrDefault(values, role)
	}
	return defaultPermissionsForRole(role)
}

func permissionsOrDefault(values []string, role string) []string {
	permissions := cleanPermissionsForRole(values, role)
	if len(permissions) == 0 {
		return defaultPermissionsForRole(role)
	}
	return permissions
}

func cleanPermissions(values []string) []string {
	seen := map[string]bool{}
	permissions := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "calls.start" {
			value = "calls.start"
		}
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		permissions = append(permissions, value)
	}
	sort.Strings(permissions)
	return permissions
}

func defaultPermissionsForRole(role string) []string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "owner", "admin":
		return []string{
			"audit.read",
			"backups.manage",
			"calls.manage",
			"chat.manage",
			"customers.manage",
			"directory.manage",
			"email.manage",
			"email.read",
			"kyc.review",
			"manufacturing.manage",
			"payments.manage",
			"platform.settings.manage",
			"products.manage",
			"quotes.manage",
			"shipping.manage",
			"users.manage",
		}
	case "manager":
		return []string{"calls.manage", "chat.manage", "customers.manage", "directory.manage", "manufacturing.manage", "payments.manage", "products.manage", "quotes.manage", "shipping.manage"}
	case "accountant":
		return []string{"audit.read", "payments.manage"}
	case "shipping staff":
		return []string{"chat.manage", "shipping.manage"}
	case "production staff":
		return []string{"chat.manage", "manufacturing.manage"}
	case "support agent":
		return []string{"calls.manage", "chat.manage", "customers.manage", "directory.manage"}
	default:
		return []string{
			"calls.start",
			"chat.use",
			"orders.read",
			"payments.create",
			"products.manage",
			"quotes.create",
			"shipping.create",
		}
	}
}

func nullableLastOnline(value string) any {
	value = strings.TrimSpace(value)
	if value == "" || strings.EqualFold(value, "Never") {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil
	}
	return parsed.UTC()
}
