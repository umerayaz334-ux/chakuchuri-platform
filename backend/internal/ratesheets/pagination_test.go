package ratesheets

import (
	"net/http/httptest"
	"testing"
)

func TestPaginateRateSnapshotDefaultsToTwenty(t *testing.T) {
	books := make([]BookSummary, 47)
	snapshot := Snapshot{Books: books, ActiveBooks: 3, ActiveServices: 8}
	request := httptest.NewRequest("GET", "/api/shipping-rates", nil)

	page := paginateRateSnapshot(request, snapshot)
	if len(page.Books) != 20 || page.Pagination.Loaded != 20 || page.Pagination.Total != 47 || !page.Pagination.HasMore {
		t.Fatalf("unexpected page: books=%d pagination=%+v", len(page.Books), page.Pagination)
	}
	if page.ActiveBooks != 3 || page.ActiveServices != 8 {
		t.Fatalf("aggregate metrics changed during pagination: %+v", page)
	}
}

func TestPaginateRateSnapshotCapsLimit(t *testing.T) {
	request := httptest.NewRequest("GET", "/api/shipping-rates?offset=-5&limit=500", nil)
	page := paginateRateSnapshot(request, Snapshot{Books: make([]BookSummary, 125)})
	if len(page.Books) != 100 || page.Pagination.Loaded != 100 || !page.Pagination.HasMore {
		t.Fatalf("unexpected bounded page: books=%d pagination=%+v", len(page.Books), page.Pagination)
	}
}
