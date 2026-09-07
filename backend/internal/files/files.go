package files

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"chakuchuri/backend/internal/audit"
	"chakuchuri/backend/internal/auth"
	"chakuchuri/backend/internal/platform/httpx"
	"github.com/disintegration/imaging"
)

const maxUploadBytes = 10 << 20
const maxQuotationImageBytes = 2 << 20

var errInvalidFile = errors.New("invalid file")
var errQuotationImageTooLarge = errors.New("quotation image too large")

func isQuotationImageOwner(ownerType string) bool {
	switch strings.ToLower(strings.TrimSpace(ownerType)) {
	case "quotation_photo", "quotation_reference":
		return true
	default:
		return false
	}
}

type FileRecord struct {
	ID           string `json:"id"`
	TenantID     string `json:"tenantId"`
	CustomerID   string `json:"customerId,omitempty"`
	OwnerType    string `json:"ownerType"`
	OwnerID      string `json:"ownerId,omitempty"`
	OriginalName string `json:"originalName"`
	StorageKey   string `json:"storageKey"`
	URL          string `json:"url"`
	ThumbnailKey string `json:"thumbnailKey,omitempty"`
	ThumbnailURL string `json:"thumbnailUrl,omitempty"`
	MimeType     string `json:"mimeType"`
	ByteSize     int64  `json:"byteSize"`
	SourceBytes  int64  `json:"sourceBytes,omitempty"`
	PreviewBytes int64  `json:"previewBytes,omitempty"`
	Width        int    `json:"width,omitempty"`
	Height       int    `json:"height,omitempty"`
	Checksum     string `json:"checksum"`
	CreatedAt    string `json:"createdAt"`
}

type UploadRequest struct {
	CustomerID string
	OwnerType  string
	OwnerID    string
}

type Service struct {
	mu                     sync.RWMutex
	authService            *auth.Service
	recorder               *audit.Recorder
	repository             Repository
	storageDir             string
	records                []FileRecord
	byID                   map[string]FileRecord
	customerDocumentAccess func(user auth.User, fileID string) bool
}

var errFileNotFound = errors.New("file not found")

func NewService(authService *auth.Service, recorder *audit.Recorder, storageDir string) *Service {
	if strings.TrimSpace(storageDir) == "" {
		storageDir = "storage"
	}
	return &Service{
		authService: authService,
		recorder:    recorder,
		storageDir:  storageDir,
		records:     []FileRecord{},
		byID:        map[string]FileRecord{},
	}
}

func (s *Service) SetCustomerDocumentAccess(fn func(user auth.User, fileID string) bool) {
	s.customerDocumentAccess = fn
}

func (s *Service) Create(file multipart.File, header *multipart.FileHeader, payload UploadRequest, actor auth.User) (FileRecord, error) {
	defer file.Close()

	raw, err := io.ReadAll(io.LimitReader(file, maxUploadBytes+1))
	if err != nil {
		return FileRecord{}, fmt.Errorf("read upload: %w", err)
	}
	if len(raw) == 0 || len(raw) > maxUploadBytes {
		return FileRecord{}, errInvalidFile
	}

	mimeType := http.DetectContentType(raw)
	ownerType := firstNonEmpty(payload.OwnerType, "general")
	if isQuotationImageOwner(ownerType) && len(raw) > maxQuotationImageBytes {
		return FileRecord{}, errQuotationImageTooLarge
	}
	if strings.EqualFold(ownerType, "customer_document") {
		if !allowedCustomerDocumentMIME(mimeType, header.Filename) {
			return FileRecord{}, errInvalidFile
		}
		if !auth.IsCustomerRole(actor) && !auth.HasPermission(actor, "kyc.review") {
			return FileRecord{}, auth.ErrForbidden
		}
	} else if !allowedMIME(mimeType, header.Filename) {
		return FileRecord{}, errInvalidFile
	}

	customerID := firstNonEmpty(payload.CustomerID, actor.CustomerID)
	if strings.EqualFold(ownerType, "rate_sheet") && !auth.HasPermission(actor, "shipping.manage") {
		return FileRecord{}, auth.ErrForbidden
	}
	if customerID != "" && !auth.CanAccessCustomer(actor, customerID) {
		return FileRecord{}, auth.ErrForbidden
	}

	return s.storeUpload(raw, header.Filename, mimeType, UploadRequest{
		CustomerID: customerID,
		OwnerType:  ownerType,
		OwnerID:    payload.OwnerID,
	}, firstNonEmpty(actor.TenantID, "tenant_chakuchuri"), actor.Email)
}

