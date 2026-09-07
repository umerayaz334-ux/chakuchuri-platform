package workflow

import (
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"chakuchuri/backend/internal/auth"
)

func TestFinancialFailuresRestoreStateBeforePublishing(t *testing.T) {
	actor := auth.User{Role: "Owner", Status: "Active"}
	for _, name := range []string{"payment", "confirm", "edit", "cancel", "stage", "update", "accept", "shipping"} {
		t.Run(name, func(t *testing.T) {
			s := NewService(nil, nil)
			payment, err := s.CreatePayment(PaymentRequest{CustomerID: "cust_abc_export", Amount: 10000}, actor)
			if err != nil {
				t.Fatal(err)
			}
			s.quotations[1].Status = "Priced"
			client := &workspaceClient{user: actor, send: make(chan workspaceSocketEvent, 32), done: make(chan struct{})}
			s.workspaceEvents.join(client)
			<-client.send
			before, _ := json.Marshal(financialState{s.quotations, s.manufacturing, s.shipping, s.payments, s.ledger, s.lastSequenceDay, s.sequence})
			repo := &appHomeTestRepository{fail: true, beforeSave: func() {
				if len(client.send) != 0 {
					t.Error("published before commit")
				}
			}}
			s.repository = repo
			switch name {
			case "payment":
				_, err = s.CreatePayment(PaymentRequest{CustomerID: "cust_abc_export", Amount: 100}, actor)
			case "confirm":
				_, err = s.ConfirmPayment(payment.ID, actor)
			case "edit":
				_, err = s.EditOrder("MFG-24091", OrderEditRequest{ProductName: "Revised", Quantity: 550, UnitPrice: 900, ExpectedDate: "2026-10-01", Reason: "Customer change", Confirmation: "MFG-24091"}, actor)
			case "cancel":
				_, err = s.CancelOrder("MFG-24091", CancelOrderRequest{Reason: "Customer request", Confirmation: "MFG-24091"}, actor)
			case "stage":
				_, err = s.MoveOrder("MFG-24091", "Production", actor)
			case "update":
				_, err = s.UpdateOrder("MFG-24091", OrderUpdateRequest{Detail: "Update"}, actor)
			case "accept":
				_, err = s.AcceptQuote(s.quotations[1].ID, actor)
			case "shipping":
				_, err = s.CreateShipping(ShippingRequestPayload{CustomerID: "cust_abc_export", Destination: "USA"}, actor)
			}
			if !errors.Is(err, errPersistence) {
				t.Fatalf("want persistence failure, got %v", err)
			}
			after, _ := json.Marshal(financialState{s.quotations, s.manufacturing, s.shipping, s.payments, s.ledger, s.lastSequenceDay, s.sequence})
			if string(before) != string(after) {
				t.Fatal("failed write changed financial state")
			}
			if len(client.send) != 0 {
				t.Fatal("failed write published a workspace event")
			}
		})
	}
}

func TestConcurrentConfirmationCreditsOnce(t *testing.T) {
	s := NewService(nil, nil)
	actor := auth.User{Role: "Owner", Status: "Active"}
	payment, err := s.CreatePayment(PaymentRequest{CustomerID: "cust_abc_export", Amount: 10000}, actor)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.ConfirmPayment(payment.ID, actor); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	count, credit := 0, 0
	for _, entry := range s.ledger {
		if entry.SourceType == "payment" && entry.SourceID == payment.ID {
			count++
			credit += entry.Credit
		}
	}
	if count != 1 || credit != 10000 {
		t.Fatalf("duplicate credit: count=%d credit=%d", count, credit)
	}
}
