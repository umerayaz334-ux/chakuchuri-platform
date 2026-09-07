package main

import (
	"testing"

	"chakuchuri/backend/internal/customers"
	"chakuchuri/backend/internal/files"
	"chakuchuri/backend/internal/workflow"
)

func TestPreparationPreservesFileWinnerAndAllBlobs(t *testing.T) {
	var s source
	s.Customers.Customers = []customers.Customer{{ID: "customer", VerificationStatus: "verified", DocumentsSubmittedAt: "2026-09-02T12:00:00Z"}}
	s.Customers.Requests = []customers.DocumentRequest{{ID: "link", CustomerID: "customer", Status: "active", CreatedAt: "2026-09-01T12:00:00Z"}}
	s.Files.Files = []files.FileRecord{
		{ID: "duplicate", TenantID: "tenant_chakuchuri", CustomerID: "old", StorageKey: "one.png", URL: "/api/files/duplicate/content"},
		{ID: "duplicate", TenantID: "tenant_chakuchuri", CustomerID: "customer", StorageKey: "two.png", URL: "/api/files/duplicate/content"},
	}
	s.Workflow.Payments = []workflow.Payment{{CustomerID: "customer", ProofFileID: "duplicate"}}
	repairs, err := prepareRepairs(&s)
	if err != nil {
		t.Fatal(err)
	}
	if len(repairs) != 3 || len(s.Files.Files) != 2 || s.Files.Files[1].ID != "duplicate" || s.Files.Files[1].StorageKey != "two.png" || s.Files.Files[0].StorageKey != "one.png" || s.Files.Files[0].ID == "duplicate" {
		t.Fatal("preparation lost data or changed public file winner")
	}
	if err := validateSource(s); err != nil {
		t.Fatal(err)
	}
	if s.Customers.Requests[0].Status != "submitted" {
		t.Fatal("stale verified-account request remains open")
	}
}

func TestPreparationRejectsAmbiguousOwnership(t *testing.T) {
	var s source
	s.Files.Files = []files.FileRecord{{ID: "orphan", CustomerID: "missing"}}
	if _, err := prepareRepairs(&s); err == nil {
		t.Fatal("guessed orphan ownership")
	}
}

func TestValidateRejectsDuplicateMetadata(t *testing.T) {
	var s source
	s.Files.Files = []files.FileRecord{{ID: "same", TenantID: "tenant_chakuchuri", StorageKey: "one"}, {ID: "same", TenantID: "tenant_chakuchuri", StorageKey: "two"}}
	if err := validateSource(s); err == nil {
		t.Fatal("duplicate records would be silently collapsed")
	}
}