// CreateTrusted stores an upload for a known customer without a signed-in actor
// (used by public document request links).
func (s *Service) CreateTrusted(file multipart.File, header *multipart.FileHeader, payload UploadRequest, actorLabel, tenantHint string) (FileRecord, error) {
	defer file.Close()

	raw, err := io.ReadAll(io.LimitReader(file, maxUploadBytes+1))
	if err != nil {
		return FileRecord{}, fmt.Errorf("read upload: %w", err)
	}
	if len(raw) == 0 || len(raw) > maxUploadBytes {
		return FileRecord{}, errInvalidFile
	}
	mimeType := http.DetectContentType(raw)
	ownerType := firstNonEmpty(payload.OwnerType, "customer_document")
	if !strings.EqualFold(ownerType, "customer_document") || !allowedCustomerDocumentMIME(mimeType, header.Filename) {
		return FileRecord{}, errInvalidFile
	}
	customerID := strings.TrimSpace(payload.CustomerID)
	if customerID == "" {
		return FileRecord{}, errInvalidFile
	}
	return s.storeUpload(raw, header.Filename, mimeType, UploadRequest{
		CustomerID: customerID,
		OwnerType:  "customer_document",
		OwnerID:    firstNonEmpty(payload.OwnerID, customerID),
	}, firstNonEmpty(tenantHint, "tenant_chakuchuri"), firstNonEmpty(actorLabel, "public-link"))
}

