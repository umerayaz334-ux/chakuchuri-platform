package workflow

import (
	"errors"
	"testing"

	"chakuchuri/backend/internal/auth"
)

func TestPriceQuoteUsesUnitPriceAndKeepsValidDeposit(t *testing.T) {
	service := NewService(nil, nil)
	admin := auth.User{ID: "usr_admin", Name: "Admin", Role: "Owner"}

	quote, err := service.PriceQuote("Q-24094", PriceQuoteRequest{
		ProductName:     "Custom camping axe",
		UnitPrice:       2750,
		TotalAmount:     1,
		DepositRequired: 5000,
		ExpectedDate:    "2026-10-15",
	}, admin)
	if err != nil {
		t.Fatalf("price quote: %v", err)
	}
	if quote.TotalAmount != quote.Quantity*2750 {
		t.Fatalf("total = %d, want %d", quote.TotalAmount, quote.Quantity*2750)
	}
	if quote.DepositRequired != 5000 {
		t.Fatalf("deposit = %d, want 5000", quote.DepositRequired)
	}
}

func TestEditAndCancelOrderKeepImmutableAccountingTrail(t *testing.T) {
	service := NewService(nil, nil)
	admin := auth.User{ID: "usr_admin", Name: "Admin", Role: "Owner"}
	customer := auth.User{ID: "usr_customer", Name: "Customer", Role: "Customer", CustomerID: "cust_abc_export"}

	pending, err := service.CreatePayment(PaymentRequest{
		CustomerID:      customer.CustomerID,
		ManufacturingID: "MFG-24091",
		Type:            "Order payment",
		Amount:          10000,
		ProofName:       "pending-proof.png",
	}, customer)
	if err != nil {
		t.Fatalf("create pending payment: %v", err)
	}

	edited, err := service.EditOrder("MFG-24091", OrderEditRequest{
		ProductName:  "Damascus chef knife revised batch",
		Quantity:     550,
		UnitPrice:    900,
		ExpectedDate: "2026-09-20",
		Reason:       "Customer approved 50 additional pieces.",
		Confirmation: "MFG-24091",
	}, admin)
	if err != nil {
		t.Fatalf("edit order: %v", err)
	}
	if edited.TotalAmount != 495000 || edited.BalanceDue != 369000 {
		t.Fatalf("edited totals = total %d, balance %d", edited.TotalAmount, edited.BalanceDue)
	}

	workspace := service.DashboardForUser(admin)
	if countLedgerSource(workspace.Ledger, "manufacturing_order", edited.ID) != 1 {
		t.Fatalf("original order debit must remain exactly once: %#v", workspace.Ledger)
	}
	if !hasLedgerEntry(workspace.Ledger, "manufacturing_order", edited.ID, 420000, 0) {
		t.Fatalf("original order debit was rewritten: %#v", workspace.Ledger)
	}
	if !hasLedgerAmount(workspace.Ledger, "manufacturing_adjustment", edited.ID+"-", 75000, 0) {
		t.Fatalf("order increase adjustment missing: %#v", workspace.Ledger)
	}

	cancelled, err := service.CancelOrder(edited.ID, CancelOrderRequest{
		Reason:       "Customer cancelled before final production.",
		Confirmation: edited.ID,
	}, admin)
	if err != nil {
		t.Fatalf("cancel order: %v", err)
	}
	if cancelled.Status != "Cancelled" || cancelled.CurrentStage != "Cancelled" || cancelled.BalanceDue != 0 {
		t.Fatalf("unexpected cancelled order: %#v", cancelled)
	}

	workspace = service.DashboardForUser(admin)
	if !hasLedgerEntry(workspace.Ledger, "manufacturing_cancellation", edited.ID, 0, edited.TotalAmount) {
		t.Fatalf("cancellation reversal missing: %#v", workspace.Ledger)
	}
	if manufacturingChargeBalance(workspace.Ledger, edited.ID) != 0 {
		t.Fatalf("manufacturing charge was not fully reversed: %#v", workspace.Ledger)
	}
	for _, payment := range workspace.Payments {
		if payment.ID == pending.ID && payment.Status != "Cancelled" {
			t.Fatalf("pending payment status = %s, want Cancelled", payment.Status)
		}
	}
}

func TestOrderChangeRequiresExactOrderID(t *testing.T) {
	service := NewService(nil, nil)
	admin := auth.User{ID: "usr_admin", Role: "Owner"}

	_, err := service.EditOrder("MFG-24091", OrderEditRequest{
		ProductName:  "Changed product",
		Quantity:     10,
		UnitPrice:    100,
		ExpectedDate: "2026-09-20",
		Reason:       "Test",
		Confirmation: "wrong-id",
	}, admin)
	if !errors.Is(err, errConfirmationRequired) {
		t.Fatalf("edit error = %v, want confirmation error", err)
	}

	_, err = service.CancelOrder("MFG-24091", CancelOrderRequest{Reason: "Test", Confirmation: "wrong-id"}, admin)
	if !errors.Is(err, errConfirmationRequired) {
		t.Fatalf("cancel error = %v, want confirmation error", err)
	}
}

func countLedgerSource(entries []LedgerEntry, sourceType string, sourceID string) int {
	count := 0
	for _, entry := range entries {
		if entry.SourceType == sourceType && entry.SourceID == sourceID {
			count++
		}
	}
	return count
}

func hasLedgerAmount(entries []LedgerEntry, sourceType string, sourceIDPrefix string, debit int, credit int) bool {
	for _, entry := range entries {
		if entry.SourceType == sourceType && len(entry.SourceID) >= len(sourceIDPrefix) && entry.SourceID[:len(sourceIDPrefix)] == sourceIDPrefix && entry.Debit == debit && entry.Credit == credit {
			return true
		}
	}
	return false
}

func manufacturingChargeBalance(entries []LedgerEntry, orderID string) int {
	balance := 0
	for _, entry := range entries {
		isOriginal := entry.SourceType == "manufacturing_order" && entry.SourceID == orderID
		isAdjustment := entry.SourceType == "manufacturing_adjustment" && len(entry.SourceID) >= len(orderID)+1 && entry.SourceID[:len(orderID)+1] == orderID+"-"
		isCancellation := entry.SourceType == "manufacturing_cancellation" && entry.SourceID == orderID
		if isOriginal || isAdjustment || isCancellation {
			balance += entry.Debit - entry.Credit
		}
	}
	return balance
}
