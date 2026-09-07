package customers

import (
	"encoding/json"
	"reflect"
	"testing"

	"chakuchuri/backend/internal/platform/tenantdb"
)

func TestServiceFlagsRoundTrip(t *testing.T) {
	raw, err := encodeServiceFlags([]string{"Shipping", "Manufacturing", "Shipping"})
	if err != nil {
		t.Fatalf("encode flags: %v", err)
	}

	got := decodeServiceFlags(raw)
	want := []string{"Manufacturing", "Shipping"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("services mismatch: got %#v want %#v", got, want)
	}
}

func TestPortalDataRoundTrip(t *testing.T) {
	customer := Customer{
		ID:            "cust_abc",
		Online:        true,
		LastOnline:    "Online now",
		BalanceDue:    "Rs 500",
		OpenOrders:    2,
		OpenShipments: 3,
	}

	raw, err := encodePortalData(customer)
	if err != nil {
		t.Fatalf("encode portal: %v", err)
	}
	if !json.Valid(raw) {
		t.Fatal("portal data is not valid json")
	}

	var loaded Customer
	applyPortalData(&loaded, raw)
	if loaded.Online != customer.Online || loaded.LastOnline != customer.LastOnline || loaded.BalanceDue != customer.BalanceDue || loaded.OpenOrders != customer.OpenOrders || loaded.OpenShipments != customer.OpenShipments {
		t.Fatalf("portal data mismatch: got %#v want %#v", loaded, customer)
	}
}

func TestSlugFromExternalID(t *testing.T) {
	got := tenantdb.SlugFromExternalID("tenant_ChakuChuri Main")
	if got != "tenant-chakuchuri-main" {
		t.Fatalf("unexpected slug: %s", got)
	}
}
