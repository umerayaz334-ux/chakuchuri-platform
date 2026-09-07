package workflow

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"chakuchuri/backend/internal/auth"
)

var errAccountingReview = errors.New("Accounting reconciliation is required before allocating payments for this customer.")
var errPaymentConflict = errors.New("This payment reference was already used with different details.")

const maxMoneyAmount = 2_000_000_000

func paymentSubmissionKey(p PaymentRequest) (string, error) {
	key := strings.TrimSpace(p.IdempotencyKey)
	if len(key) > 128 {
		return "", errValidation
	}
	if key != "" {
		return "request:" + key, nil
	}
	if file := strings.TrimSpace(p.ProofFileID); file != "" {
		return "proof:" + file, nil
	}
	return "", nil
}

type AccountingSummary struct {
	Balance      int `json:"balance"`
	Due          int `json:"due"`
	Credit       int `json:"credit"`
	Charges      int `json:"charges"`
	Adjustments  int `json:"adjustments"`
	Received     int `json:"received"`
	Pending      int `json:"pending"`
	LedgerCount  int `json:"ledgerCount"`
	PaymentCount int `json:"paymentCount"`
}

func accountingSummary(ledger []LedgerEntry, payments []Payment) AccountingSummary {
	summary := AccountingSummary{LedgerCount: len(ledger), PaymentCount: len(payments)}
	balances := map[string]int{}
	for _, row := range ledger {
		balances[row.CustomerID] += row.Debit - row.Credit
		summary.Balance += row.Debit - row.Credit
		switch row.SourceType {
		case "payment":
			summary.Received += row.Credit
		case "payment_reversal":
			summary.Received -= row.Debit
		case "manufacturing_order", "manufacturing_adjustment", "manufacturing_cancellation", "shipping_request", "shipping_adjustment", "shipping_cancellation":
			// Charges are the net posted charge after adjustments and reversals.
			summary.Charges += row.Debit - row.Credit
			if row.SourceType != "manufacturing_order" && row.SourceType != "shipping_request" {
				summary.Adjustments += row.Debit - row.Credit
			}
		}
	}
	for _, balance := range balances {
		summary.Due += max(balance, 0)
		summary.Credit += max(-balance, 0)
	}
	for _, payment := range payments {
		if payment.Status == "Waiting confirmation" {
			summary.Pending += payment.Amount
		}
	}
	return summary
}
func addAccountingMetrics(metrics map[string]interface{}, ledger []LedgerEntry, payments []Payment) {
	a := accountingSummary(ledger, payments)
	metrics["balanceDue"], metrics["ledgerBalance"], metrics["accountCredit"] = a.Due, a.Balance, a.Credit
	metrics["totalCharges"], metrics["totalReceived"], metrics["totalAdjustments"] = a.Charges, a.Received, a.Adjustments
	metrics["pendingPaymentAmount"] = a.Pending
}

type AccountingIssue struct {
	CustomerID string `json:"customerId"`
	RecordID   string `json:"recordId"`
	Detail     string `json:"detail"`
}

