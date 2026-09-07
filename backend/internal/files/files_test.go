package files

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http/httptest"
	"testing"

	"chakuchuri/backend/internal/auth"
)

func TestCreateGeneratesUniqueIDsForRapidUploads(t *testing.T) {
	service := NewService(nil, nil, t.TempDir())
	actor := auth.User{ID: "user_admin", TenantID: "tenant_chakuchuri", Name: "Admin", Role: "Admin", Status: "Active"}
	seen := map[string]bool{}

	for index := 0; index < 5; index++ {
		file, header := testMultipartFile(t, fmt.Sprintf("attachment-%d.csv", index), []byte(fmt.Sprintf("sku,quantity\nitem-%d,1\n", index)))
		record, err := service.Create(file, header, UploadRequest{OwnerType: "message", OwnerID: "conversation_test"}, actor)
		if err != nil {
			t.Fatalf("create attachment %d: %v", index, err)
		}
		if seen[record.ID] {
			t.Fatalf("duplicate file ID generated for rapid upload: %s", record.ID)
		}
		seen[record.ID] = true
		if stored, ok := service.Get(record.ID); !ok || stored.OriginalName != header.Filename {
			t.Fatalf("attachment %d was not independently retrievable", index)
		}
	}

	if len(seen) != 5 {
		t.Fatalf("expected five unique upload IDs, got %d", len(seen))
	}
}

func testMultipartFile(t *testing.T, name string, contents []byte) (multipart.File, *multipart.FileHeader) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", name)
	if err != nil {
		t.Fatalf("create multipart part: %v", err)
	}
	if _, err := part.Write(contents); err != nil {
		t.Fatalf("write multipart file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	request := httptest.NewRequest("POST", "/api/files", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	if err := request.ParseMultipartForm(maxUploadBytes); err != nil {
		t.Fatalf("parse multipart form: %v", err)
	}
	file, header, err := request.FormFile("file")
	if err != nil {
		t.Fatalf("open multipart file: %v", err)
	}
	return file, header
}
