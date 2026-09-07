package ratesheets

import (
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"chakuchuri/backend/internal/auth"
	fileuploads "chakuchuri/backend/internal/files"
	"chakuchuri/backend/internal/platform/httpx"
)

const maxCatalogBytes = 12 << 20

func Register(mux *http.ServeMux, service *Service, authService *auth.Service, fileService *fileuploads.Service) {
	mux.HandleFunc("/api/shipping-rates", func(w http.ResponseWriter, r *http.Request) {
		if !httpx.RequireMethod(w, r, http.MethodGet) {
			return
		}
		if _, ok := requireRateUser(w, r, authService); !ok {
			return
		}
		httpx.Write(w, r, http.StatusOK, paginateRateSnapshot(r, service.Snapshot()))
	})

	mux.HandleFunc("/api/shipping-rates/preview", func(w http.ResponseWriter, r *http.Request) {
		if !httpx.RequireMethod(w, r, http.MethodPost) {
			return
		}
		if _, ok := requireRateManager(w, r, authService); !ok {
			return
		}
		file, header, raw, ok := readRateUpload(w, r)
		if !ok {
			return
		}
		defer file.Close()
		book, err := ParseCatalog(raw, header.Filename, parseOptions(r))
		if err != nil {
			writeRateError(w, r, err)
			return
		}
		httpx.Write(w, r, http.StatusOK, PreviewBook(book))
	})

	mux.HandleFunc("/api/shipping-rates/import", func(w http.ResponseWriter, r *http.Request) {
		if !httpx.RequireMethod(w, r, http.MethodPost) {
			return
		}
		user, ok := requireRateManager(w, r, authService)
		if !ok {
			return
		}
		file, header, raw, ok := readRateUpload(w, r)
		if !ok {
			return
		}
		defer file.Close()
		book, err := ParseCatalog(raw, header.Filename, parseOptions(r))
		if err != nil {
			writeRateError(w, r, err)
			return
		}
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			httpx.Error(w, r, http.StatusBadRequest, "invalid_upload", "The uploaded workbook could not be prepared for storage.")
			return
		}
		record, err := fileService.Create(file, header, fileuploads.UploadRequest{OwnerType: "shipping_rate_book"}, user)
		if err != nil {
			httpx.Error(w, r, http.StatusBadRequest, "invalid_file", "Only XLSX rate sheets up to 12 MB are allowed.")
			return
		}
		activate := !strings.EqualFold(strings.TrimSpace(r.FormValue("activate")), "false")
		summary, err := service.Import(book, record.ID, record.OriginalName, activate, user)
		if err != nil {
			writeRateError(w, r, err)
			return
		}
		httpx.Write(w, r, http.StatusOK, map[string]interface{}{
			"book": summary, "source": record, "snapshot": paginateRateSnapshot(r, service.Snapshot()),
		})
	})

	mux.HandleFunc("/api/shipping-rates/lookup", func(w http.ResponseWriter, r *http.Request) {
		if !httpx.RequireMethod(w, r, http.MethodPost) {
			return
		}
		if _, ok := requireRateUser(w, r, authService); !ok {
			return
		}
		var request LookupRequest
		if !httpx.DecodeJSON(w, r, &request) {
			return
		}
		result, err := service.Lookup(request)
		if err != nil {
			writeRateError(w, r, err)
			return
		}
		httpx.Write(w, r, http.StatusOK, result)
	})

	mux.HandleFunc("/api/shipping-rates/book", func(w http.ResponseWriter, r *http.Request) {
		if !httpx.RequireMethod(w, r, http.MethodPost) {
			return
		}
		user, ok := requireRateUser(w, r, authService)
		if !ok {
			return
		}
		if !auth.HasAnyPermission(user, "shipping.create", "shipping.manage") {
			httpx.Error(w, r, http.StatusForbidden, "forbidden", "You do not have access to create shipments.")
			return
		}
		var request BookingRequest
		if !httpx.DecodeJSON(w, r, &request) {
			return
		}
		result, err := service.Book(request, user)
		if err != nil {
			writeRateError(w, r, err)
			return
		}
		httpx.Write(w, r, http.StatusOK, result)
	})

	mux.HandleFunc("/api/shipping-rates/books/", func(w http.ResponseWriter, r *http.Request) {
		if !httpx.RequireMethod(w, r, http.MethodPost) {
			return
		}
		user, ok := requireRateManager(w, r, authService)
		if !ok {
			return
		}
		path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/shipping-rates/books/"), "/")
		if !strings.HasSuffix(path, "/status") {
			httpx.Error(w, r, http.StatusNotFound, "not_found", "Shipping rate action was not found.")
			return
		}
		id := strings.Trim(strings.TrimSuffix(path, "/status"), "/")
		var payload struct {
			Active bool `json:"active"`
		}
		if id == "" || !httpx.DecodeJSON(w, r, &payload) {
			return
		}
		result, err := service.SetActive(id, payload.Active, user)
		if err != nil {
			writeRateError(w, r, err)
			return
		}
		httpx.Write(w, r, http.StatusOK, paginateRateSnapshot(r, result))
	})
}

