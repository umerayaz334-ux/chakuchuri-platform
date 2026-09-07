package customers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"chakuchuri/backend/internal/audit"
	"chakuchuri/backend/internal/auth"
	fileuploads "chakuchuri/backend/internal/files"
	"chakuchuri/backend/internal/platform/httpx"
)

type Customer struct {
	ID                     string              `json:"id"`
	TenantID               string              `json:"tenantId"`
	CompanyName            string              `json:"companyName"`
	ContactName            string              `json:"contactName"`
	Email                  string              `json:"email"`
	Phone                  string              `json:"phone"`
	Country                string              `json:"country"`
	Services               []string            `json:"services"`
	Online                 bool                `json:"online"`
	LastOnline             string              `json:"lastOnline"`
	BalanceDue             string              `json:"balanceDue"`
	OpenOrders             int                 `json:"openOrders"`
	OpenShipments          int                 `json:"openShipments"`
	VerificationStatus     string              `json:"verificationStatus"`
	VerificationStage      string              `json:"verificationStage"`
	VerifiedAt             string              `json:"verifiedAt,omitempty"`
	VerifiedByUserID       string              `json:"verifiedByUserId,omitempty"`
	IdentityNote           string              `json:"identityNote,omitempty"`
	CNIC                   string              `json:"cnic,omitempty"`
	Documents              []CustomerDocument  `json:"documents,omitempty"`
	RequestedDocuments     []RequestedDocument `json:"requestedDocuments,omitempty"`
	DocumentsSubmittedAt   string              `json:"documentsSubmittedAt,omitempty"`
	VerificationInviteOpen bool                `json:"verificationInviteOpen"`
}

type CreateRequest struct {
	ID          string   `json:"id"`
	TenantID    string   `json:"tenantId"`
	CompanyName string   `json:"companyName"`
	ContactName string   `json:"contactName"`
	Email       string   `json:"email"`
	Phone       string   `json:"phone"`
	Country     string   `json:"country"`
	Services    []string `json:"services"`
}

type Service struct {
	mu                    sync.RWMutex
	recorder              *audit.Recorder
	repository            Repository
	changeNotifier        func(string)
	documentFileValidator func(fileID, customerID string) bool
	customers             []Customer
	documentRequests      []DocumentRequest
	portalURL             string
}

func (s *Service) SetPortalURL(portalURL string) {
	s.portalURL = strings.TrimRight(strings.TrimSpace(portalURL), "/")
}

func (s *Service) SetChangeNotifier(notifier func(string)) {
	s.changeNotifier = notifier
}

func (s *Service) SetDocumentFileValidator(validator func(fileID, customerID string) bool) {
	s.documentFileValidator = validator
}

func (s *Service) notifyChange(scope string) {
	if s.changeNotifier != nil {
		s.changeNotifier(scope)
	}
}

func NewService(recorder *audit.Recorder) *Service {
	return &Service{
		recorder: recorder,
		customers: []Customer{
			{
				ID:            "cust_abc_export",
				TenantID:      "tenant_chakuchuri",
				CompanyName:   "ABC Export House",
				ContactName:   "Muhammad Zain",
				Email:         "customer@chakuchuri.pk",
				Country:       "Pakistan",
				Services:      []string{"Manufacturing", "Shipping", "Chat", "Calls"},
				Online:        true,
				LastOnline:    "Online now",
				BalanceDue:    "Rs 184,000",
				OpenOrders:    3,
				OpenShipments: 8,
			},
			{
				ID:            "cust_northern_trading",
				TenantID:      "tenant_chakuchuri",
				CompanyName:   "Northern Trading Co.",
				ContactName:   "Ali Khan",
				Country:       "Pakistan",
				Services:      []string{"Manufacturing", "Shipping"},
				Online:        false,
				LastOnline:    "4 minutes ago",
				BalanceDue:    "Rs 32,500",
				OpenOrders:    1,
				OpenShipments: 2,
			},
		},
	}
}

