package files

import (
	"testing"

	"chakuchuri/backend/internal/auth"
)

func TestCustomerDocumentUploadUsesStrictMIMEAllowlist(t *testing.T) {
	service := NewService(nil, nil, t.TempDir())
	actor := auth.User{ID: "owner", TenantID: "tenant_test", Role: "Owner", Email: "owner@example.com"}

	csv, csvHeader := testMultipartFile(t, "identity.csv", []byte("name,id\nPerson,123\n"))
	if _, err := service.Create(csv, csvHeader, UploadRequest{
		CustomerID: "cust_test", OwnerType: "customer_document", OwnerID: "cust_test",
	}, actor); err == nil {
		t.Fatal("KYC upload accepted a CSV file")
	}

	pdf, pdfHeader := testMultipartFile(t, "identity.pdf", []byte("%PDF-1.4\n1 0 obj\n<<>>\nendobj\n"))
	record, err := service.Create(pdf, pdfHeader, UploadRequest{
		CustomerID: "cust_test", OwnerType: "customer_document", OwnerID: "cust_test",
	}, actor)
	if err != nil {
		t.Fatalf("KYC upload rejected a PDF: %v", err)
	}
	if !service.IsCustomerDocument(record.ID, "cust_test") {
		t.Fatal("stored KYC file failed ownership validation")
	}
	if !service.DiscardCustomerDocument(record.ID, "cust_test") {
		t.Fatal("failed to discard unattached KYC file")
	}
	if _, ok := service.Get(record.ID); ok {
		t.Fatal("discarded KYC metadata is still available")
	}
}

func TestKYCFileAccessAlwaysUsesDedicatedPolicy(t *testing.T) {
	service := NewService(nil, nil, t.TempDir())
	actor := auth.User{ID: "owner", TenantID: "tenant_test", Role: "Owner", Email: "owner@example.com"}
	pdf, header := testMultipartFile(t, "identity.pdf", []byte("%PDF-1.4\n1 0 obj\n<<>>\nendobj\n"))
	record, err := service.Create(pdf, header, UploadRequest{
		CustomerID: "cust_test", OwnerType: "customer_document", OwnerID: "cust_test",
	}, actor)
	if err != nil {
		t.Fatalf("create KYC file: %v", err)
	}

	service.SetCustomerDocumentAccess(func(auth.User, string) bool { return false })
	if _, ok := service.GetForUser(record.ID, actor); ok {
		t.Fatal("internal user bypassed the KYC access policy")
	}
	service.SetCustomerDocumentAccess(func(auth.User, string) bool { return true })
	if _, ok := service.GetForUser(record.ID, actor); !ok {
		t.Fatal("authorized KYC reviewer could not access the file")
	}
}
