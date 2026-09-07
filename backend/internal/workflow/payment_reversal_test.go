package workflow

import (
	"testing"

	"chakuchuri/backend/internal/auth"
)

func TestReverseConfirmedPaymentPostsCompensatingEntryAndRebuildsBalances(t *testing.T) {
	service := NewService(nil, nil)
	admin := auth.User{ID: "usr_admin", Role: "Owner"}
	customer := auth.User{ID: "usr_customer", Role: "Customer", CustomerID: "cust_reverse"}
	order := insertOpenOrder(t, service, customer.CustomerID, "MFG-REV-1", 100000, 0)
	payment := PaymentRequest{CustomerID: customer.CustomerID, ManufacturingID: order.ID, Amount: 40000, Type: "Order payment", ProofName: "proof.png"}
	created, err := service.CreatePayment(payment, customer)
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}
	if _, err := service.ConfirmPayment(created.ID, admin); err != nil {
		t.Fatalf("confirm payment: %v", err)
	}
	result, err := service.ReversePayment(created.ID, PaymentReversalRequest{Reason: "Duplicate bank transfer", Confirmation: created.ID}, admin)
	if err != nil {
		t.Fatalf("reverse payment: %v", err)
	}
	if result.Payment.Status != "Reversed" || result.Payment.ReversalReason != "Duplicate bank transfer" {
		t.Fatalf("unexpected reversal: %#v", result.Payment)
	}
	workspace := service.DashboardForUser(customer)
	refreshed := mustFindOrder(t, workspace.Manufacturing, order.ID)
	if refreshed.PaidAmount != 0 || refreshed.BalanceDue != 100000 {
		t.Fatalf("order was not debited after reversal: paid=%d due=%d", refreshed.PaidAmount, refreshed.BalanceDue)
	}
	if !hasLedgerEntry(workspace.Ledger, "payment_reversal", created.ID, 40000, 0) {
		t.Fatalf("payment reversal ledger entry missing: %#v", workspace.Ledger)
	}
	if ledgerBalance(workspace.Ledger) != 100000 {
		t.Fatalf("ledger balance=%d, want 100000", ledgerBalance(workspace.Ledger))
	}
}

func TestReversePaymentRequiresTypedConfirmationAndIsIdempotent(t *testing.T) {
	service := NewService(nil, nil)
	admin := auth.User{ID: "usr_admin", Role: "Owner"}
	customer := auth.User{ID: "usr_customer", Role: "Customer", CustomerID: "cust_reverse_id"}
	created, err := service.CreatePayment(PaymentRequest{CustomerID: customer.CustomerID, Amount: 1000, ProofName: "proof.png"}, customer)
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}
	if _, err := service.ConfirmPayment(created.ID, admin); err != nil {
		t.Fatalf("confirm payment: %v", err)
	}
	if _, err := service.ReversePayment(created.ID, PaymentReversalRequest{Reason: "Correction", Confirmation: "wrong"}, admin); err != errConfirmationRequired {
		t.Fatalf("wrong confirmation error=%v", err)
	}
	if _, err := service.ReversePayment(created.ID, PaymentReversalRequest{Reason: "Correction", Confirmation: created.ID}, admin); err != nil {
		t.Fatalf("reverse payment: %v", err)
	}
	if _, err := service.ReversePayment(created.ID, PaymentReversalRequest{Reason: "Correction", Confirmation: created.ID}, admin); err != nil {
		t.Fatalf("idempotent reverse: %v", err)
	}
	workspace := service.DashboardForUser(customer)
	count := 0
	for _, entry := range workspace.Ledger {
		if entry.SourceType == "payment_reversal" && entry.SourceID == created.ID {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("reversal ledger entries=%d, want 1", count)
	}
}
