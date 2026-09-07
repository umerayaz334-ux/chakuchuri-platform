package files

import (
	"errors"
	"testing"
)

type pointFileRepository struct {
	controlledFileRepository
	writes int
	loads  int
}

func (r *pointFileRepository) Check() error { return nil }
func (r *pointFileRepository) LoadFiles() ([]FileRecord, string, error) {
	r.loads++
	return nil, "", errors.New("full load forbidden")
}
func (r *pointFileRepository) SaveFiles([]FileRecord) error {
	return errors.New("whole-list save forbidden")
}
func (r *pointFileRepository) SaveFile(row FileRecord) error {
	r.writes++
	if r.saveErr != nil {
		return r.saveErr
	}
	r.records = append(r.records, row)
	return nil
}
func (r *pointFileRepository) GetFile(tenantID, id string) (FileRecord, bool, error) {
	for _, row := range r.records {
		if row.TenantID == tenantID && row.ID == id {
			return row, true, nil
		}
	}
	return FileRecord{}, false, nil
}

func TestPointRepositoryDoesNotCacheOrRewriteCatalog(t *testing.T) {
	repo := &pointFileRepository{}
	s := NewService(nil, nil, t.TempDir())
	if err := s.EnableRepository(repo); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		record, err := s.storeUpload([]byte("%PDF-1.4\n"), "proof.pdf", "application/pdf", UploadRequest{OwnerType: "payment_proof"}, defaultTenantExternalID, "test")
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := s.Get(record.ID); !ok {
			t.Fatal("point read failed")
		}
	}
	if repo.writes != 3 || repo.loads != 0 || len(s.records) != 0 || len(s.byID) != 0 {
		t.Fatal("upload used whole catalog")
	}
	repo.saveErr = errors.New("offline")
	if _, err := s.storeUpload([]byte("%PDF-1.4\n"), "proof.pdf", "application/pdf", UploadRequest{}, defaultTenantExternalID, "test"); err == nil {
		t.Fatal("failed SQL write acknowledged")
	}
}