func (s *Service) List() []Customer {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows := make([]Customer, len(s.customers))
	copy(rows, s.customers)
	return rows
}

func (s *Service) ListForUser(user auth.User) []Customer {
	s.mu.RLock()
	defer s.mu.RUnlock()

	all, allowed := auth.CustomerScope(user)
	rows := make([]Customer, 0, len(s.customers))
	for _, customer := range s.customers {
		if all || allowed[customer.ID] {
			rows = append(rows, customer)
		}
	}
	return rows
}

func (s *Service) Create(payload CreateRequest) Customer {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.persistLocked()

	services := payload.Services
	if len(services) == 0 {
		services = []string{"Manufacturing", "Shipping", "Chat", "Calls"}
	}
	id := payload.ID
	if id == "" {
		id = "cust_" + safeID(payload.CompanyName) + "_" + time.Now().UTC().Format("150405")
	}
	customer := Customer{
		ID:                 id,
		TenantID:           valueOr(payload.TenantID, "tenant_chakuchuri"),
		CompanyName:        strings.TrimSpace(payload.CompanyName),
		ContactName:        strings.TrimSpace(payload.ContactName),
		Email:              strings.ToLower(strings.TrimSpace(payload.Email)),
		Phone:              strings.TrimSpace(payload.Phone),
		Country:            valueOr(strings.TrimSpace(payload.Country), "Pakistan"),
		Services:           services,
		Online:             false,
		LastOnline:         "Never",
		BalanceDue:         "Rs 0",
		OpenOrders:         0,
		OpenShipments:      0,
		VerificationStatus: "unverified",
		Documents:          []CustomerDocument{},
	}
	s.customers = append([]Customer{customer}, s.customers...)
	if s.recorder != nil {
		s.recorder.Record(customer.Email, "customers.created", "customer", "Customer account created.")
	}
	s.notifyChange("customers")
	return customer
}
func (s *Service) EnsurePortalCustomer(user auth.User) Customer {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.persistLocked()

	id := valueOr(strings.TrimSpace(user.CustomerID), "cust_"+safeID(user.Name))
	email := strings.ToLower(strings.TrimSpace(user.Email))
	for index := range s.customers {
		matchesID := id != "" && s.customers[index].ID == id
		matchesEmail := email != "" && strings.EqualFold(s.customers[index].Email, email)
		if !matchesID && !matchesEmail {
			continue
		}
		if s.customers[index].ID == "" {
			s.customers[index].ID = id
		}
		if s.customers[index].TenantID == "" {
			s.customers[index].TenantID = valueOr(user.TenantID, "tenant_chakuchuri")
		}
		if strings.TrimSpace(s.customers[index].CompanyName) == "" {
			s.customers[index].CompanyName = valueOr(user.Name, "Customer")
		}
		if strings.TrimSpace(s.customers[index].ContactName) == "" {
			s.customers[index].ContactName = user.Name
		}
		if s.customers[index].Email == "" {
			s.customers[index].Email = email
		}
		if len(s.customers[index].Services) == 0 {
			s.customers[index].Services = []string{"Manufacturing", "Shipping", "Chat", "Calls"}
		}
		return s.customers[index]
	}

	customer := Customer{
		ID:                 id,
		TenantID:           valueOr(user.TenantID, "tenant_chakuchuri"),
		CompanyName:        valueOr(user.Name, "Customer"),
		ContactName:        user.Name,
		Email:              email,
		Country:            "Pakistan",
		Services:           []string{"Manufacturing", "Shipping", "Chat", "Calls"},
		Online:             false,
		LastOnline:         "Not tracked yet",
		BalanceDue:         "Rs 0",
		OpenOrders:         0,
		OpenShipments:      0,
		VerificationStatus: "unverified",
		Documents:          []CustomerDocument{},
	}
	s.customers = append([]Customer{customer}, s.customers...)
	if s.recorder != nil {
		s.recorder.Record(email, "customers.portal_created", customer.ID, "Customer portal account created from user access.")
	}
	s.notifyChange("customers")
	return customer
}

