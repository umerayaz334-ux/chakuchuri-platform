package files

import (
	"errors"
	"io/fs"
	"path/filepath"
	"testing"

	"chakuchuri/backend/internal/auth"
)

type controlledFileRepository struct {
	records   []FileRecord
	saveErr   error
	deleteErr error
}

func (r *controlledFileRepository) LoadFiles() ([]FileRecord, string, error) {
	return append([]FileRecord(nil), r.records...), "test.repository", nil
}

func (r *controlledFileRepository) SaveFiles(records []FileRecord) error {
	if r.saveErr != nil {
		return r.saveErr
	}
	r.records = append([]FileRecord(nil), records...)
	return nil
}

func (r *controlledFileRepository) DeleteFile(record FileRecord) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	next := make([]FileRecord, 0, len(r.records))
	for _, row := range r.records {
		if row.ID != record.ID {
			next = append(next, row)
		}
	}
	r.records = next
	return nil
}

func TestUploadFailureRollsBackMetadataAndStorage(t *testing.T) {
	storageDir := t.TempDir()
	repository := &controlledFileRepository{saveErr: errors.New("database unavailable")}
	service := NewService(nil, nil, storageDir)
	service.EnableRepository(repository)
	actor := auth.User{ID: "owner", TenantID: "tenant_test", Role: "Owner", Email: "owner@example.com"}
	pdf, header := testMultipartFile(t, "identity.pdf", []byte("%PDF-1.4\n1 0 obj\n<<>>\nendobj\n"))

	if _, err := service.Create(pdf, header, UploadRequest{
		CustomerID: "cust_test", OwnerType: "customer_document", OwnerID: "cust_test",
	}, actor); err == nil {
		t.Fatal("upload succeeded while metadata persistence failed")
	}
	if len(service.records) != 0 || len(service.byID) != 0 {
		t.Fatal("failed upload remained in the in-memory index")
	}
	regularFiles := 0
	if err := filepath.WalkDir(storageDir, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			regularFiles++
		}
		return nil
	}); err != nil {
		t.Fatalf("inspect storage: %v", err)
	}
	if regularFiles != 0 {
		t.Fatalf("failed upload left %d stored file(s)", regularFiles)
	}
}

func TestDiscardKeepsUsableFileWhenMetadataDeleteFails(t *testing.T) {
	repository := &controlledFileRepository{}
	service := NewService(nil, nil, t.TempDir())
	service.EnableRepository(repository)
	actor := auth.User{ID: "owner", TenantID: "tenant_test", Role: "Owner", Email: "owner@example.com"}
	pdf, header := testMultipartFile(t, "identity.pdf", []byte("%PDF-1.4\n1 0 obj\n<<>>\nendobj\n"))
	record, err := service.Create(pdf, header, UploadRequest{
		CustomerID: "cust_test", OwnerType: "customer_document", OwnerID: "cust_test",
	}, actor)
	if err != nil {
		t.Fatalf("create KYC file: %v", err)
	}

	repository.deleteErr = errors.New("database unavailable")
	if service.DiscardCustomerDocument(record.ID, "cust_test") {
		t.Fatal("discard reported success while metadata deletion failed")
	}
	if _, _, err := service.Read(record.ID); err != nil {
		t.Fatalf("failed discard made the still-indexed file unusable: %v", err)
	}

	repository.deleteErr = nil
	if !service.DiscardCustomerDocument(record.ID, "cust_test") {
		t.Fatal("discard failed after metadata repository recovered")
	}
	if _, ok := service.Get(record.ID); ok {
		t.Fatal("discarded file remained in memory")
	}
	if len(repository.records) != 0 {
		t.Fatal("discarded file remained in repository metadata")
	}
}
