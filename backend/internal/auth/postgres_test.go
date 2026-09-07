package auth

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestPermissionsRoundTrip(t *testing.T) {
	raw, err := encodePermissions([]string{"quotes.create", "orders.read", "quotes.create"})
	if err != nil {
		t.Fatalf("encode permissions: %v", err)
	}
	if !json.Valid(raw) {
		t.Fatal("permissions are not valid json")
	}

	got := decodePermissions(raw, "Customer")
	want := []string{"orders.read", "quotes.create"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("permissions mismatch: got %#v want %#v", got, want)
	}
}

func TestDefaultPermissionsForRole(t *testing.T) {
	owner := defaultPermissionsForRole("Owner")
	if len(owner) == 0 || owner[0] != "audit.read" {
		t.Fatalf("unexpected owner permissions: %#v", owner)
	}

	customer := defaultPermissionsForRole("Customer")
	if len(customer) == 0 || customer[0] != "calls.start" {
		t.Fatalf("unexpected customer permissions: %#v", customer)
	}
}