func valueOr(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func safeID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	builder := strings.Builder{}
	for _, char := range value {
		if char >= 'a' && char <= 'z' || char >= '0' && char <= '9' {
			builder.WriteRune(char)
		} else if char == ' ' || char == '-' || char == '_' {
			builder.WriteRune('_')
		}
	}
	cleaned := strings.Trim(builder.String(), "_")
	if cleaned == "" {
		return "new"
	}
	return cleaned
}

func Register(mux *http.ServeMux, service *Service, authService *auth.Service, fileService *fileuploads.Service) {
	mux.HandleFunc("/api/customers", func(w http.ResponseWriter, r *http.Request) {
		if !httpx.RequireMethod(w, r, http.MethodGet) {
			return
		}
		user, ok := authService.UserFromRequest(r)
		if !ok {
			httpx.Error(w, r, http.StatusUnauthorized, "not_authenticated", "Sign in is required.")
			return
		}
		rows := service.ListForUser(user)
		total := len(rows)
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
		if offset > total {
			offset = total
		}
		end := offset + limit
		if end > total {
			end = total
		}
		rows = rows[offset:end]
		for i := range rows {
			rows[i] = customerForActor(rows[i], user)
			online, lastOnline := authService.PresenceByCustomerID(rows[i].ID)
			rows[i].Online = online
			if lastOnline != "" {
				rows[i].LastOnline = lastOnline
			} else if rows[i].LastOnline == "" || rows[i].LastOnline == "Online now" || rows[i].LastOnline == "4 minutes ago" {
				rows[i].LastOnline = "Never"
			}
		}
		httpx.Write(w, r, http.StatusOK, map[string]interface{}{
			"customers":  rows,
			"pagination": map[string]interface{}{"loaded": end, "total": total, "hasMore": end < total},
		})
	})

	mux.HandleFunc("/api/customers/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/customers/"), "/")
		parts := strings.Split(path, "/")
		if len(parts) == 0 || parts[0] == "" {
			httpx.Error(w, r, http.StatusNotFound, "not_found", "Customer route was not found.")
			return
		}
		customerID := parts[0]

		if len(parts) == 3 && parts[1] == "documents" && parts[2] == "upload" && r.Method == http.MethodPost {
			user, ok := authService.UserFromRequest(r)
			if !ok {
				httpx.Error(w, r, http.StatusUnauthorized, "not_authenticated", "Sign in is required.")
				return
			}
			if fileService == nil {
				httpx.Error(w, r, http.StatusServiceUnavailable, "uploads_unavailable", "File uploads are unavailable.")
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, 10<<20+1024)
			if err := r.ParseMultipartForm(10 << 20); err != nil {
				httpx.Error(w, r, http.StatusBadRequest, "invalid_upload", "Upload could not be read.")
				return
			}
			kind := r.FormValue("kind")
			label := r.FormValue("label")
			requestID := r.FormValue("requestId")
			if err := service.ValidateDocumentUpload(customerID, kind, requestID, user); err != nil {
				writeCustomerErr(w, r, err)
				return
			}
			file, header, err := r.FormFile("file")
			if err != nil {
				httpx.Error(w, r, http.StatusBadRequest, "file_required", "Choose a file to upload.")
				return
			}
			record, err := fileService.Create(file, header, fileuploads.UploadRequest{
				CustomerID: customerID,
				OwnerType:  "customer_document",
				OwnerID:    customerID,
			}, user)
			if err != nil {
				if errors.Is(err, auth.ErrForbidden) {
					httpx.Error(w, r, http.StatusForbidden, "forbidden", "You do not have access to upload KYC documents.")
					return
				}
				httpx.Error(w, r, http.StatusBadRequest, "invalid_file", "Use a JPEG, PNG, WebP or PDF file up to 10 MB.")
				return
			}
			customer, err := service.AttachDocument(customerID, AttachDocumentRequest{
				FileID:       record.ID,
				Kind:         kind,
				Label:        label,
				OriginalName: record.OriginalName,
				RequestID:    requestID,
			}, user)
			if err != nil {
				fileService.DiscardCustomerDocument(record.ID, customerID)
				writeCustomerErr(w, r, err)
				return
			}
			httpx.Write(w, r, http.StatusOK, map[string]any{"customer": customerForActor(customer, user), "file": record})
			return
		}

		if len(parts) == 2 && parts[1] == "documents" && r.Method == http.MethodPost {
			user, ok := authService.UserFromRequest(r)
			if !ok {
				httpx.Error(w, r, http.StatusUnauthorized, "not_authenticated", "Sign in is required.")
				return
			}
			var payload AttachDocumentRequest
			if !httpx.DecodeJSON(w, r, &payload) {
				return
			}
			customer, err := service.AttachDocument(customerID, payload, user)
			writeCustomerErr(w, r, err)
			if err != nil {
				return
			}
			httpx.Write(w, r, http.StatusOK, map[string]Customer{"customer": customerForActor(customer, user)})
			return
		}

		if len(parts) == 3 && parts[1] == "documents" && parts[2] == "submit" && r.Method == http.MethodPost {
			user, ok := authService.UserFromRequest(r)
			if !ok {
				httpx.Error(w, r, http.StatusUnauthorized, "not_authenticated", "Sign in is required.")
				return
			}
			customer, err := service.SubmitDocuments(customerID, user)
			writeCustomerErr(w, r, err)
			if err != nil {
				return
			}
			httpx.Write(w, r, http.StatusOK, map[string]Customer{"customer": customerForActor(customer, user)})
			return
		}

		if len(parts) == 4 && parts[1] == "documents" && parts[3] == "review" && r.Method == http.MethodPost {
			user, ok := authService.UserFromRequest(r)
			if !ok {
				httpx.Error(w, r, http.StatusUnauthorized, "not_authenticated", "Sign in is required.")
				return
			}
			var payload ReviewDocumentPayload
			if !httpx.DecodeJSON(w, r, &payload) {
				return
			}
			customer, err := service.ReviewDocument(customerID, parts[2], payload, user)
			writeCustomerErr(w, r, err)
			if err != nil {
				return
			}
			httpx.Write(w, r, http.StatusOK, map[string]Customer{"customer": customerForActor(customer, user)})
			return
		}

		if len(parts) == 2 && parts[1] == "verification-invite" && r.Method == http.MethodPost {
			user, ok := authService.UserFromRequest(r)
			if !ok {
				httpx.Error(w, r, http.StatusUnauthorized, "not_authenticated", "Sign in is required.")
				return
			}
			var payload OpenInvitePayload
			if !httpx.DecodeJSON(w, r, &payload) {
				return
			}
			customer, err := service.OpenVerificationInvite(customerID, payload, user)
			writeCustomerErr(w, r, err)
			if err != nil {
				return
			}
			httpx.Write(w, r, http.StatusOK, map[string]Customer{"customer": customerForActor(customer, user)})
			return
		}

		if len(parts) == 2 && parts[1] == "document-asks" && r.Method == http.MethodPost {
			user, ok := authService.UserFromRequest(r)
			if !ok {
				httpx.Error(w, r, http.StatusUnauthorized, "not_authenticated", "Sign in is required.")
				return
			}
			var payload AskDocumentsPayload
			if !httpx.DecodeJSON(w, r, &payload) {
				return
			}
			customer, err := service.AskDocuments(customerID, payload, user)
			writeCustomerErr(w, r, err)
			if err != nil {
				return
			}
			httpx.Write(w, r, http.StatusOK, map[string]Customer{"customer": customerForActor(customer, user)})
			return
		}

		if len(parts) == 4 && parts[1] == "document-asks" && parts[3] == "withdraw" && r.Method == http.MethodPost {
			user, ok := authService.UserFromRequest(r)
			if !ok {
				httpx.Error(w, r, http.StatusUnauthorized, "not_authenticated", "Sign in is required.")
				return
			}
			customer, err := service.WithdrawDocumentAsk(customerID, parts[2], user)
			writeCustomerErr(w, r, err)
			if err != nil {
				return
			}
			httpx.Write(w, r, http.StatusOK, map[string]Customer{"customer": customerForActor(customer, user)})
			return
		}

		if len(parts) == 4 && parts[1] == "documents" && parts[3] == "delete" && r.Method == http.MethodPost {
			user, ok := authService.UserFromRequest(r)
			if !ok {
				httpx.Error(w, r, http.StatusUnauthorized, "not_authenticated", "Sign in is required.")
				return
			}
			customer, err := service.DeleteDocument(customerID, parts[2], user)
			writeCustomerErr(w, r, err)
			if err != nil {
				return
			}
			httpx.Write(w, r, http.StatusOK, map[string]Customer{"customer": customerForActor(customer, user)})
			return
		}

		if len(parts) == 2 && parts[1] == "verification" && r.Method == http.MethodPost {
			user, ok := authService.UserFromRequest(r)
			if !ok {
				httpx.Error(w, r, http.StatusUnauthorized, "not_authenticated", "Sign in is required.")
				return
			}
			var payload VerificationRequest
			if !httpx.DecodeJSON(w, r, &payload) {
				return
			}
			customer, err := service.SetVerification(customerID, payload, user)
			writeCustomerErr(w, r, err)
			if err != nil {
				return
			}
			httpx.Write(w, r, http.StatusOK, map[string]Customer{"customer": customerForActor(customer, user)})
			return
		}

		if len(parts) == 2 && parts[1] == "document-requests" && r.Method == http.MethodPost {
			user, ok := authService.UserFromRequest(r)
			if !ok {
				httpx.Error(w, r, http.StatusUnauthorized, "not_authenticated", "Sign in is required.")
				return
			}
			var payload CreateDocumentRequestPayload
			if !httpx.DecodeJSON(w, r, &payload) {
				return
			}
			request, link, err := service.CreateDocumentRequest(customerID, payload, user, service.portalURL)
			writeCustomerErr(w, r, err)
			if err != nil {
				return
			}
			customer, _ := service.GetByID(customerID)
			httpx.Write(w, r, http.StatusOK, map[string]any{
				"request":  request,
				"url":      link,
				"customer": customerForActor(customer, user),
			})
			return
		}

		httpx.Error(w, r, http.StatusNotFound, "not_found", "Customer route was not found.")
	})

	mux.HandleFunc("/api/public/document-requests/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/public/document-requests/"), "/")
		parts := strings.Split(path, "/")
		if len(parts) == 0 || parts[0] == "" {
			httpx.Error(w, r, http.StatusNotFound, "not_found", "Document request was not found.")
			return
		}
		token := parts[0]

		if len(parts) == 1 && r.Method == http.MethodGet {
			request, customer, err := service.PublicDocumentRequest(token)
			writeCustomerErr(w, r, err)
			if err != nil {
				return
			}
			httpx.Write(w, r, http.StatusOK, map[string]any{
				"request":              request,
				"slots":                requiredKYCKinds,
				"verificationStatus":   firstNonEmpty(customer.VerificationStatus, "unverified"),
				"submitted":            publicSubmittedSummary(customer),
				"requestedDocuments":   customer.RequestedDocuments,
				"documentsSubmittedAt": customer.DocumentsSubmittedAt,
			})
			return
		}

		if len(parts) == 2 && parts[1] == "submit" && r.Method == http.MethodPost {
			customer, updatedRequest, err := service.SubmitPublicDocuments(token)
			writeCustomerErr(w, r, err)
			if err != nil {
				return
			}
			httpx.Write(w, r, http.StatusOK, map[string]any{
				"request":            updatedRequest,
				"verificationStatus": firstNonEmpty(customer.VerificationStatus, "unverified"),
				"submitted":          publicSubmittedSummary(customer),
				"requestedDocuments": customer.RequestedDocuments,
			})
			return
		}

		if len(parts) == 2 && parts[1] == "upload" && r.Method == http.MethodPost {
			if fileService == nil {
				httpx.Error(w, r, http.StatusServiceUnavailable, "uploads_unavailable", "File uploads are unavailable.")
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, 10<<20+1024)
			if err := r.ParseMultipartForm(10 << 20); err != nil {
				httpx.Error(w, r, http.StatusBadRequest, "invalid_upload", "Upload could not be read.")
				return
			}
			kind := r.FormValue("kind")
			label := r.FormValue("label")
			requestID := strings.TrimSpace(r.FormValue("requestId"))
			validateKind := kind
			if requestID != "" {
				validateKind = ""
			}
			request, err := service.ValidatePublicDocumentUpload(token, validateKind)
			writeCustomerErr(w, r, err)
			if err != nil {
				return
			}
			file, header, err := r.FormFile("file")
			if err != nil {
				httpx.Error(w, r, http.StatusBadRequest, "file_required", "Choose a file to upload.")
				return
			}
			record, err := fileService.CreateTrusted(file, header, fileuploads.UploadRequest{
				CustomerID: request.CustomerID,
				OwnerType:  "customer_document",
				OwnerID:    request.CustomerID,
			}, "document-link", request.TenantID)
			if err != nil {
				httpx.Error(w, r, http.StatusBadRequest, "invalid_file", "Use a JPEG, PNG, WebP or PDF file up to 10 MB.")
				return
			}
			customer, updatedRequest, err := service.AttachPublicDocument(token, AttachDocumentRequest{
				FileID:       record.ID,
				Kind:         kind,
				Label:        label,
				OriginalName: record.OriginalName,
				RequestID:    requestID,
			})
			if err != nil {
				fileService.DiscardCustomerDocument(record.ID, request.CustomerID)
				writeCustomerErr(w, r, err)
				return
			}
			httpx.Write(w, r, http.StatusOK, map[string]any{
				"request":            updatedRequest,
				"verificationStatus": firstNonEmpty(customer.VerificationStatus, "unverified"),
				"submitted":          publicSubmittedSummary(customer),
				"requestedDocuments": customer.RequestedDocuments,
			})
			return
		}

		httpx.Error(w, r, http.StatusNotFound, "not_found", "Document request route was not found.")
	})
}

