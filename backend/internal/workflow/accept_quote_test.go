package workflow

import (
	"testing"

	"chakuchuri/backend/internal/auth"
)

func TestAcceptQuoteReusesQuotationIDAsOrderID(t *testing.T) {
	service := NewService(nil, nil)
	admin := auth.User{ID: "usr_admin", Role: "Owner", CustomerID: "cust_abc_export"}
	customer := auth.User{ID: "usr_customer", Role: "Customer", CustomerID: "cust_abc_export"}

	quote, err := service.PriceQuote("Q-24094", PriceQuoteRequest{
		TotalAmount:     25000,
		DepositRequired: 5000,
		ProductName:     "Custom camping axe sample",
	}, admin)
	if err != nil {
		t.Fatalf("price quote: %v", err)
	}

	order, err := service.AcceptQuote(quote.ID, customer)
	if err != nil {
		t.Fatalf("accept quote: %v", err)
	}
	if order.ID != quote.ID {
		t.Fatalf("expected order id %q to match quote id, got %q", quote.ID, order.ID)
	}
	if order.QuotationID != quote.ID {
		t.Fatalf("expected quotationId %q, got %q", quote.ID, order.QuotationID)
	}

	again, err := service.AcceptQuote(quote.ID, customer)
	if err != nil {
		t.Fatalf("idempotent accept: %v", err)
	}
	if again.ID != order.ID {
		t.Fatalf("idempotent accept returned different order id: %q vs %q", again.ID, order.ID)
	}

	var matches int
	for _, row := range service.DashboardForUser(admin).Manufacturing {
		if row.QuotationID == quote.ID || row.ID == quote.ID {
			matches++
		}
	}
	if matches != 1 {
		t.Fatalf("expected exactly one manufacturing order for quote, got %d", matches)
	}
}
