package workflow

import (
	"errors"
	"testing"
	"time"

	"chakuchuri/backend/internal/auth"
)

func TestAccountingQuoteOrderPaymentLedgerInvariant(t *testing.T) {
	service := NewService(nil, nil)
	admin := auth.User{ID: "usr_admin", Role: "Owner"}
	customer := auth.User{ID: "usr_customer", Role: "Customer", CustomerID: "cust_acct_rules"}

	order := insertOpenOrder(t, service, customer.CustomerID, "MFG-ACCT-1", 100000, 30000)
	pay := insertConfirmedPayment(t, service, admin, customer.CustomerID, "PAY-ACCT-1", "", 40000, "2026-09-01T12:00:00Z")

	workspace := service.DashboardForUser(customer)
	healed := mustFindOrder(t, workspace.Manufacturing, order.ID)
	if healed.PaidAmount != 40000 || healed.BalanceDue != 60000 {
		t.Fatalf("order balances paid=%d due=%d", healed.PaidAmount, healed.BalanceDue)
	}
	if healed.PaidAmount+healed.BalanceDue != healed.TotalAmount {
		t.Fatalf("paid+due invariant broken")
	}
	healedPay := mustFindPayment(t, workspace.Payments, pay.ID)
	if paymentAllocationSum(healedPay)+healedPay.CreditLeft != healedPay.Amount {
		t.Fatalf("allocation invariant broken: %#v", healedPay)
	}
	if ledgerBalance(workspace.Ledger) != 100000-40000 {
		t.Fatalf("ledger due = %d, want 60000", ledgerBalance(workspace.Ledger))
	}
}

func TestCompletedOrderPaymentDoesNotLeakOntoNewOrder(t *testing.T) {
	service := NewService(nil, nil)
	admin := auth.User{ID: "usr_admin", Role: "Owner"}
	customer := auth.User{ID: "usr_customer", Role: "Customer", CustomerID: "cust_acct_complete"}

	first := insertOpenOrder(t, service, customer.CustomerID, "MFG-DONE-1", 50000, 0)
	pay := insertConfirmedPayment(t, service, admin, customer.CustomerID, "PAY-DONE-1", first.ID, 50000, "2026-09-01T10:00:00Z")
	_ = service.DashboardForUser(customer)

	service.mu.Lock()
	for index := range service.manufacturing {
		if service.manufacturing[index].ID == first.ID {
			service.manufacturing[index].Status = "Completed"
			service.manufacturing[index].CurrentStage = "Completed"
			service.manufacturing[index].Progress = 100
			service.manufacturing[index].BalanceDue = 0
			service.manufacturing[index].PaidAmount = 50000
			break
		}
	}
	// Keep allocation frozen on completed order.
	for index := range service.payments {
		if service.payments[index].ID == pay.ID {
			service.payments[index].Allocations = []PaymentAllocation{{ManufacturingID: first.ID, Amount: 50000}}
			service.payments[index].CreditLeft = 0
			break
		}
	}
	second := ManufacturingOrder{
		ID: "MFG-OPEN-2", CustomerID: customer.CustomerID, ProductName: "Second batch",
		Quantity: 10, Status: "Confirmed", CurrentStage: "Confirmed", Progress: 43,
		TotalAmount: 80000, BalanceDue: 80000, CreatedAt: "2026-09-04T10:00:00Z",
	}
	service.manufacturing = append(service.manufacturing, second)
	service.postLedgerEntryLocked(admin, customer.CustomerID, "manufacturing_order", second.ID, second.TotalAmount, 0, "Manufacturing order total posted.", second.CreatedAt)
	service.mu.Unlock()

	workspace := service.DashboardForUser(customer)
	open := mustFindOrder(t, workspace.Manufacturing, second.ID)
	if open.PaidAmount != 0 || open.BalanceDue != 80000 {
		t.Fatalf("new order should stay unpaid, got paid=%d due=%d", open.PaidAmount, open.BalanceDue)
	}
	frozenPay := mustFindPayment(t, workspace.Payments, pay.ID)
	if len(frozenPay.Allocations) != 1 || frozenPay.Allocations[0].ManufacturingID != first.ID || frozenPay.Allocations[0].Amount != 50000 {
		t.Fatalf("completed allocation changed: %#v", frozenPay.Allocations)
	}
}