func (s *Service) storeUpload(raw []byte, filename, mimeType string, payload UploadRequest, tenantID, actorEmail string) (FileRecord, error) {
	sourceBytes := int64(len(raw))
	raw, filename, mimeType = optimizePhoto(raw, filename, mimeType, payload.OwnerType)
	now := time.Now().UTC()
	record := FileRecord{
		ID:           newFileID(now),
		TenantID:     firstNonEmpty(tenantID, "tenant_chakuchuri"),
		CustomerID:   strings.TrimSpace(payload.CustomerID),
		OwnerType:    firstNonEmpty(payload.OwnerType, "general"),
		OwnerID:      strings.TrimSpace(payload.OwnerID),
		OriginalName: safeFilename(filename),
		MimeType:     mimeType,
		ByteSize:     int64(len(raw)),
		SourceBytes:  sourceBytes,
		CreatedAt:    now.Format(time.RFC3339),
	}
	if record.OriginalName == "" {
		record.OriginalName = record.ID
	}
	sum := sha256.Sum256(raw)
	record.Checksum = hex.EncodeToString(sum[:])
	record.StorageKey = filepath.ToSlash(filepath.Join(now.Format("2006"), now.Format("01"), record.ID+"_"+record.OriginalName))
	record.URL = "/api/files/" + record.ID + "/content"

	if width, height := imageSize(raw); width > 0 && height > 0 {
		record.Width = width
		record.Height = height
	}

	if err := s.writeStorageFile(record.StorageKey, raw); err != nil {
		return FileRecord{}, err
	}

	if thumb, ok := storageThumbnail(raw, mimeType, sourceBytes); ok {
		record.ThumbnailKey = filepath.ToSlash(filepath.Join(now.Format("2006"), now.Format("01"), record.ID+"_thumb.jpg"))
		record.ThumbnailURL = "/api/files/" + record.ID + "/thumbnail"
		record.PreviewBytes = int64(len(thumb))
		if err := s.writeStorageFile(record.ThumbnailKey, thumb); err != nil {
			s.removeStoredFiles(record)
			return FileRecord{}, err
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	_, pointReads := s.repository.(recordReader)
	if !pointReads {
		s.records = append([]FileRecord{record}, s.records...)
		s.byID[record.ID] = record
	}
	if err := s.persistUploadLocked(record); err != nil {
		if !pointReads {
			delete(s.byID, record.ID)
			s.records = s.records[1:]
		}
		s.removeStoredFiles(record)
		return FileRecord{}, fmt.Errorf("persist upload: %w", err)
	}

	if s.recorder != nil {
		s.recorder.Record(actorEmail, "files.uploaded", record.ID, "File uploaded and stored.")
	}
	return record, nil
}

func newFileID(now time.Time) string {
	var entropy [6]byte
	if _, err := rand.Read(entropy[:]); err == nil {
		return fmt.Sprintf("file_%d_%s", now.UnixNano(), hex.EncodeToString(entropy[:]))
	}
	return fmt.Sprintf("file_%d", now.UnixNano())
}

func (s *Service) Get(id string) (FileRecord, bool) {
	return s.getForTenant(id, defaultTenantExternalID)
}

func (s *Service) getForTenant(id, tenantID string) (FileRecord, bool) {
	s.mu.RLock()
	repository := s.repository
	record, ok := s.byID[strings.TrimSpace(id)]
	s.mu.RUnlock()
	if reader, pointReads := repository.(recordReader); pointReads {
		var err error
		record, ok, err = reader.GetFile(tenantID, strings.TrimSpace(id))
		if err != nil {
			s.recordSystem("files.persistence_error", "Could not retrieve file metadata: "+err.Error())
			return FileRecord{}, false
		}
	}
	return record, ok
}

func (s *Service) IsCustomerDocument(id, customerID string) bool {
	record, ok := s.Get(id)
	if !ok || !strings.EqualFold(record.OwnerType, "customer_document") {
		return false
	}
	customerID = strings.TrimSpace(customerID)
	return customerID != "" && record.CustomerID == customerID && record.OwnerID == customerID
}

// DiscardCustomerDocument removes a just-uploaded KYC file when attachment validation fails.
func (s *Service) DiscardCustomerDocument(id, customerID string) bool {
	id = strings.TrimSpace(id)
	customerID = strings.TrimSpace(customerID)
	record, ok := s.Get(id)
	s.mu.Lock()
	defer s.mu.Unlock()
	if !ok || !strings.EqualFold(record.OwnerType, "customer_document") || record.CustomerID != customerID || record.OwnerID != customerID {
		return false
	}
	if s.repository != nil {
		if err := s.repository.DeleteFile(record); err != nil {
			s.recordSystem("files.persistence_error", "Could not discard file record: "+err.Error())
			return false
		}
	}
	delete(s.byID, id)
	next := make([]FileRecord, 0, max(0, len(s.records)-1))
	for _, row := range s.records {
		if row.ID != id {
			next = append(next, row)
		}
	}
	s.records = next
	s.removeStoredFiles(record)
	return true
}

func (s *Service) removeStoredFiles(record FileRecord) {
	if path, err := s.storagePath(record.StorageKey); err == nil {
		_ = os.Remove(path)
	}
	if record.ThumbnailKey != "" {
		if path, err := s.storagePath(record.ThumbnailKey); err == nil {
			_ = os.Remove(path)
		}
	}
}

func (s *Service) GetForUser(id string, user auth.User) (FileRecord, bool) {
	record, ok := s.getForTenant(id, firstNonEmpty(user.TenantID, defaultTenantExternalID))
	if !ok {
		return FileRecord{}, false
	}
	if record.TenantID != "" && user.TenantID != "" && record.TenantID != user.TenantID {
		return FileRecord{}, false
	}
	if strings.EqualFold(record.OwnerType, "rate_sheet") && !auth.HasPermission(user, "shipping.manage") {
		return FileRecord{}, false
	}
	// Directory listing photos are meant for the shared trade directory once uploaded.
	if strings.EqualFold(record.OwnerType, "directory_listing") {
		return record, true
	}
	// All KYC media goes through the dedicated reviewer/customer access policy.
	if strings.EqualFold(record.OwnerType, "customer_document") {
		if s.customerDocumentAccess == nil || !s.customerDocumentAccess(user, record.ID) {
			return FileRecord{}, false
		}
	}
	if record.CustomerID != "" && !auth.CanAccessCustomer(user, record.CustomerID) {
		return FileRecord{}, false
	}
	return record, true
}

func (s *Service) Read(id string) (FileRecord, []byte, error) {
	record, ok := s.Get(id)
	if !ok {
		return FileRecord{}, nil, errFileNotFound
	}
	path, err := s.storagePath(record.StorageKey)
	if err != nil {
		return FileRecord{}, nil, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return FileRecord{}, nil, fmt.Errorf("read file: %w", err)
	}
	return record, raw, nil
}

func (s *Service) Serve(w http.ResponseWriter, r *http.Request, id string, thumbnail bool) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Add("Vary", "Authorization")
	record, ok := s.Get(id)
	if !ok {
		httpx.Error(w, r, http.StatusNotFound, "file_not_found", "File was not found.")
		return
	}
	s.serveRecord(w, r, record, thumbnail)
}

func (s *Service) serveRecord(w http.ResponseWriter, r *http.Request, record FileRecord, thumbnail bool) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Vary", "Authorization")

	key := record.StorageKey
	if thumbnail && record.ThumbnailKey != "" {
		key = record.ThumbnailKey
	}
	path, err := s.storagePath(key)
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "invalid_file_path", "File path is not valid.")
		return
	}
	if !strings.EqualFold(record.OwnerType, "customer_document") {
		if info, statErr := os.Stat(path); statErr == nil && !info.IsDir() {
			// Revalidate with authorization on every reuse; never allow shared caching.
			digest := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%d", key, info.Size(), info.ModTime().UnixNano())))
			w.Header().Set("ETag", fmt.Sprintf("W/%q", hex.EncodeToString(digest[:])))
			w.Header().Set("Cache-Control", "private, no-cache")
		}
	}
	http.ServeFile(w, r, path)
}