func paginateRateSnapshot(r *http.Request, snapshot Snapshot) Snapshot {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if offset < 0 {
		offset = 0
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset > len(snapshot.Books) {
		offset = len(snapshot.Books)
	}
	end := offset + limit
	if end > len(snapshot.Books) {
		end = len(snapshot.Books)
	}
	snapshot.Pagination = PageInfo{Loaded: end, Total: len(snapshot.Books), HasMore: end < len(snapshot.Books)}
	snapshot.Books = append([]BookSummary(nil), snapshot.Books[offset:end]...)
	return snapshot
}

func readRateUpload(w http.ResponseWriter, r *http.Request) (multipart.File, *multipart.FileHeader, []byte, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxCatalogBytes+4096)
	if err := r.ParseMultipartForm(maxCatalogBytes); err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_upload", "Rate sheet upload could not be read or exceeded 12 MB.")
		return nil, nil, nil, false
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "file_required", "Choose an XLSX rate sheet to continue.")
		return nil, nil, nil, false
	}
	limited := io.LimitReader(file, maxCatalogBytes+1)
	raw, err := io.ReadAll(limited)
	if err != nil || len(raw) == 0 || len(raw) > maxCatalogBytes {
		file.Close()
		httpx.Error(w, r, http.StatusBadRequest, "invalid_upload", "Rate sheet must be a non-empty XLSX file up to 12 MB.")
		return nil, nil, nil, false
	}
	return file, header, raw, true
}

func parseOptions(r *http.Request) ParseOptions {
	return ParseOptions{
		Carrier:  strings.TrimSpace(r.FormValue("carrier")),
		Name:     strings.TrimSpace(r.FormValue("name")),
		Currency: strings.ToUpper(strings.TrimSpace(r.FormValue("currency"))),
	}
}

func requireRateUser(w http.ResponseWriter, r *http.Request, authService *auth.Service) (auth.User, bool) {
	user, ok := authService.UserFromRequest(r)
	if !ok {
		httpx.Error(w, r, http.StatusUnauthorized, "not_authenticated", "Sign in is required.")
		return auth.User{}, false
	}
	return user, true
}

func requireRateManager(w http.ResponseWriter, r *http.Request, authService *auth.Service) (auth.User, bool) {
	user, ok := requireRateUser(w, r, authService)
	if !ok {
		return auth.User{}, false
	}
	if !auth.HasPermission(user, "shipping.manage") {
		httpx.Error(w, r, http.StatusForbidden, "forbidden", "You do not have access to manage shipping rate sheets.")
		return auth.User{}, false
	}
	return user, true
}

func writeRateError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, errBookNotFound) {
		httpx.Error(w, r, http.StatusNotFound, "rate_book_not_found", "Shipping rate book was not found.")
		return
	}
	message := strings.TrimSpace(err.Error())
	message = strings.TrimPrefix(message, errInvalidLookup.Error()+": ")
	if errors.Is(err, ErrNoRates) {
		message = "The workbook needs service rate tables and a three-digit ZIP-to-zone section."
	}
	if message == "" || message == errInvalidLookup.Error() {
		message = "Check the shipping details and try again."
	}
	if len(message) > 260 {
		message = message[:260]
	}
	status := http.StatusBadRequest
	if errors.Is(err, errInvalidLookup) {
		status = http.StatusUnprocessableEntity
	}
	httpx.Error(w, r, status, "shipping_rate_validation_failed", message)
}

func parseBool(value string, fallback bool) bool {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func rateUploadDebug(header *multipart.FileHeader) string {
	if header == nil {
		return "rate sheet"
	}
	return fmt.Sprintf("%s (%d bytes)", header.Filename, header.Size)
}
