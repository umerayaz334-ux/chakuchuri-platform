package workflow

import (
	"encoding/json"
	"testing"
)

func TestWorkflowAppDataRoundTrip(t *testing.T) {
	quote := Quotation{
		ID:              "Q-100",
		CustomerID:      "cust_abc",
		ProductName:     "Custom knife batch",
		Quantity:        24,
		Status:          "Priced",
		TotalAmount:     12000,
		DepositRequired: 5000,
		History:         []Update{{Label: "Quote priced", Detail: "Ready for review", Actor: "Admin", CreatedAt: "2026-09-01T10:00:00Z"}},
	}

	raw, err := encodeAppData(quote)
	if err != nil {
		t.Fatalf("encode app data: %v", err)
	}
	if !json.Valid(raw) {
		t.Fatal("app data is not valid json")
	}

	var loaded Quotation
	if !decodeAppData(raw, &loaded) {
		t.Fatal("app data did not decode")
	}
	if loaded.ID != quote.ID || loaded.History[0].Label != "Quote priced" {
		t.Fatalf("quote mismatch: %#v", loaded)
	}
}

func TestNullableDate(t *testing.T) {
	if nullableDate("") != nil {
		t.Fatal("empty date should be nil")
	}
	if nullableDate("not-a-date") != nil {
		t.Fatal("invalid date should be nil")
	}
	if nullableDate("2026-09-15") == nil {
		t.Fatal("valid date should be stored")
	}
}