func (s *Service) accountingIssuesLocked(customerID string) []AccountingIssue {
	issues := []AccountingIssue{}
	add := func(recordID, detail string) {
		issues = append(issues, AccountingIssue{CustomerID: customerID, RecordID: recordID, Detail: detail})
	}
	orderByID := map[string]ManufacturingOrder{}
	for _, order := range s.manufacturing {
		if order.CustomerID == customerID {
			orderByID[order.ID] = order
		}
	}
	shippingByID := map[string]ShippingRequest{}
	for _, shipment := range s.shipping {
		if shipment.CustomerID == customerID {
			shippingByID[shipment.ID] = shipment
		}
	}
	ledgerCount := map[string]int{}
	ledgerNet := map[string]int{}
	for _, entry := range s.ledger {
		if entry.CustomerID != customerID {
			continue
		}
		key := entry.SourceType + "\x00" + entry.SourceID
		ledgerCount[key]++
		ledgerNet[key] += entry.Debit - entry.Credit
	}
	for key, count := range ledgerCount {
		if count > 1 && !strings.Contains(strings.SplitN(key, "\x00", 2)[0], "_adjustment") {
			parts := strings.SplitN(key, "\x00", 2)
			add(parts[1], "Duplicate ledger entries exist for this source.")
		}
	}
	confirmedAllocations := map[string]int{}
	for _, payment := range s.payments {
		if payment.CustomerID != customerID {
			continue
		}
		key := "payment\x00" + payment.ID
		if payment.Status == "Confirmed" {
			if ledgerCount[key] != 1 || ledgerNet[key] != -payment.Amount {
				add(payment.ID, "Confirmed payment must have exactly one matching ledger credit.")
			}
			allocated := 0
			for _, allocation := range payment.Allocations {
				if allocation.Amount < 1 || (allocation.ManufacturingID == "" && allocation.ShippingID == "") || (allocation.ManufacturingID != "" && allocation.ShippingID != "") {
					add(payment.ID, "Payment allocation has an invalid amount or target.")
					continue
				}
				if allocation.ManufacturingID != "" {
					order, ok := orderByID[allocation.ManufacturingID]
					if !ok || strings.EqualFold(order.Status, "Cancelled") {
						add(payment.ID, "Payment allocation targets a missing or cancelled order.")
						continue
					}
					confirmedAllocations[allocation.ManufacturingID] += allocation.Amount
				} else if shipment, ok := shippingByID[allocation.ShippingID]; !ok || strings.EqualFold(shipment.Status, "Cancelled") {
					add(payment.ID, "Payment allocation targets a missing or cancelled shipment.")
					continue
				}
				allocated += allocation.Amount
			}
			if allocated+payment.CreditLeft != payment.Amount || payment.CreditLeft < 0 {
				add(payment.ID, "Payment allocations plus customer credit must equal the confirmed amount.")
			}
		} else if payment.Status == "Waiting confirmation" && ledgerCount[key] != 0 {
			add(payment.ID, "Pending payment must not affect the ledger before confirmation.")
		} else if payment.Status == "Reversed" {
			if ledgerCount[key] != 1 || ledgerCount["payment_reversal\x00"+payment.ID] != 1 || ledgerNet["payment_reversal\x00"+payment.ID] != payment.Amount {
				add(payment.ID, "Reversed payment must have one original credit and one matching reversal debit.")
			}
		}
	}
	for id, order := range orderByID {
		chargeNet := 0
		for _, entry := range s.ledger {
			if entry.CustomerID == customerID && ((entry.SourceType == "manufacturing_order" && entry.SourceID == id) || (entry.SourceType == "manufacturing_adjustment" && strings.HasPrefix(entry.SourceID, id+"-")) || (entry.SourceType == "manufacturing_cancellation" && entry.SourceID == id)) {
				chargeNet += entry.Debit - entry.Credit
			}
		}
		if strings.EqualFold(order.Status, "Cancelled") {
			if order.PaidAmount != 0 || order.BalanceDue != 0 || chargeNet != 0 {
				add(id, "Cancelled order must have zero balance and a fully reversed charge.")
			}
			continue
		}
		if order.TotalAmount < 0 || order.PaidAmount < 0 || order.BalanceDue < 0 || order.PaidAmount+order.BalanceDue != order.TotalAmount || order.DepositRequired > order.TotalAmount {
			add(id, "Order paid and due fields must reconcile to the order total.")
		}

		if chargeNet != order.TotalAmount {
			add(id, "Manufacturing charge ledger does not equal the current order total.")
		}
		if confirmedAllocations[id] != order.PaidAmount {
			add(id, "Confirmed payment allocations do not equal the order paid amount.")
		}
	}
	for id, shipment := range shippingByID {
		chargeNet := 0
		for _, entry := range s.ledger {
			if entry.CustomerID == customerID && (entry.SourceType == "shipping_request" && entry.SourceID == id || entry.SourceType == "shipping_adjustment" && strings.HasPrefix(entry.SourceID, id+"-")) {
				chargeNet += entry.Debit - entry.Credit
			}
		}
		if strings.EqualFold(shipment.Status, "Cancelled") {
			if chargeNet != 0 {
				add(id, "Cancelled shipment must have a fully reversed charge.")
			}
		} else if chargeNet != shipment.QuotedAmount {
			add(id, "Shipping charge ledger does not equal the current quoted amount.")
		}
	}
	return issues
}
func (s *Service) requireAccountingReadyLocked(customerID string) error {
	// Allocation drift is repairable and is healed by the authoritative
	// resynchronizer. Only a completed order with inconsistent confirmed money
	// requires an explicit owner reconciliation before another mutation.
	paid := map[string]int{}
	for _, payment := range s.payments {
		if payment.CustomerID != customerID || payment.Status != "Confirmed" {
			continue
		}
		for _, allocation := range payment.Allocations {
			paid[allocation.ManufacturingID] += allocation.Amount
		}
	}
	for _, order := range s.manufacturing {
		if order.CustomerID == customerID && order.Status == "Completed" && (order.PaidAmount != order.TotalAmount || order.BalanceDue != 0 || paid[order.ID] != order.PaidAmount) {
			return errAccountingReview
		}
	}
	return nil
}