func TestCancelReallocatesPaymentOntoOpenOrder(t *testing.T) {
	service := NewService(nil, nil)
	admin := auth.User{ID: "usr_admin", Role: "Owner"}
	customer := auth.User{ID: "usr_customer", Role: "Customer", CustomerID: "cust_acct_cancel"}

	first := insertOpenOrder(t, service, customer.CustomerID, "MFG-CAN-1", 60000, 0)
	second := insertOpenOrder(t, service, customer.CustomerID, "MFG-CAN-2", 40000, 0)
	_ = insertConfirmedPayment(t, service, admin, customer.CustomerID, "PAY-CAN-1", first.ID, 60000, "2026-09-01T10:00:00Z")
	_ = service.DashboardForUser(customer)

	cancelled, err := service.CancelOrder(first.ID, CancelOrderRequest{Reason: "Customer cancelled", Confirmation: first.ID}, admin)
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if cancelled.Status != "Cancelled" || cancelled.BalanceDue != 0 {
		t.Fatalf("unexpected cancel state: %#v", cancelled)
	}

	workspace := service.DashboardForUser(customer)
	open := mustFindOrder(t, workspace.Manufacturing, second.ID)
	if open.PaidAmount != 40000 || open.BalanceDue != 0 {
		t.Fatalf("cancel should reallocate onto open order, paid=%d due=%d", open.PaidAmount, open.BalanceDue)
	}
	pay := mustFindPayment(t, workspace.Payments, "PAY-CAN-1")
	if pay.CreditLeft != 20000 {
		t.Fatalf("leftover credit = %d, want 20000", pay.CreditLeft)
	}
	if manufacturingChargeBalance(workspace.Ledger, first.ID) != 0 {
		t.Fatalf("cancelled manufacturing charge not reversed")
	}
}

func TestEditOrderClampsDepositAndRebuildsBalances(t *testing.T) {
	service := NewService(nil, nil)
	admin := auth.User{ID: "usr_admin", Role: "Owner"}
	customer := auth.User{ID: "usr_customer", Role: "Customer", CustomerID: "cust_acct_edit"}

	order := insertOpenOrder(t, service, customer.CustomerID, "MFG-EDIT-1", 100000, 40000)
	_ = insertConfirmedPayment(t, service, admin, customer.CustomerID, "PAY-EDIT-1", order.ID, 50000, "2026-09-01T10:00:00Z")
	_ = service.DashboardForUser(customer)

	edited, err := service.EditOrder(order.ID, OrderEditRequest{
		ProductName:  "Edited batch",
		Quantity:     10,
		UnitPrice:    3000,
		ExpectedDate: "2026-10-01",
		Reason:       "Customer reduced quantity",
		Confirmation: order.ID,
	}, admin)
	if err != nil {
		t.Fatalf("edit: %v", err)
	}
	if edited.TotalAmount != 30000 {
		t.Fatalf("total = %d, want 30000", edited.TotalAmount)
	}
	if edited.DepositRequired > edited.TotalAmount {
		t.Fatalf("deposit %d exceeds total %d", edited.DepositRequired, edited.TotalAmount)
	}
	if edited.PaidAmount != 30000 || edited.BalanceDue != 0 {
		t.Fatalf("edited balances paid=%d due=%d", edited.PaidAmount, edited.BalanceDue)
	}
	pay := mustFindPayment(t, service.DashboardForUser(customer).Payments, "PAY-EDIT-1")
	if pay.CreditLeft != 20000 {
		t.Fatalf("excess after edit should become credit, got %d", pay.CreditLeft)
	}
}

