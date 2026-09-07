package workflow

import (
	"testing"

	"chakuchuri/backend/internal/auth"
)

func TestConfirmPaymentWaterfallCoversOrdersThenCredit(t *testing.T) {
	service := NewService(nil, nil)
	admin := auth.User{ID: "usr_admin", Role: "Owner"}
	customer := auth.User{ID: "usr_customer", Role: "Customer", CustomerID: "cust_abc_export"}

	quote, err := service.PriceQuote("Q-24094", PriceQuoteRequest{
		ProductName: "Custom camping axe", UnitPrice: 1000, DepositRequired: 500, ExpectedDate: "2026-10-01",
	}, admin)
	if err != nil {
		t.Fatalf("price quote: %v", err)
	}
	orderB, err := service.AcceptQuote(quote.ID, customer)
	if err != nil {
		t.Fatalf("accept quote: %v", err)
	}

	var orderA ManufacturingOrder
	for _, order := range service.DashboardForUser(customer).Manufacturing {
		if order.ID == "MFG-24091" {
			orderA = order
			break
		}
	}
	if orderA.ID == "" || orderA.BalanceDue < 1 {
		t.Fatalf("seed order missing: %#v", orderA)
	}

	// Clear older production balance first (deposit-pending orderB is preferred before older orderA?
	// Deposit pending comes first, so orderB first, then orderA.
	payAmount := orderB.BalanceDue + 15000
	payment, err := service.CreatePayment(PaymentRequest{
		CustomerID: customer.CustomerID,
		Type:       "Account payment",
		Amount:     payAmount,
		ProofName:  "transfer.png",
	}, customer)
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}
	confirmed, err := service.ConfirmPayment(payment.ID, admin)
	if err != nil {
		t.Fatalf("confirm payment: %v", err)
	}
	if confirmed.Payment.CreditLeft != 0 {
		t.Fatalf("credit left = %d, want 0", confirmed.Payment.CreditLeft)
	}
	if len(confirmed.Payment.Allocations) < 2 {
		t.Fatalf("expected allocations across both orders, got %#v", confirmed.Payment.Allocations)
	}

	workspace := service.DashboardForUser(customer)
	var refreshedA, refreshedB ManufacturingOrder
	for _, order := range workspace.Manufacturing {
		if order.ID == orderA.ID {
			refreshedA = order
		}
		if order.ID == orderB.ID {
			refreshedB = order
		}
	}
	if refreshedB.BalanceDue != 0 {
		t.Fatalf("deposit order balance = %d, want 0", refreshedB.BalanceDue)
	}
	if refreshedB.Status == "Deposit pending" {
		t.Fatalf("deposit order should be confirmed after full payment")
	}
	if refreshedA.BalanceDue != orderA.BalanceDue-15000 {
		t.Fatalf("older order balance = %d, want %d", refreshedA.BalanceDue, orderA.BalanceDue-15000)
	}

	// Overpay leftover becomes credit.
	left := refreshedA.BalanceDue
	overpay, err := service.CreatePayment(PaymentRequest{
		CustomerID: customer.CustomerID,
		Amount:     left + 2500,
		ProofName:  "extra.png",
	}, customer)
	if err != nil {
		t.Fatalf("create overpay: %v", err)
	}
	extra, err := service.ConfirmPayment(overpay.ID, admin)
	if err != nil {
		t.Fatalf("confirm overpay: %v", err)
	}
	if extra.Payment.CreditLeft != 2500 {
		t.Fatalf("credit left = %d, want 2500", extra.Payment.CreditLeft)
	}
}

func TestDashboardHealsStaleOrderBalancesFromConfirmedPayments(t *testing.T) {
	service := NewService(nil, nil)
	admin := auth.User{ID: "usr_admin", Role: "Owner"}
	customer := auth.User{ID: "usr_customer", Role: "Customer", CustomerID: "cust_heal_test"}

	now := "2026-09-01T10:00:00Z"
	order := ManufacturingOrder{
		ID: "MFG-HEAL-1", CustomerID: customer.CustomerID, ProductName: "Healing axe",
		Quantity: 25, Status: "Confirmed", CurrentStage: "Confirmed", Progress: 43,
		ExpectedDate: "2026-10-01", TotalAmount: 100000, PaidAmount: 0, BalanceDue: 100000,
		CreatedAt: now,
	}
	payment := Payment{
		ID: "PAY-HEAL-1", CustomerID: customer.CustomerID, Type: "Account payment",
		Amount: 40000, Status: "Confirmed", ProofName: "old-transfer.png",
		CreatedAt: now, ConfirmedAt: "2026-09-03T10:00:00Z",
	}

	service.mu.Lock()
	service.manufacturing = append(service.manufacturing, order)
	service.payments = append(service.payments, payment)
	service.postLedgerEntryLocked(admin, customer.CustomerID, "manufacturing_order", order.ID, order.TotalAmount, 0, "Manufacturing order total posted.", now)
	service.postLedgerEntryLocked(admin, customer.CustomerID, "payment", payment.ID, 0, payment.Amount, "Payment confirmed by company.", payment.ConfirmedAt)
	service.mu.Unlock()

	workspace := service.DashboardForUser(customer)
	var healed ManufacturingOrder
	for _, row := range workspace.Manufacturing {
		if row.ID == order.ID {
			healed = row
			break
		}
	}
	if healed.PaidAmount != 40000 {
		t.Fatalf("healed paid = %d, want 40000", healed.PaidAmount)
	}
	if healed.BalanceDue != 60000 {
		t.Fatalf("healed balance = %d, want 60000", healed.BalanceDue)
	}

	var healedPayment Payment
	for _, row := range workspace.Payments {
		if row.ID == payment.ID {
			healedPayment = row
			break
		}
	}
	if len(healedPayment.Allocations) != 1 || healedPayment.Allocations[0].Amount != 40000 {
		t.Fatalf("expected rebuilt allocation, got %#v", healedPayment.Allocations)
	}
}

func TestDashboardPreservesSeedDepositBalances(t *testing.T) {
	service := NewService(nil, nil)
	customer := auth.User{ID: "usr_customer", Role: "Customer", CustomerID: "cust_abc_export"}
	workspace := service.DashboardForUser(customer)
	var seed ManufacturingOrder
	for _, order := range workspace.Manufacturing {
		if order.ID == "MFG-24091" {
			seed = order
			break
		}
	}
	if seed.ID == "" {
		t.Fatal("seed order missing")
	}
	if seed.PaidAmount != 126000 || seed.BalanceDue != 294000 {
		t.Fatalf("seed balances changed unexpectedly: paid=%d due=%d", seed.PaidAmount, seed.BalanceDue)
	}
}
