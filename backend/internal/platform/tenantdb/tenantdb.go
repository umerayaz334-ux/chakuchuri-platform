package tenantdb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

func EnsureTenant(ctx context.Context, tx *sql.Tx, externalID string, fallback string) (string, error) {
	externalID = FirstNonEmpty(externalID, fallback, "tenant_chakuchuri")
	slug := SlugFromExternalID(externalID)
	name := strings.ReplaceAll(strings.TrimPrefix(externalID, "tenant_"), "_", " ")

	var tenantID string
	if err := tx.QueryRowContext(ctx, `
INSERT INTO tenants (external_id, slug, name, status, updated_at)
VALUES ($1, $2, $3, 'active', now())
ON CONFLICT (external_id) DO UPDATE SET
  updated_at = now()
RETURNING id::text`, externalID, slug, TitleLike(name)).Scan(&tenantID); err != nil {
		return "", fmt.Errorf("ensure tenant %s: %w", externalID, err)
	}
	return tenantID, nil
}

func SetTenant(ctx context.Context, tx *sql.Tx, tenantID string) error {
	if _, err := tx.ExecContext(ctx, "SELECT set_config('app.current_tenant_id', $1, true)", tenantID); err != nil {
		return fmt.Errorf("set tenant context: %w", err)
	}
	return nil
}

func FirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func SlugFromExternalID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	builder := strings.Builder{}
	lastDash := false
	for _, char := range value {
		if char >= 'a' && char <= 'z' || char >= '0' && char <= '9' {
			builder.WriteRune(char)
			lastDash = false
			continue
		}
		if !lastDash {
			builder.WriteRune('-')
			lastDash = true
		}
	}
	slug := strings.Trim(builder.String(), "-")
	if slug == "" {
		return "tenant"
	}
	return slug
}

func TitleLike(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "ChakuChuri"
	}
	return strings.ToUpper(value[:1]) + value[1:]
}
