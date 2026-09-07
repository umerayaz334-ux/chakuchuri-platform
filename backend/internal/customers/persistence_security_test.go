package customers

import (
	"os"
	"strings"
	"testing"
)

func TestFileRepositoryNeverPersistsRawDocumentRequestToken(t *testing.T) {
	path := t.TempDir() + "/customers.dev.json"
	repository := fileRepository{path: path}
	rawToken := "very-secret-verification-token"
	request := DocumentRequest{
		ID:         "kyc_req_test",
		Token:      rawToken,
		CustomerID: "cust_test",
		Kinds:      append([]string{}, requiredKYCKinds...),
		Status:     "active",
		CreatedAt:  "2026-09-05T00:00:00Z",
		ExpiresAt:  "2099-09-05T00:00:00Z",
		CreatedBy:  "owner@example.com",
	}
	customers := []Customer{{ID: "cust_test", CompanyName: "Test Export House"}}

	if err := repository.SaveCustomers(customers, []DocumentRequest{request}); err != nil {
		t.Fatalf("save customer snapshot: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read customer snapshot: %v", err)
	}
	if strings.Contains(string(raw), rawToken) {
		t.Fatal("customer snapshot contains the raw verification token")
	}
	if !strings.Contains(string(raw), hashRequestToken(rawToken)) {
		t.Fatal("customer snapshot is missing the verification token hash")
	}

	_, requests, _, err := repository.LoadCustomers()
	if err != nil {
		t.Fatalf("load customer snapshot: %v", err)
	}
	if len(requests) != 1 || requests[0].Token != "" || requests[0].TokenHash != hashRequestToken(rawToken) {
		t.Fatalf("unexpected persisted request: %#v", requests)
	}
}
