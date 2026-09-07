package workflow

import (
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"chakuchuri/backend/internal/auth"
	"chakuchuri/backend/internal/platform/pgtest"
)

func TestPostgresFinancialAtomicityAndConcurrentWriters(t *testing.T) {
	db := pgtest.New(t)
	s := NewService(nil, nil)
	repository := NewPostgresRepository(db)
	s.repository = repository
	if err := s.saveStateLocked(); err != nil {
		t.Fatal(err)
	}
	actor := auth.User{Role: "Owner", Status: "Active"}
	payment, err := s.CreatePayment(PaymentRequest{CustomerID: "cust_abc_export", Amount: 10000}, actor)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.ConfirmPayment(payment.ID, actor); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	loaded, _, err := NewPostgresRepository(db).LoadState()
	if err != nil {
		t.Fatal(err)
	}
	count, credit := 0, 0
	for _, entry := range loaded.Ledger {
		if entry.SourceType == "payment" && entry.SourceID == payment.ID {
			count++
			credit += entry.Credit
		}
	}
	if count != 1 || credit != 10000 {
		t.Fatalf("duplicate durable credit: count=%d amount=%d", count, credit)
	}
	for _, entry := range s.ledger {
		if entry.SourceType == "payment" && entry.SourceID == payment.ID {
			found := false
			for _, stored := range loaded.Ledger {
				found = found || stored.ID == entry.ID
			}
			if !found {
				t.Fatal("ledger public ID changed on reload")
			}
		}
	}

	// Fail after order/payment SQL statements have executed but before COMMIT.
	if _, err := db.Exec(`CREATE FUNCTION reject_ledger_test() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected ledger failure'; END $$;
CREATE TRIGGER reject_ledger_test BEFORE INSERT OR UPDATE ON ledger_entries FOR EACH ROW EXECUTE FUNCTION reject_ledger_test()`); err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(financialState{s.quotations, s.manufacturing, s.shipping, s.payments, s.ledger, s.lastSequenceDay, s.sequence})
	if _, err := s.EditOrder("MFG-24091", OrderEditRequest{ProductName: "Amended", Quantity: 600, UnitPrice: 900, ExpectedDate: "2026-10-01", Reason: "Test", Confirmation: "MFG-24091"}, actor); !errors.Is(err, errPersistence) {
		t.Fatalf("expected storage failure, got %v", err)
	}
	after, _ := json.Marshal(financialState{s.quotations, s.manufacturing, s.shipping, s.payments, s.ledger, s.lastSequenceDay, s.sequence})
	if string(before) != string(after) {
		t.Fatal("SQL rollback left in-memory changes")
	}
	durable, _, err := NewPostgresRepository(db).LoadState()
	if err != nil {
		t.Fatal(err)
	}
	left, _ := json.Marshal(loaded.Manufacturing)
	right, _ := json.Marshal(durable.Manufacturing)
	if string(left) != string(right) {
		t.Fatal("order changed despite ledger failure")
	}
	if _, err := db.Exec("DROP TRIGGER reject_ledger_test ON ledger_entries; DROP FUNCTION reject_ledger_test()"); err != nil {
		t.Fatal(err)
	}

	// Independently loaded API writers cannot silently overwrite each other's state.
	one, two := NewPostgresRepository(db), NewPostgresRepository(db)
	first, _, err := one.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := two.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	first.Manufacturing[0].ProductName = "Writer one"
	second.Manufacturing[0].ProductName = "Writer two"
	start := make(chan struct{})
	results := make(chan error, 2)
	go func() { <-start; results <- one.SaveState(first) }()
	go func() { <-start; results <- two.SaveState(second) }()
	close(start)
	success, stale := 0, 0
	for i := 0; i < 2; i++ {
		err := <-results
		if err == nil {
			success++
		} else if errors.Is(err, ErrStaleWorkflow) {
			stale++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || stale != 1 {
		t.Fatalf("writer conflict not detected: successful=%d stale=%d", success, stale)
	}
}
