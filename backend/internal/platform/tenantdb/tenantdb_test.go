package tenantdb

import "testing"

func TestSlugFromExternalID(t *testing.T) {
	got := SlugFromExternalID("tenant_ChakuChuri Main")
	if got != "tenant-chakuchuri-main" {
		t.Fatalf("unexpected slug: %s", got)
	}
}

func TestFirstNonEmpty(t *testing.T) {
	got := FirstNonEmpty(" ", "", "tenant_chakuchuri")
	if got != "tenant_chakuchuri" {
		t.Fatalf("unexpected value: %s", got)
	}
}