func Register(mux *http.ServeMux, service *Service) {
	mux.HandleFunc("/api/files", func(w http.ResponseWriter, r *http.Request) {
		if !httpx.RequireMethod(w, r, http.MethodPost) {
			return
		}
		user, ok := service.authService.UserFromRequest(r)
		if !ok {
			httpx.Error(w, r, http.StatusUnauthorized, "not_authenticated", "Sign in is required.")
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes+1024)
		if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
			httpx.Error(w, r, http.StatusBadRequest, "invalid_upload", "Upload could not be read.")
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			httpx.Error(w, r, http.StatusBadRequest, "file_required", "Choose a file to upload.")
			return
		}

		ownerType := r.FormValue("ownerType")
		if strings.EqualFold(ownerType, "customer_document") {
			httpx.Error(w, r, http.StatusBadRequest, "use_kyc_upload", "Use the protected customer document upload workflow.")
			return
		}
		record, err := service.Create(file, header, UploadRequest{
			CustomerID: r.FormValue("customerId"),
			OwnerType:  ownerType,
			OwnerID:    r.FormValue("ownerId"),
		}, user)
		if err != nil {
			if errors.Is(err, auth.ErrForbidden) {
				httpx.Error(w, r, http.StatusForbidden, "forbidden", "You do not have access to upload this file.")
				return
			}
			if errors.Is(err, errQuotationImageTooLarge) {
				httpx.Error(w, r, http.StatusBadRequest, "file_too_large", "Quotation photos must be 2 MB or smaller — about a normal phone screenshot. Compress or choose a smaller image.")
				return
			}
			httpx.Error(w, r, http.StatusBadRequest, "invalid_file", "File type or size is not allowed.")
			return
		}
		httpx.Write(w, r, http.StatusOK, record)
	})

	mux.HandleFunc("/api/files/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Add("Vary", "Authorization")
		user, ok := service.authService.UserFromRequest(r)
		if !ok {
			httpx.Error(w, r, http.StatusUnauthorized, "not_authenticated", "Sign in is required.")
			return
		}

		path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/files/"), "/")
		parts := strings.Split(path, "/")
		if len(parts) == 0 || parts[0] == "" {
			httpx.Error(w, r, http.StatusNotFound, "file_not_found", "File was not found.")
			return
		}
		record, found := service.GetForUser(parts[0], user)
		if !found {
			httpx.Error(w, r, http.StatusNotFound, "file_not_found", "File was not found.")
			return
		}

		if len(parts) == 1 {
			httpx.Write(w, r, http.StatusOK, record)
			return
		}
		if parts[1] == "content" {
			service.serveRecord(w, r, record, false)
			return
		}
		if parts[1] == "thumbnail" {
			service.serveRecord(w, r, record, true)
			return
		}
		httpx.Error(w, r, http.StatusNotFound, "file_not_found", "File route was not found.")
	})
}