// Retain valid allocations; distribute only remaining money. Closed-order money
// is never silently reused. Explicit reconciliation also covers legacy deficits.
func (s *Service) allocateCustomerLocked(customerID, historyPaymentID string, actor auth.User, now time.Time, reconcile bool) []ManufacturingOrder {
	orderIndexes := map[string]int{}
	shippingIndexes := map[string]int{}
	for i := range s.manufacturing {
		o := &s.manufacturing[i]
		if o.CustomerID != customerID {
			continue
		}
		if o.Status == "Cancelled" {
			o.PaidAmount, o.BalanceDue = 0, 0
			continue
		}
		orderIndexes[o.ID] = i
		o.PaidAmount, o.BalanceDue = 0, o.TotalAmount
		o.DepositRequired = min(o.DepositRequired, o.TotalAmount)
	}
	shippingLeft := map[string]int{}
	for i, sh := range s.shipping {
		if sh.CustomerID == customerID && sh.Status != "Cancelled" {
			shippingIndexes[sh.ID], shippingLeft[sh.ID] = i, sh.QuotedAmount
		}
	}
	refs := []int{}
	for i, p := range s.payments {
		if p.CustomerID == customerID && p.Status == "Confirmed" {
			refs = append(refs, i)
		}
	}
	sort.Slice(refs, func(i, j int) bool {
		a, b := s.payments[refs[i]], s.payments[refs[j]]
		x, y := firstNonEmpty(a.ConfirmedAt, a.CreatedAt), firstNonEmpty(b.ConfirmedAt, b.CreatedAt)
		if x == y {
			return a.ID < b.ID
		}
		return x < y
	})
	apply := func(p *Payment, a PaymentAllocation) {
		amount := min(a.Amount, p.CreditLeft)
		if amount <= 0 {
			return
		}
		if a.ManufacturingID != "" && a.ShippingID == "" {
			i, ok := orderIndexes[a.ManufacturingID]
			if !ok {
				return
			}
			o := &s.manufacturing[i]
			amount = min(amount, o.BalanceDue)
			o.PaidAmount += amount
			o.BalanceDue -= amount
		} else if a.ShippingID != "" && a.ManufacturingID == "" {
			if _, ok := shippingIndexes[a.ShippingID]; !ok {
				return
			}
			amount = min(amount, shippingLeft[a.ShippingID])
			shippingLeft[a.ShippingID] -= amount
		} else {
			return
		}
		if amount <= 0 {
			return
		}
		a.Amount = amount
		p.CreditLeft -= amount
		for i := range p.Allocations {
			if p.Allocations[i].ManufacturingID == a.ManufacturingID && p.Allocations[i].ShippingID == a.ShippingID {
				p.Allocations[i].Amount += amount
				return
			}
		}
		p.Allocations = append(p.Allocations, a)
	}
	// Preserve all existing valid allocations before applying unallocated funds.
	for _, i := range refs {
		p := &s.payments[i]
		old := p.Allocations
		p.Allocations = nil
		p.CreditLeft = p.Amount
		for _, a := range old {
			apply(p, a)
		}
	}
	orderedOrders := []int{}
	for _, i := range orderIndexes {
		orderedOrders = append(orderedOrders, i)
	}
	sort.Slice(orderedOrders, func(i, j int) bool {
		a, b := s.manufacturing[orderedOrders[i]], s.manufacturing[orderedOrders[j]]
		if a.Status == "Completed" && b.Status != "Completed" {
			return true
		}
		if b.Status == "Completed" && a.Status != "Completed" {
			return false
		}
		if (a.Status == "Deposit pending") != (b.Status == "Deposit pending") {
			return a.Status == "Deposit pending"
		}
		if a.CreatedAt == b.CreatedAt {
			return a.ID < b.ID
		}
		return a.CreatedAt < b.CreatedAt
	})

	for _, i := range refs {
		p := &s.payments[i]
		if p.ShippingID != "" {
			apply(p, PaymentAllocation{ShippingID: p.ShippingID, Amount: p.CreditLeft})
		}
		if reconcile {
			for _, oi := range orderedOrders {
				o := s.manufacturing[oi]
				if o.Status == "Completed" {
					apply(p, PaymentAllocation{ManufacturingID: o.ID, Amount: p.CreditLeft})
				}
			}
		}
		if oi, ok := orderIndexes[p.ManufacturingID]; ok && s.manufacturing[oi].Status != "Completed" {
			apply(p, PaymentAllocation{ManufacturingID: p.ManufacturingID, Amount: p.CreditLeft})
		}
		for _, oi := range orderedOrders {
			o := s.manufacturing[oi]
			if o.Status != "Completed" {
				apply(p, PaymentAllocation{ManufacturingID: o.ID, Amount: p.CreditLeft})
			}
		}
		// A shipping charge is paid only by a payment explicitly linked to it.
		// General account payments may cover manufacturing orders, then remain as
		// customer credit; they must not be silently redirected to shipping.
	}
	result := []ManufacturingOrder{}
	for _, i := range orderedOrders {
		o := &s.manufacturing[i]
		if o.Status == "Deposit pending" && depositOutstanding(*o) == 0 {
			o.Status, o.CurrentStage, o.Progress = "Confirmed", "Confirmed", 43
			markStage(o.Stages, "Deposit received", "done", now.Format("2006-01-02"))
		}
		if historyPaymentID != "" {
			for _, p := range s.payments {
				if p.ID == historyPaymentID {
					for _, a := range p.Allocations {
						if a.ManufacturingID == o.ID {
							o.History = append([]Update{{Label: "Payment applied", Detail: fmt.Sprintf("Rs %d applied; Rs %d due.", a.Amount, o.BalanceDue), Actor: actorName(actor, "Admin"), CreatedAt: now.Format(time.RFC3339)}}, o.History...)
						}
					}
				}
			}
		}
		result = append(result, *o)
	}
	return result
}

