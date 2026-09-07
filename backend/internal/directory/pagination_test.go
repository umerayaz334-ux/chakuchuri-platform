package directory

import (
	"fmt"
	"net/http/httptest"
	"testing"
)

func TestPaginateListingsDefaultsToTwentyAndReportsTotals(t *testing.T) {
	rows := make([]Listing, 45)
	for index := range rows {
		rows[index].ID = fmt.Sprintf("listing-%d", index)
	}

	request := httptest.NewRequest("GET", "/api/directory/listings", nil)
	page, info := paginateListings(request, rows)

	if len(page) != 20 || info["loaded"] != 20 || info["total"] != 45 || info["hasMore"] != true {
		t.Fatalf("unexpected first page: len=%d info=%v", len(page), info)
	}
}

func TestPaginateListingsBoundsOffsetAndLimit(t *testing.T) {
	rows := make([]Listing, 150)
	request := httptest.NewRequest("GET", "/api/directory/listings?offset=-4&limit=500", nil)
	page, info := paginateListings(request, rows)

	if len(page) != 100 || info["loaded"] != 100 || info["hasMore"] != true {
		t.Fatalf("unexpected bounded page: len=%d info=%v", len(page), info)
	}

	request = httptest.NewRequest("GET", "/api/directory/listings?offset=999&limit=20", nil)
	page, info = paginateListings(request, rows)
	if len(page) != 0 || info["loaded"] != 150 || info["hasMore"] != false {
		t.Fatalf("unexpected exhausted page: len=%d info=%v", len(page), info)
	}
}
