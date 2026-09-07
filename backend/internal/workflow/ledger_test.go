package workflow

import (
	"testing"

	"chakuchuri/backend/internal/auth"
)

func TestLedgerPostsOrderShippingAndConfirmedPayment(t *testing.T) {
	service := NewService(nil, nil)
	admin := auth.User{ID: "usr_admin", Role: "Owner", CustomerID: "cust_abc_export"}
	customer := auth.User{ID: "usr_customer", Role: "Customer", CustomerID: "cust_abc_export"}

	quote, err := service.PriceQuote("Q-24094", PriceQuoteRequest{TotalAmount: 1000, DepositRequired: 300}, admin)
	if err != nil {
		t.Fatalf("price quote: %v", err)
	}
	order, err := service.AcceptQuote(quote.ID, customer)
	if err != nil {
		t.Fatalf("accept quote: %v", err)
	}
	shipment, err := service.CreateShipping(ShippingRequestPayload{
		CustomerID:  customer.CustomerID,
		Type:        "Outside product",
		Courier:     "FedEx",
		Service:     "Duty paid premium",
		Destination: "Texas, USA",
		Zone:        "7",
		Weight:      "0-5 kg",
	}, customer)
	if err != nil {
		t.Fatalf("create shipping: %v", err)
	}
	payment, err := service.CreatePayment(PaymentRequest{
		CustomerID:      customer.CustomerID,
		ManufacturingID: order.ID,
		Type:            "Deposit",
		Amount:          300,
		ProofName:       "proof.png",
	}, customer)
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}
	if _, err := service.ConfirmPayment(payment.ID, admin); err != nil {
		t.Fatalf("confirm payment: %v", err)
	}

	workspace := service.DashboardForUser(admin)
	if !hasLedgerEntry(workspace.Ledger, "manufacturing_order", order.ID, 1000, 0) {
		t.Fatalf("manufacturing debit missing: %#v", workspace.Ledger)
	}
	if !hasLedgerEntry(workspace.Ledger, "shipping_request", shipment.ID, shipment.QuotedAmount, 0) {
		t.Fatalf("shipping debit missing: %#v", workspace.Ledger)
	}
	if !hasLedgerEntry(workspace.Ledger, "payment", payment.ID, 0, 300) {
		t.Fatalf("payment credit missing: %#v", workspace.Ledger)
	}
	if workspace.Metrics["ledgerBalance"] == nil {
		t.Fatalf("ledger balance metric missing: %#v", workspace.Metrics)
	}
}

func hasLedgerEntry(entries []LedgerEntry, sourceType string, sourceID string, debit int, credit int) bool {
	for _, entry := range entries {
		if entry.SourceType == sourceType && entry.SourceID == sourceID && entry.Debit == debit && entry.Credit == credit {
			return true
		}
	}
	return false
}