func writeCustomerErr(w http.ResponseWriter, r *http.Request, err error) {
	if err == nil {
		return
	}
	switch {
	case errors.Is(err, errCustomerNotFound), errors.Is(err, errDocumentNotFound), errors.Is(err, errRequestNotFound):
		httpx.Error(w, r, http.StatusNotFound, "not_found", "Record was not found.")
	case errors.Is(err, errRequestExpired):
		httpx.Error(w, r, http.StatusGone, "expired", "This upload link has expired.")
	case errors.Is(err, errRequestClosed):
		httpx.Error(w, r, http.StatusGone, "closed", "This secure upload link is closed. Ask ChakuChuri for a new link.")
	case errors.Is(err, errVerificationNotReady):
		httpx.Error(w, r, http.StatusConflict, "verification_not_ready", "Accept every required document before marking this account verified.")
	case errors.Is(err, errVerificationInReview):
		httpx.Error(w, r, http.StatusConflict, "verification_in_review", "Your documents are already in review. No further action is needed right now.")
	case errors.Is(err, errForbiddenCustomer):
		httpx.Error(w, r, http.StatusForbidden, "forbidden", "You do not have access to this customer.")
	case errors.Is(err, errInvalidDocument), errors.Is(err, errInvalidVerification):
		httpx.Error(w, r, http.StatusBadRequest, "invalid_request", "Request could not be processed.")
	default:
		httpx.Error(w, r, http.StatusBadRequest, "invalid_request", "Request could not be processed.")
	}
}