func TestCannotCompleteOrderWithBalanceDue(t *testing.T) {
	service := NewService(nil, nil)
	admin := auth.User{ID: "usr_admin", Role: "Owner"}
	customer := auth.User{ID: "usr_customer", Role: "Customer", CustomerID: "cust_acct_block"}

	order := insertOpenOrder(t, service, customer.CustomerID, "MFG-BLOCK-1", 50000, 0)
	_, err := service.MoveOrder(order.ID, "Completed", admin)
	var paymentRequired *orderPaymentRequiredError
	if !errors.As(err, &paymentRequired) || paymentRequired.Kind != "balance" || paymentRequired.Amount != 50000 {
		t.Fatalf("complete unpaid error = %#v, want balance payment requirement", err)
	}

	_ = insertConfirmedPayment(t, service, admin, customer.CustomerID, "PAY-BLOCK-1", order.ID, 50000, time.Now().UTC().Format(time.RFC3339))
	_ = service.DashboardForUser(customer)
	if _, err := service.MoveOrder(order.ID, "Completed", admin); err != nil {
		t.Fatalf("complete paid order: %v", err)
	}
}

func TestMergedLedgerUsesOriginalDebitWhenAdjustmentsExist(t *testing.T) {
	entries := []LedgerEntry{
		{ID: "1", CustomerID: "c1", SourceType: "manufacturing_adjustment", SourceID: "MFG-X-20260901", Debit: 75000, Credit: 0, PostedAt: "2026-09-02T00:00:00Z"},
	}
	orders := []ManufacturingOrder{{ID: "MFG-X", CustomerID: "c1", TotalAmount: 495000, CreatedAt: "2026-09-01T00:00:00Z"}}
	merged := mergedLedgerEntries(entries, orders, nil, nil)
	if !hasLedgerEntry(merged, "manufacturing_order", "MFG-X", 420000, 0) {
		t.Fatalf("expected reconstructed original debit 420000, got %#v", merged)
	}
	if countLedgerSource(merged, "manufacturing_order", "MFG-X") != 1 {
		t.Fatalf("duplicate manufacturing debit")
	}
}

func insertOpenOrder(t *testing.T, service *Service, customerID, id string, total, deposit int) ManufacturingOrder {
	t.Helper()
	order := ManufacturingOrder{
		ID: id, CustomerID: customerID, ProductName: "Test product", Quantity: 10,
		Status: "Confirmed", CurrentStage: "Confirmed", Progress: 43,
		TotalAmount: total, DepositRequired: deposit, PaidAmount: 0, BalanceDue: total,
		CreatedAt: "2026-09-01T09:00:00Z",
	}
	admin := auth.User{ID: "usr_admin", Role: "Owner"}
	service.mu.Lock()
	service.manufacturing = append(service.manufacturing, order)
	service.postLedgerEntryLocked(admin, customerID, "manufacturing_order", order.ID, order.TotalAmount, 0, "Manufacturing order total posted.", order.CreatedAt)
	service.mu.Unlock()
	return order
}

func insertConfirmedPayment(t *testing.T, service *Service, admin auth.User, customerID, id, manufacturingID string, amount int, confirmedAt string) Payment {
	t.Helper()
	payment := Payment{
		ID: id, CustomerID: customerID, ManufacturingID: manufacturingID,
		Type: "Account payment", Amount: amount, Status: "Confirmed",
		ProofName: "proof.png", CreatedAt: confirmedAt, ConfirmedAt: confirmedAt,
	}
	service.mu.Lock()
	service.payments = append(service.payments, payment)
	service.postLedgerEntryLocked(admin, customerID, "payment", payment.ID, 0, amount, "Payment confirmed by company.", confirmedAt)
	service.mu.Unlock()
	return payment
}

func mustFindOrder(t *testing.T, orders []ManufacturingOrder, id string) ManufacturingOrder {
	t.Helper()
	for _, order := range orders {
		if order.ID == id {
			return order
		}
	}
	t.Fatalf("order %s missing", id)
	return ManufacturingOrder{}
}

func mustFindPayment(t *testing.T, payments []Payment, id string) Payment {
	t.Helper()
	for _, payment := range payments {
		if payment.ID == id {
			return payment
		}
	}
	t.Fatalf("payment %s missing", id)
	return Payment{}
}
