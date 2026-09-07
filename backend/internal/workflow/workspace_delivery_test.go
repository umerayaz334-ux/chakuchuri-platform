package workflow

import (
	"sync"
	"testing"

	"chakuchuri/backend/internal/auth"
)

func deliveryClient(h *workspaceEventHub, customerID string, capacity int) *workspaceClient {
	c := &workspaceClient{
		user: auth.User{Role: "Customer", Status: "Active", CustomerID: customerID},
		send: make(chan workspaceSocketEvent, capacity), done: make(chan struct{}),
	}
	h.join(c)
	return c
}

func TestWorkspaceDeliverySkipsUnrelatedCustomers(t *testing.T) {
	h := newWorkspaceEventHub()
	a := deliveryClient(h, "a", 16)
	b := deliveryClient(h, "b", 16)
	for _, c := range []*workspaceClient{a, b} {
		if ready := <-c.send; ready.Type != "ready" || ready.Revision != 0 {
			t.Fatal("subscription must begin with ready")
		}
	}
	h.publish([]WorkspaceChange{workspaceUpsert("quotations", "a", map[string]string{"id": "q1"})})
	h.publish([]WorkspaceChange{workspaceUpsert("quotations", "b", map[string]string{"id": "q2"})})
	h.publish([]WorkspaceChange{workspaceUpsert("quotations", "a", map[string]string{"id": "q3"})})
	if len(a.send) != 2 || len(b.send) != 1 {
		t.Fatal("unrelated event generated a frame")
	}
	for _, want := range []uint64{1, 2} {
		if event := <-a.send; event.Revision != want || event.Changes[0].CustomerID != "a" {
			t.Fatalf("incorrect customer sequence: %+v", event)
		}
	}
	if event := <-b.send; event.Revision != 1 {
		t.Fatal("customer sequence contains a global gap")
	}
	h.pong(b)
	if event := <-b.send; event.Type != "pong" || event.Revision != 1 {
		t.Fatal("pong used global revision")
	}
	h.leave(b)
	h.publish([]WorkspaceChange{workspaceUpsert("quotations", "b", map[string]string{"id": "q4"})})
	h.pong(b)
	if len(b.send) != 0 {
		t.Fatal("departed client received an event")
	}
}

func TestConcurrentWorkspacePublishersStayOrdered(t *testing.T) {
	h := newWorkspaceEventHub()
	c := deliveryClient(h, "a", 128)
	<-c.send
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h.publish([]WorkspaceChange{workspaceUpsert("presence", "a", map[string]string{"userId": "u"})})
		}()
	}
	wg.Wait()
	for want := uint64(1); want <= 100; want++ {
		if event := <-c.send; event.Revision != want {
			t.Fatalf("want %d, got %d", want, event.Revision)
		}
	}
}

func TestSlowWorkspaceClientDisconnectsWithoutBlockingOthers(t *testing.T) {
	h := newWorkspaceEventHub()
	slow := deliveryClient(h, "a", 1)
	fast := deliveryClient(h, "a", 8)
	<-fast.send
	h.publish([]WorkspaceChange{workspaceUpsert("quotations", "a", map[string]string{"id": "q1"})})
	select {
	case <-slow.done:
	default:
		t.Fatal("overflowed client stayed connected")
	}
	if len(fast.send) != 1 {
		t.Fatal("slow subscriber blocked healthy subscriber")
	}
}