type ReconciliationPlan struct {
	CustomerID string               `json:"customerId"`
	Digest     string               `json:"digest"`
	Orders     []ManufacturingOrder `json:"orders"`
	Payments   []Payment            `json:"payments"`
	Issues     []AccountingIssue    `json:"issues"`
	Applied    bool                 `json:"applied"`
}

func (s *Service) ReconcileAccounting(customerID, digest, reason string, actor auth.User) (ReconciliationPlan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if actor.Role != "Owner" || !auth.CanAccessCustomer(actor, customerID) || customerID == "" {
		return ReconciliationPlan{}, errForbidden
	}
	mutation, err := s.beginFinancialMutationLocked()
	if err != nil {
		return ReconciliationPlan{}, err
	}
	defer mutation.rollback()
	raw, _ := json.Marshal(mutation.before)
	hash := sha256.Sum256(raw)
	plan := ReconciliationPlan{CustomerID: customerID, Digest: hex.EncodeToString(hash[:]), Issues: s.accountingIssuesLocked(customerID), Orders: []ManufacturingOrder{}, Payments: []Payment{}}
	s.allocateCustomerLocked(customerID, "", actor, time.Now().UTC(), true)
	for _, o := range s.manufacturing {
		if o.CustomerID == customerID {
			plan.Orders = append(plan.Orders, o)
		}
	}
	for _, p := range s.payments {
		if p.CustomerID == customerID {
			plan.Payments = append(plan.Payments, p)
		}
	}
	if digest == "" {
		return plan, nil
	}
	if digest != plan.Digest || strings.TrimSpace(reason) == "" {
		return ReconciliationPlan{}, errValidation
	}
	if len(s.accountingIssuesLocked(customerID)) > 0 {
		return ReconciliationPlan{}, errAccountingReview
	}
	for i := range s.manufacturing {
		o := &s.manufacturing[i]
		if o.CustomerID != customerID {
			continue
		}
		before := mutation.before.Orders[i]
		if before.PaidAmount != o.PaidAmount || before.BalanceDue != o.BalanceDue {
			o.History = append([]Update{{Label: "Accounting reconciled", Detail: fmt.Sprintf("Paid %d -> %d; due %d -> %d. %s", before.PaidAmount, o.PaidAmount, before.BalanceDue, o.BalanceDue, reason), Actor: actorName(actor, "Owner"), CreatedAt: time.Now().UTC().Format(time.RFC3339)}}, o.History...)
		}
	}
	if err := mutation.commit(actor); err != nil {
		return ReconciliationPlan{}, err
	}
	plan.Applied = true
	s.record(actor, "accounting.reconciled", customerID, "Allocation-only reconciliation; charges and receipts unchanged. "+reason)
	s.notifyAccountingRecordsLocked(customerID)
	return plan, nil
}

func (s *Service) notifyAccountingRecordsLocked(customerID string) {
	changes := []WorkspaceChange{}
	for _, o := range s.manufacturing {
		if o.CustomerID == customerID {
			changes = append(changes, workspaceUpsert("manufacturing", customerID, o))
		}
	}
	for _, p := range s.payments {
		if p.CustomerID == customerID {
			changes = append(changes, workspaceUpsert("payments", customerID, p))
		}
	}
	for _, l := range s.ledger {
		if l.CustomerID == customerID {
			changes = append(changes, workspaceUpsert("ledger", customerID, l))
		}
	}
	s.notifyWorkspaceChanges(changes...)
}
