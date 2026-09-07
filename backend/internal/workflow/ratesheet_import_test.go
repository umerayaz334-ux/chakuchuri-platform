package workflow

import (
	"testing"

	"chakuchuri/backend/internal/auth"
)

func TestImportRateSheetsUpsertsRows(t *testing.T) {
	service := NewService(nil, nil)
	service.rateSheets = nil
	actor := auth.User{Email: "admin@chakuchuri.pk", Role: "Owner", TenantID: "tenant_chakuchuri"}

	rows, err := service.ImportRateSheets([]RateSheetRequest{
		{Courier: "FedEx", Service: "Duty paid premium", Zone: "7", Weight: "0-5 kg", Price: 18800, Status: "Active", SourceName: "rates.csv"},
		{Courier: "FedEx", Service: "Duty paid premium", Zone: "7", Weight: "0-5 kg", Price: 19000, Status: "Active", SourceName: "rates.csv"},
	}, actor)
	if err != nil {
		t.Fatalf("import rate sheets: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 returned rows, got %d", len(rows))
	}

	snapshot := service.DashboardForUser(actor)
	if len(snapshot.RateSheets) != 1 {
		t.Fatalf("expected one stored upserted rate, got %d", len(snapshot.RateSheets))
	}
	if snapshot.RateSheets[0].Price != 19000 || snapshot.RateSheets[0].SourceName != "rates.csv" {
		t.Fatalf("unexpected stored rate: %#v", snapshot.RateSheets[0])
	}
}
