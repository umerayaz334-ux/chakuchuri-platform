package email

import (
	"net/http/httptest"
	"testing"
)

func TestPaginateEmailSnapshotLimitsActivityHistory(t *testing.T) {
	snapshot := Snapshot{
		Outbox:     make([]OutboxItem, 31),
		Deliveries: make([]Delivery, 45),
		Metrics:    Metrics{Captured: 45},
	}
	request := httptest.NewRequest("GET", "/api/email", nil)

	page := paginateEmailSnapshot(request, snapshot)
	if len(page.Outbox) != 20 || len(page.Deliveries) != 20 {
		t.Fatalf("activity was not limited: outbox=%d deliveries=%d", len(page.Outbox), len(page.Deliveries))
	}
	if page.Pagination["deliveries"].Total != 45 || !page.Pagination["deliveries"].HasMore {
		t.Fatalf("unexpected delivery page: %+v", page.Pagination["deliveries"])
	}
	if page.Metrics.Captured != 45 {
		t.Fatalf("aggregate metrics changed: %+v", page.Metrics)
	}
}

func TestPaginateEmailSnapshotReturnsRequestedDeliveryPageOnly(t *testing.T) {
	snapshot := Snapshot{Outbox: make([]OutboxItem, 9), Deliveries: make([]Delivery, 52)}
	request := httptest.NewRequest("GET", "/api/email?scope=deliveries&offset=20&limit=20", nil)

	page := paginateEmailSnapshot(request, snapshot)
	if len(page.Deliveries) != 20 || len(page.Outbox) != 0 || page.Pagination["deliveries"].Loaded != 40 {
		t.Fatalf("unexpected delivery page: deliveries=%d outbox=%d pagination=%+v", len(page.Deliveries), len(page.Outbox), page.Pagination)
	}
}