func (s *Service) writeStorageFile(key string, data []byte) error {
	path, err := s.storagePath(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create storage directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write storage file: %w", err)
	}
	return nil
}

func (s *Service) storagePath(key string) (string, error) {
	root := filepath.Clean(filepath.Join(s.storageDir, "uploads"))
	path := filepath.Clean(filepath.Join(root, filepath.FromSlash(key)))
	if path != root && !strings.HasPrefix(path, root+string(os.PathSeparator)) {
		return "", fmt.Errorf("unsafe storage key: %s", key)
	}
	return path, nil
}

func allowedCustomerDocumentMIME(mimeType string, filename string) bool {
	extension := strings.ToLower(filepath.Ext(strings.TrimSpace(filename)))
	switch mimeType {
	case "image/jpeg":
		return extension == ".jpg" || extension == ".jpeg"
	case "image/png":
		return extension == ".png"
	case "image/webp":
		return extension == ".webp"
	case "application/pdf":
		return extension == ".pdf"
	default:
		return false
	}
}

func allowedMIME(mimeType string, filename string) bool {
	switch mimeType {
	case "image/jpeg", "image/png", "image/webp", "application/pdf", "text/plain", "text/csv",
		"application/msword", "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"application/vnd.ms-powerpoint", "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		"application/zip":
		return true
	}
	extension := strings.ToLower(filepath.Ext(filename))
	return extension == ".xlsx" || extension == ".xls" || extension == ".csv" ||
		extension == ".doc" || extension == ".docx" || extension == ".ppt" || extension == ".pptx" || extension == ".zip"
}

func imageSize(raw []byte) (int, int) {
	config, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return 0, 0
	}
	return config.Width, config.Height
}

func thumbnail(raw []byte, mimeType string) ([]byte, bool) {
	if mimeType != "image/jpeg" && mimeType != "image/png" && mimeType != "image/webp" {
		return nil, false
	}
	if !boundedImage(raw) {
		return nil, false
	}
	select {
	case imageProcessingSlots <- struct{}{}:
		defer func() { <-imageProcessingSlots }()
	default:
		return nil, false
	}
	img, err := imaging.Decode(bytes.NewReader(raw), imaging.AutoOrientation(true))
	if err != nil {
		return nil, false
	}
	resized := imaging.Fit(img, 360, 360, imaging.Lanczos)
	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, resized, &jpeg.Options{Quality: 74}); err != nil {
		return nil, false
	}
	if buffer.Len() >= len(raw) {
		return nil, false
	}
	return buffer.Bytes(), true
}

func safeFilename(value string) string {
	value = filepath.Base(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "-")
	builder := strings.Builder{}
	for _, char := range value {
		if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '.' || char == '-' || char == '_' {
			builder.WriteRune(char)
		}
	}
	return strings.Trim(builder.String(), ".-_")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
