package files

import (
	"bytes"
	"fmt"
	"net/http/httptest"
	"sync"
	"testing"

	"chakuchuri/backend/internal/auth"
	"chakuchuri/backend/internal/platform/pgtest"
)

func TestPostgresFileIsolationAndPhotoStorage(t *testing.T) {
	db := pgtest.New(t)
	s := NewService(nil, nil, t.TempDir())
	if err := s.EnableRepository(NewPostgresRepository(db)); err != nil {
		t.Fatal(err)
	}
	raw := photoFixture(t, 320, 160)
	for _, owner := range []string{"message", "user_profile", "product_image", "quotation_photo", "directory_listing", "customer_document", "payment_proof"} {
		record, err := s.storeUpload(raw, "sample.png", "image/png", UploadRequest{OwnerType: owner, OwnerID: "cust_test", CustomerID: "cust_test"}, defaultTenantExternalID, "test")
		if err != nil {
			t.Fatal(err)
		}
		got, content, err := s.Read(record.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got != record {
			t.Fatal("metadata round trip changed")
		}
		protected := owner == "customer_document" || owner == "payment_proof"
		if protected && !bytes.Equal(raw, content) {
			t.Fatal("protected original modified")
		}
		if !protected && (got.MimeType != "image/webp" || got.ByteSize+got.PreviewBytes >= got.SourceBytes) {
			t.Fatalf("photo not optimized: %+v", got)
		}
		response := httptest.NewRecorder()
		s.serveRecord(response, httptest.NewRequest("GET", record.URL, nil), got, false)
		if response.Code != 200 || !bytes.Equal(response.Body.Bytes(), content) {
			t.Fatal("stored content not served")
		}
	}
	if len(s.records) != 0 || len(s.byID) != 0 {
		t.Fatal("SQL file service cached whole catalog")
	}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r := NewPostgresRepository(db)
			err := r.SaveFile(FileRecord{ID: fmt.Sprintf("concurrent_%d", i), TenantID: defaultTenantExternalID, StorageKey: fmt.Sprintf("test/%d.pdf", i), OriginalName: "test.pdf", MimeType: "application/pdf", ByteSize: 10})
			if err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	r := NewPostgresRepository(db)
	for _, tenant := range []string{defaultTenantExternalID, "tenant_other"} {
		if err := r.SaveFile(FileRecord{ID: "same_logical_id", TenantID: tenant, StorageKey: tenant + "/file.pdf", OriginalName: "private.pdf", MimeType: "application/pdf", ByteSize: 10}); err != nil {
			t.Fatal(err)
		}
	}
	for _, tenant := range []string{defaultTenantExternalID, "tenant_other"} {
		got, ok, err := r.GetFile(tenant, "same_logical_id")
		if err != nil || !ok || got.TenantID != tenant {
			t.Fatal("cross-tenant point query failed")
		}
		if record, ok := s.GetForUser("same_logical_id", auth.User{TenantID: tenant, Role: "Owner"}); !ok || record.TenantID != tenant {
			t.Fatal("service selected another tenant's file")
		}
	}
	if _, ok, err := r.GetFile("unknown_tenant", "same_logical_id"); err != nil || ok {
		t.Fatal("unknown tenant could read metadata")
	}
	var count int
	if err := db.QueryRow("SELECT count(*) FROM files WHERE external_id LIKE 'concurrent_%'").Scan(&count); err != nil || count != 20 {
		t.Fatalf("concurrent file writes lost: %d %v", count, err)
	}
}
