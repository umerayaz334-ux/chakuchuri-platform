package files

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"chakuchuri/backend/internal/auth"
)

func TestPrivateFileRevalidation(t *testing.T) {
	service := NewService(nil, nil, t.TempDir())
	actor := auth.User{ID: "owner", Role: "Owner", Status: "Active"}
	file, header := testMultipartFile(t, "photo.csv", []byte("sku,quantity\nitem,1\n"))
	record, err := service.Create(file, header, UploadRequest{OwnerType: "message"}, actor)
	if err != nil {
		t.Fatal(err)
	}
	serve := func(etag string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, "/api/files/"+record.ID+"/content", nil)
		if etag != "" {
			r.Header.Set("If-None-Match", etag)
		}
		w := httptest.NewRecorder()
		service.Serve(w, r, record.ID, false)
		return w
	}
	first := serve("")
	if first.Code != 200 || first.Header().Get("Cache-Control") != "private, no-cache" ||
		first.Header().Get("ETag") == "" || !strings.Contains(first.Header().Get("Vary"), "Authorization") {
		t.Fatalf("unexpected response: %d %v", first.Code, first.Header())
	}
	cached := serve(first.Header().Get("ETag"))
	if cached.Code != http.StatusNotModified || cached.Body.Len() != 0 {
		t.Fatalf("expected empty 304, got %d", cached.Code)
	}
	if err := service.writeStorageFile(record.StorageKey, []byte("changed content,more bytes")); err != nil {
		t.Fatal(err)
	}
	changed := serve(first.Header().Get("ETag"))
	if changed.Code != 200 || changed.Header().Get("ETag") == first.Header().Get("ETag") {
		t.Fatal("changed file retained its old validator")
	}
}

func TestIdentityDocumentsAreNotCacheable(t *testing.T) {
	service := NewService(nil, nil, t.TempDir())
	actor := auth.User{ID: "owner", Role: "Owner", Status: "Active"}
	file, header := testMultipartFile(t, "identity.pdf", []byte("%PDF-1.4\n1 0 obj\n<<>>\nendobj\n"))
	record, err := service.Create(file, header, UploadRequest{OwnerType: "customer_document", CustomerID: "c1"}, actor)
	if err != nil {
		t.Fatal(err)
	}
	for _, thumbnail := range []bool{false, true} {
		w := httptest.NewRecorder()
		service.Serve(w, httptest.NewRequest(http.MethodGet, "/file", nil), record.ID, thumbnail)
		if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("ETag") != "" {
			t.Fatalf("KYC cache policy: %d %v", w.Code, w.Header())
		}
	}
}

func TestConditionalRequestsCannotBypassFileAuthorization(t *testing.T) {
	accounts := auth.NewService(nil)
	owner, err := accounts.StartSession("admin@chakuchuri.pk")
	if err != nil {
		t.Fatal(err)
	}
	customer, err := accounts.StartSession("customer@chakuchuri.pk")
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(accounts, nil, t.TempDir())
	file, header := testMultipartFile(t, "private.csv", []byte("sku,quantity\nprivate,1\n"))
	record, err := service.Create(file, header, UploadRequest{OwnerType: "message", CustomerID: "another_customer"}, owner.User)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	Register(mux, service)
	request := func(token, etag string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, "/api/files/"+record.ID+"/content", nil)
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		if etag != "" {
			r.Header.Set("If-None-Match", etag)
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w
	}
	first := request(owner.Token, "")
	if first.Code != 200 {
		t.Fatalf("owner response: %d %s", first.Code, first.Body.String())
	}
	for _, token := range []string{"", "expired-token", customer.Token} {
		w := request(token, first.Header().Get("ETag"))
		if w.Code != 401 && w.Code != 404 {
			t.Fatalf("unauthorized conditional request: %d", w.Code)
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("authorization error was cacheable")
		}
	}
}
