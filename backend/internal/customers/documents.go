package customers

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"chakuchuri/backend/internal/auth"
)

var (
	errCustomerNotFound     = errors.New("customer not found")
	errDocumentNotFound     = errors.New("document not found")
	errRequestNotFound      = errors.New("document request not found")
	errRequestExpired       = errors.New("document request expired")
	errRequestClosed        = errors.New("document request closed")
	errForbiddenCustomer    = errors.New("forbidden")
	errVerificationNotReady = errors.New("verification documents are not ready")
	errVerificationInReview = errors.New("verification is already in review")
	errInvalidDocument      = errors.New("invalid document")
	errInvalidVerification  = errors.New("invalid verification status")
)

var requiredKYCKinds = []string{"cnic_front", "cnic_back", "selfie"}

type CustomerDocument struct {
	ID                string `json:"id"`
	Kind              string `json:"kind"`
	Label             string `json:"label,omitempty"`
	FileID            string `json:"fileId"`
	OriginalName      string `json:"originalName"`
	Status            string `json:"status"`
	UploadedAt        string `json:"uploadedAt"`
	UploadedByUserID  string `json:"uploadedByUserId,omitempty"`
	RequestID         string `json:"requestId,omitempty"`
	DocumentRequestID string `json:"documentRequestId,omitempty"`
}

type RequestedDocument struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Kind      string `json:"kind"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
	Note      string `json:"note,omitempty"`
}

type DocumentRequest struct {
	ID          string   `json:"id"`
	Token       string   `json:"token,omitempty"`
	TokenHash   string   `json:"tokenHash,omitempty"`
	TenantID    string   `json:"-"`
	CustomerID  string   `json:"customerId"`
	Kinds       []string `json:"kinds"`
	Status      string   `json:"status"`
	Note        string   `json:"note,omitempty"`
	ExpiresAt   string   `json:"expiresAt"`
	CreatedAt   string   `json:"createdAt"`
	CreatedBy   string   `json:"createdBy"`
	SubmittedAt string   `json:"submittedAt,omitempty"`
	RevokedAt   string   `json:"revokedAt,omitempty"`
	UsedAt      string   `json:"usedAt,omitempty"`
	CompanyName string   `json:"companyName,omitempty"`
}

type AttachDocumentRequest struct {
	FileID       string `json:"fileId"`
	Kind         string `json:"kind"`
	Label        string `json:"label"`
	OriginalName string `json:"originalName"`
	RequestID    string `json:"requestId"`
}

type VerificationRequest struct {
	Status string `json:"status"`
	Note   string `json:"note"`
	CNIC   string `json:"cnic"`
}

type CreateDocumentRequestPayload struct {
	Kinds         []string `json:"kinds"`
	ExpiresInDays int      `json:"expiresInDays"`
	Note          string   `json:"note"`
}

type AskDocumentsPayload struct {
	Labels []string `json:"labels"`
	Note   string   `json:"note"`
}

type ReviewDocumentPayload struct {
	Status string `json:"status"`
	Note   string `json:"note"`
}

type OpenInvitePayload struct {
	Note string `json:"note"`
}

func normalizeDocumentKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "cnic_front", "cnic-front", "cnicfront":
		return "cnic_front"
	case "cnic_back", "cnic-back", "cnicback":
		return "cnic_back"
	case "selfie":
		return "selfie"
	case "other", "":
		return "other"
	default:
		return "other"
	}
}

func documentKindLabel(kind, label string) string {
	switch kind {
	case "cnic_front":
		return "CNIC front"
	case "cnic_back":
		return "CNIC back"
	case "selfie":
		return "Selfie"
	default:
		if strings.TrimSpace(label) != "" {
			return strings.TrimSpace(label)
		}
		return "Other document"
	}
}

func normalizeVerificationStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "verified":
		return "verified"
	default:
		// Legacy pending/rejected account statuses collapse to unverified.
		return "unverified"
	}
}

func normalizeDocStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "draft":
		return "draft"
	case "submitted", "uploaded":
		// "uploaded" is legacy locked status from the older one-shot flow.
		return "submitted"
	case "accepted":
		return "accepted"
	case "rejected":
		return "rejected"
	case "requested":
		return "requested"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func docIsEditable(status string) bool {
	switch normalizeDocStatus(status) {
	case "draft", "rejected", "requested", "":
		return true
	default:
		return false
	}
}

func docIsLocked(status string) bool {
	switch normalizeDocStatus(status) {
	case "submitted", "accepted":
		return true
	default:
		return false
	}
}

func (s *Service) GetByID(id string) (Customer, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, customer := range s.customers {
		if customer.ID == id {
			return normalizeCustomer(customer), true
		}
	}
	return Customer{}, false
}

func normalizeCustomer(customer Customer) Customer {
	customer.VerificationStatus = firstNonEmpty(normalizeVerificationStatus(customer.VerificationStatus), "unverified")
	if customer.Documents == nil {
		customer.Documents = []CustomerDocument{}
	}
	for i := range customer.Documents {
		customer.Documents[i].Status = firstNonEmpty(normalizeDocStatus(customer.Documents[i].Status), customer.Documents[i].Status)
	}
	customer.RequestedDocuments = reconcileRequestedDocuments(customer.RequestedDocuments, customer.Documents)
	customer.VerificationStage = verificationStageFor(customer)
	if customer.VerificationStage == "in_review" || customer.VerificationStage == "ready_to_verify" || customer.VerificationStage == "verified" {
		customer.VerificationInviteOpen = false
	}
	return customer
}

func reconcileRequestedDocuments(asks []RequestedDocument, docs []CustomerDocument) []RequestedDocument {
	if len(asks) == 0 {
		return []RequestedDocument{}
	}
	byRequest := map[string]CustomerDocument{}
	for _, doc := range docs {
		if requestID := strings.TrimSpace(doc.RequestID); requestID != "" {
			byRequest[requestID] = doc
		}
	}
	next := make([]RequestedDocument, 0, len(asks))
	for _, ask := range asks {
		ask.ID = strings.TrimSpace(ask.ID)
		ask.Label = strings.TrimSpace(ask.Label)
		if ask.ID == "" && ask.Label == "" {
			continue
		}
		storedStatus := strings.ToLower(strings.TrimSpace(ask.Status))
		if storedStatus == "withdrawn" || storedStatus == "cancelled" || storedStatus == "canceled" {
			ask.Status = "withdrawn"
			next = append(next, ask)
			continue
		}
		doc, found := byRequest[ask.ID]
		if !found || strings.TrimSpace(doc.FileID) == "" {
			// A fulfilled legacy request with no linked evidence was deleted by staff.
			// Preserve it as withdrawn history instead of resurrecting a customer task.
			if storedStatus == "fulfilled" || storedStatus == "submitted" || storedStatus == "accepted" {
				ask.Status = "withdrawn"
			} else {
				ask.Status = "open"
			}
			next = append(next, ask)
			continue
		}
		switch normalizeDocStatus(doc.Status) {
		case "accepted":
			ask.Status = "accepted"
		case "submitted", "uploaded":
			ask.Status = "submitted"
		default:
			ask.Status = "open"
		}
		next = append(next, ask)
	}
	return next
}

func activeRequestedDocuments(asks []RequestedDocument) []RequestedDocument {
	active := make([]RequestedDocument, 0, len(asks))
	for _, ask := range asks {
		if !strings.EqualFold(strings.TrimSpace(ask.Status), "withdrawn") {
			active = append(active, ask)
		}
	}
	return active
}

func verificationReadyForNormalizedCustomer(customer Customer) (bool, string) {
	byKind := map[string]CustomerDocument{}
	byRequest := map[string]CustomerDocument{}
	for _, doc := range customer.Documents {
		if doc.Kind != "other" {
			byKind[doc.Kind] = doc
		}
		if requestID := strings.TrimSpace(doc.RequestID); requestID != "" {
			byRequest[requestID] = doc
		}
	}
	for _, kind := range requiredKYCKinds {
		doc, ok := byKind[kind]
		if !ok || normalizeDocStatus(doc.Status) != "accepted" {
			return false, documentKindLabel(kind, "") + " must be accepted"
		}
	}
	for _, ask := range activeRequestedDocuments(customer.RequestedDocuments) {
		doc, ok := byRequest[ask.ID]
		if !ok || normalizeDocStatus(doc.Status) != "accepted" {
			return false, firstNonEmpty(strings.TrimSpace(ask.Label), "Requested document") + " must be accepted"
		}
	}
	return true, "All required documents are accepted"
}

func verificationStageFor(customer Customer) string {
	if normalizeVerificationStatus(customer.VerificationStatus) == "verified" {
		return "verified"
	}
	activeRequestIDs := map[string]struct{}{}
	for _, ask := range activeRequestedDocuments(customer.RequestedDocuments) {
		activeRequestIDs[ask.ID] = struct{}{}
	}
	byRequest := map[string]CustomerDocument{}
	hasSubmitted := false
	hasAccepted := false
	for _, doc := range customer.Documents {
		if requestID := strings.TrimSpace(doc.RequestID); requestID != "" {
			byRequest[requestID] = doc
			if _, active := activeRequestIDs[requestID]; !active {
				// Retain withdrawn evidence for audit without keeping the live workflow open.
				continue
			}
		}
		switch normalizeDocStatus(doc.Status) {
		case "draft", "rejected", "requested":
			return "action_required"
		case "submitted", "uploaded":
			hasSubmitted = true
		case "accepted":
			hasAccepted = true
		}
	}
	for _, ask := range activeRequestedDocuments(customer.RequestedDocuments) {
		doc, ok := byRequest[ask.ID]
		if !ok || normalizeDocStatus(doc.Status) == "draft" || normalizeDocStatus(doc.Status) == "rejected" {
			return "action_required"
		}
	}
	if hasSubmitted {
		return "in_review"
	}
	if ready, _ := verificationReadyForNormalizedCustomer(customer); ready {
		return "ready_to_verify"
	}
	if customer.VerificationInviteOpen {
		return "action_required"
	}
	if strings.TrimSpace(customer.DocumentsSubmittedAt) != "" || hasAccepted {
		return "in_review"
	}
	return "not_started"
}

// redactDocumentSecrets removes file handles after final submit so customers cannot reopen KYC media.
func redactDocumentSecrets(docs []CustomerDocument) []CustomerDocument {
	if len(docs) == 0 {
		return []CustomerDocument{}
	}
	out := make([]CustomerDocument, len(docs))
	for i, doc := range docs {
		out[i] = CustomerDocument{
			ID:                doc.ID,
			Kind:              doc.Kind,
			Label:             doc.Label,
			Status:            normalizeDocStatus(doc.Status),
			UploadedAt:        doc.UploadedAt,
			RequestID:         doc.RequestID,
			DocumentRequestID: doc.DocumentRequestID,
		}
		// Drafts stay previewable until the customer clicks final Submit.
		if normalizeDocStatus(doc.Status) == "draft" {
			out[i].FileID = doc.FileID
			out[i].OriginalName = doc.OriginalName
		}
	}
	return out
}

func customerForActor(customer Customer, user auth.User) Customer {
	customer = normalizeCustomer(customer)
	if auth.IsCustomerRole(user) {
		customer.Documents = redactDocumentSecrets(customer.Documents)
		return customer
	}
	if !auth.HasPermission(user, "kyc.review") {
		customer.CNIC = ""
		customer.IdentityNote = ""
		customer.Documents = []CustomerDocument{}
		customer.RequestedDocuments = []RequestedDocument{}
		customer.DocumentsSubmittedAt = ""
		customer.VerifiedByUserID = ""
		customer.VerificationInviteOpen = false
	}
	return customer
}

func publicSubmittedSummary(customer Customer) []map[string]string {
	customer = normalizeCustomer(customer)
	rows := make([]map[string]string, 0, len(customer.Documents))
	for _, doc := range customer.Documents {
		row := map[string]string{
			"kind":       doc.Kind,
			"label":      doc.Label,
			"status":     normalizeDocStatus(doc.Status),
			"uploadedAt": doc.UploadedAt,
		}
		if requestID := strings.TrimSpace(doc.RequestID); requestID != "" {
			row["requestId"] = requestID
		}
		if normalizeDocStatus(doc.Status) == "draft" {
			row["originalName"] = doc.OriginalName
		}
		rows = append(rows, row)
	}
	return rows
}

// CanAccessDocument is the single authorization gate for private KYC media.
func (s *Service) CanAccessDocument(user auth.User, fileID string) bool {
	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return false
	}
	if !auth.IsCustomerRole(user) && !auth.HasPermission(user, "kyc.review") {
		return false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, customer := range s.customers {
		if auth.IsCustomerRole(user) && customer.ID != strings.TrimSpace(user.CustomerID) {
			continue
		}
		for _, doc := range customer.Documents {
			if doc.FileID != fileID {
				continue
			}
			if auth.IsCustomerRole(user) {
				return normalizeDocStatus(doc.Status) == "draft"
			}
			return true
		}
	}
	return false
}

// CanCustomerPreviewDocument remains as a compatibility alias for older wiring.
func (s *Service) CanCustomerPreviewDocument(user auth.User, fileID string) bool {
	return s.CanAccessDocument(user, fileID)
}

func (s *Service) ValidateDocumentUpload(customerID, kind, requestID string, actor auth.User) error {
	customerID = strings.TrimSpace(customerID)
	requestID = strings.TrimSpace(requestID)
	kind = normalizeDocumentKind(kind)
	if customerID == "" || !auth.CanAccessCustomer(actor, customerID) {
		return errForbiddenCustomer
	}
	if auth.IsCustomerRole(actor) && actor.CustomerID != customerID {
		return errForbiddenCustomer
	}
	if !auth.IsCustomerRole(actor) && !auth.HasPermission(actor, "kyc.review") {
		return errForbiddenCustomer
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	index := s.indexLocked(customerID)
	if index < 0 {
		return errCustomerNotFound
	}
	if auth.IsCustomerRole(actor) {
		if normalizeVerificationStatus(s.customers[index].VerificationStatus) == "verified" || !s.customers[index].VerificationInviteOpen {
			return errInvalidDocument
		}
	}
	if requestID != "" {
		found := false
		for _, ask := range s.customers[index].RequestedDocuments {
			if ask.ID == requestID && strings.EqualFold(ask.Status, "open") {
				found = true
				break
			}
		}
		if !found {
			return errInvalidDocument
		}
	}
	if kind != "other" {
		for _, doc := range s.customers[index].Documents {
			if doc.Kind == kind && !docIsEditable(doc.Status) {
				return errInvalidDocument
			}
		}
	}
	return nil
}

func (s *Service) AttachDocument(customerID string, payload AttachDocumentRequest, actor auth.User) (Customer, error) {
	customerID = strings.TrimSpace(customerID)
	fileID := strings.TrimSpace(payload.FileID)
	kind := normalizeDocumentKind(payload.Kind)
	requestID := strings.TrimSpace(payload.RequestID)
	if customerID == "" || fileID == "" {
		return Customer{}, errInvalidDocument
	}
	if !auth.CanAccessCustomer(actor, customerID) {
		return Customer{}, errForbiddenCustomer
	}
	if auth.IsCustomerRole(actor) && actor.CustomerID != customerID {
		return Customer{}, errForbiddenCustomer
	}
	if !auth.IsCustomerRole(actor) && !auth.HasPermission(actor, "kyc.review") {
		return Customer{}, errForbiddenCustomer
	}
	if s.documentFileValidator != nil && !s.documentFileValidator(fileID, customerID) {
		return Customer{}, errInvalidDocument
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.persistLocked()

	index := s.indexLocked(customerID)
	if index < 0 {
		return Customer{}, errCustomerNotFound
	}

	verification := normalizeVerificationStatus(s.customers[index].VerificationStatus)
	customerActor := auth.IsCustomerRole(actor)
	if customerActor && verification == "verified" {
		return Customer{}, errInvalidDocument
	}
	if customerActor && !s.customers[index].VerificationInviteOpen {
		// Allow replace only while the invite/form is open (or after a reject re-opened it).
		return Customer{}, errInvalidDocument
	}

	docs := append([]CustomerDocument{}, s.customers[index].Documents...)
	if customerActor && kind != "other" {
		for _, existing := range docs {
			if existing.Kind == kind && !docIsEditable(existing.Status) {
				return Customer{}, errInvalidDocument
			}
		}
	}

	label := documentKindLabel(kind, payload.Label)
	if requestID != "" {
		asks := s.customers[index].RequestedDocuments
		foundAsk := false
		for _, ask := range asks {
			if ask.ID == requestID && strings.EqualFold(ask.Status, "open") {
				foundAsk = true
				if strings.TrimSpace(ask.Label) != "" {
					label = ask.Label
				}
				kind = firstNonEmpty(normalizeDocumentKind(ask.Kind), "other")
				break
			}
		}
		if !foundAsk && customerActor {
			return Customer{}, errInvalidDocument
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	doc := CustomerDocument{
		ID:               "doc_" + randomRequestToken()[:16],
		Kind:             kind,
		Label:            label,
		FileID:           fileID,
		OriginalName:     firstNonEmpty(strings.TrimSpace(payload.OriginalName), fileID),
		Status:           "draft",
		UploadedAt:       now,
		UploadedByUserID: actor.ID,
		RequestID:        requestID,
	}

	if kind != "other" {
		filtered := docs[:0]
		for _, existing := range docs {
			if existing.Kind != kind {
				filtered = append(filtered, existing)
			}
		}
		docs = append(filtered, doc)
	} else if requestID != "" {
		filtered := docs[:0]
		for _, existing := range docs {
			if existing.RequestID != requestID {
				filtered = append(filtered, existing)
			}
		}
		docs = append(filtered, doc)
	} else {
		docs = append(docs, doc)
	}
	s.customers[index].Documents = docs
	if s.recorder != nil {
		s.recorder.Record(actor.Email, "customers.document_uploaded", customerID, "Customer document drafted: "+doc.Label)
	}
	s.notifyChange("customers")
	return normalizeCustomer(s.customers[index]), nil
}

func (s *Service) DeleteDocument(customerID, documentID string, actor auth.User) (Customer, error) {
	customerID = strings.TrimSpace(customerID)
	documentID = strings.TrimSpace(documentID)
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.persistLocked()

	index := s.indexLocked(customerID)
	if index < 0 {
		return Customer{}, errCustomerNotFound
	}
	if !auth.CanAccessCustomer(actor, customerID) {
		return Customer{}, errForbiddenCustomer
	}

	docs := s.customers[index].Documents
	next := make([]CustomerDocument, 0, len(docs))
	found := false
	requestID := ""
	for _, doc := range docs {
		if doc.ID != documentID {
			next = append(next, doc)
			continue
		}
		found = true
		requestID = strings.TrimSpace(doc.RequestID)
		if auth.IsCustomerRole(actor) {
			if actor.CustomerID != customerID || !docIsEditable(doc.Status) {
				return Customer{}, errForbiddenCustomer
			}
		} else if !auth.HasPermission(actor, "kyc.review") {
			return Customer{}, errForbiddenCustomer
		} else if !docIsEditable(doc.Status) {
			return Customer{}, errInvalidDocument
		}
	}
	if !found {
		return Customer{}, errDocumentNotFound
	}
	s.customers[index].Documents = next
	if requestID != "" {
		for i := range s.customers[index].RequestedDocuments {
			if s.customers[index].RequestedDocuments[i].ID == requestID && !strings.EqualFold(s.customers[index].RequestedDocuments[i].Status, "withdrawn") {
				s.customers[index].RequestedDocuments[i].Status = "open"
			}
		}
	}
	s.customers[index] = normalizeCustomer(s.customers[index])
	if s.recorder != nil {
		s.recorder.Record(actor.Email, "customers.document_deleted", customerID, "Customer document removed.")
	}
	s.notifyChange("customers")
	return normalizeCustomer(s.customers[index]), nil
}

func (s *Service) SubmitDocuments(customerID string, actor auth.User) (Customer, error) {
	customerID = strings.TrimSpace(customerID)
	if customerID == "" {
		return Customer{}, errInvalidDocument
	}
	if !auth.CanAccessCustomer(actor, customerID) {
		return Customer{}, errForbiddenCustomer
	}
	if auth.IsCustomerRole(actor) && actor.CustomerID != customerID {
		return Customer{}, errForbiddenCustomer
	}
	if !auth.IsCustomerRole(actor) && !auth.HasPermission(actor, "kyc.review") {
		return Customer{}, errForbiddenCustomer
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.persistLocked()

	index := s.indexLocked(customerID)
	if index < 0 {
		return Customer{}, errCustomerNotFound
	}
	if normalizeVerificationStatus(s.customers[index].VerificationStatus) == "verified" {
		return Customer{}, errInvalidDocument
	}
	if auth.IsCustomerRole(actor) && !s.customers[index].VerificationInviteOpen {
		return Customer{}, errInvalidDocument
	}

	docs := append([]CustomerDocument{}, s.customers[index].Documents...)
	byKind := map[string]CustomerDocument{}
	for _, doc := range docs {
		if doc.Kind != "other" {
			byKind[doc.Kind] = doc
		}
	}
	hasDraft := false
	for _, doc := range docs {
		if normalizeDocStatus(doc.Status) == "draft" {
			hasDraft = true
			break
		}
	}
	if !hasDraft {
		return Customer{}, errInvalidDocument
	}
	for _, kind := range requiredKYCKinds {
		doc, ok := byKind[kind]
		if !ok || normalizeDocStatus(doc.Status) == "rejected" || strings.TrimSpace(doc.FileID) == "" {
			return Customer{}, errInvalidDocument
		}
		st := normalizeDocStatus(doc.Status)
		if st == "accepted" || st == "submitted" {
			continue
		}
		if st != "draft" {
			return Customer{}, errInvalidDocument
		}
	}
	for _, ask := range s.customers[index].RequestedDocuments {
		if !strings.EqualFold(ask.Status, "open") {
			continue
		}
		ok := false
		for _, doc := range docs {
			if doc.RequestID == ask.ID && strings.TrimSpace(doc.FileID) != "" && normalizeDocStatus(doc.Status) != "rejected" {
				ok = true
				break
			}
		}
		if !ok {
			return Customer{}, errInvalidDocument
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	for i := range docs {
		status := normalizeDocStatus(docs[i].Status)
		if status == "draft" {
			docs[i].Status = "submitted"
		}
	}
	asks := append([]RequestedDocument{}, s.customers[index].RequestedDocuments...)
	for i := range asks {
		if !strings.EqualFold(asks[i].Status, "open") {
			continue
		}
		fulfilled := false
		for _, doc := range docs {
			if doc.RequestID == asks[i].ID && strings.TrimSpace(doc.FileID) != "" && normalizeDocStatus(doc.Status) == "submitted" {
				fulfilled = true
				break
			}
		}
		if fulfilled {
			asks[i].Status = "submitted"
		}
	}

	s.customers[index].Documents = docs
	s.customers[index].RequestedDocuments = asks
	s.customers[index].DocumentsSubmittedAt = now
	s.customers[index].VerificationInviteOpen = false
	s.customers[index].VerificationStatus = "unverified"
	if s.recorder != nil {
		s.recorder.Record(actor.Email, "customers.documents_submitted", customerID, "Identity documents submitted for review.")
	}
	s.notifyChange("customers")
	return normalizeCustomer(s.customers[index]), nil
}

func (s *Service) ReviewDocument(customerID, documentID string, payload ReviewDocumentPayload, actor auth.User) (Customer, error) {
	if !auth.HasPermission(actor, "kyc.review") {
		return Customer{}, errForbiddenCustomer
	}
	status := strings.ToLower(strings.TrimSpace(payload.Status))
	if status != "accepted" && status != "rejected" {
		return Customer{}, errInvalidDocument
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.persistLocked()

	index := s.indexLocked(customerID)
	if index < 0 {
		return Customer{}, errCustomerNotFound
	}
	found := false
	requestID := ""
	for i := range s.customers[index].Documents {
		if s.customers[index].Documents[i].ID != documentID {
			continue
		}
		found = true
		current := normalizeDocStatus(s.customers[index].Documents[i].Status)
		if current != "submitted" && current != "accepted" && current != "rejected" && current != "draft" {
			return Customer{}, errInvalidDocument
		}
		requestID = strings.TrimSpace(s.customers[index].Documents[i].RequestID)
		s.customers[index].Documents[i].Status = status
		break
	}
	if !found {
		return Customer{}, errDocumentNotFound
	}
	if note := strings.TrimSpace(payload.Note); note != "" {
		s.customers[index].IdentityNote = note
	}
	if status == "rejected" {
		s.customers[index].VerificationInviteOpen = true
		s.customers[index].VerificationStatus = "unverified"
		s.customers[index].VerifiedAt = ""
		s.customers[index].VerifiedByUserID = ""
		if strings.TrimSpace(s.customers[index].IdentityNote) == "" {
			s.customers[index].IdentityNote = "One or more documents need to be re-uploaded."
		}
		if requestID != "" {
			asks := s.customers[index].RequestedDocuments
			for i := range asks {
				if asks[i].ID == requestID {
					asks[i].Status = "open"
				}
			}
			s.customers[index].RequestedDocuments = asks
		}
	} else if status == "accepted" && requestID != "" {
		asks := s.customers[index].RequestedDocuments
		for i := range asks {
			if asks[i].ID == requestID {
				asks[i].Status = "accepted"
			}
		}
		s.customers[index].RequestedDocuments = asks
	}
	if s.recorder != nil {
		s.recorder.Record(actor.Email, "customers.document_reviewed", customerID, "Document marked "+status)
	}
	s.notifyChange("customers")
	return normalizeCustomer(s.customers[index]), nil
}

func (s *Service) OpenVerificationInvite(customerID string, payload OpenInvitePayload, actor auth.User) (Customer, error) {
	customerID = strings.TrimSpace(customerID)
	if customerID == "" {
		return Customer{}, errInvalidDocument
	}
	if !auth.CanAccessCustomer(actor, customerID) {
		return Customer{}, errForbiddenCustomer
	}
	if auth.IsCustomerRole(actor) && actor.CustomerID != customerID {
		return Customer{}, errForbiddenCustomer
	}
	if !auth.IsCustomerRole(actor) && !auth.HasPermission(actor, "kyc.review") {
		return Customer{}, errForbiddenCustomer
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.persistLocked()

	index := s.indexLocked(customerID)
	if index < 0 {
		return Customer{}, errCustomerNotFound
	}
	s.customers[index] = normalizeCustomer(s.customers[index])
	if normalizeVerificationStatus(s.customers[index].VerificationStatus) == "verified" {
		return Customer{}, errInvalidDocument
	}
	if auth.IsCustomerRole(actor) && (s.customers[index].VerificationStage == "in_review" || s.customers[index].VerificationStage == "ready_to_verify") {
		return Customer{}, errVerificationInReview
	}
	s.customers[index].VerificationInviteOpen = true
	s.customers[index].VerificationStatus = "unverified"
	if note := strings.TrimSpace(payload.Note); note != "" {
		s.customers[index].IdentityNote = note
	} else if auth.IsCustomerRole(actor) && strings.TrimSpace(s.customers[index].IdentityNote) == "" {
		s.customers[index].IdentityNote = "Complete your one-time identity form to get verified."
	} else if !auth.IsCustomerRole(actor) && strings.TrimSpace(payload.Note) == "" && strings.TrimSpace(s.customers[index].IdentityNote) == "" {
		s.customers[index].IdentityNote = "Please complete identity verification for your ChakuChuri account."
	}
	if s.recorder != nil {
		s.recorder.Record(actor.Email, "customers.verification_invite", customerID, "Verification form opened.")
	}
	s.notifyChange("customers")
	return normalizeCustomer(s.customers[index]), nil
}

func (s *Service) AskDocuments(customerID string, payload AskDocumentsPayload, actor auth.User) (Customer, error) {
	if !auth.HasPermission(actor, "kyc.review") {
		return Customer{}, errForbiddenCustomer
	}
	labels := make([]string, 0, len(payload.Labels))
	for _, label := range payload.Labels {
		label = strings.TrimSpace(label)
		if label != "" {
			labels = append(labels, label)
		}
	}
	if len(labels) == 0 {
		return Customer{}, errInvalidDocument
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.persistLocked()

	index := s.indexLocked(customerID)
	if index < 0 {
		return Customer{}, errCustomerNotFound
	}
	now := time.Now().UTC().Format(time.RFC3339)
	asks := append([]RequestedDocument{}, s.customers[index].RequestedDocuments...)
	for _, label := range labels {
		asks = append(asks, RequestedDocument{
			ID:        "ask_" + randomRequestToken()[:14],
			Label:     label,
			Kind:      "other",
			Status:    "open",
			CreatedAt: now,
			Note:      strings.TrimSpace(payload.Note),
		})
	}
	s.customers[index].RequestedDocuments = asks
	if note := strings.TrimSpace(payload.Note); note != "" {
		s.customers[index].IdentityNote = note
	} else if strings.TrimSpace(s.customers[index].IdentityNote) == "" {
		s.customers[index].IdentityNote = "Additional documents requested by ChakuChuri."
	}
	s.customers[index].VerificationInviteOpen = true
	s.customers[index].VerificationStatus = "unverified"
	s.customers[index].VerifiedAt = ""
	s.customers[index].VerifiedByUserID = ""
	if s.recorder != nil {
		s.recorder.Record(actor.Email, "customers.documents_asked", customerID, "Additional documents requested.")
	}
	s.notifyChange("customers")
	return normalizeCustomer(s.customers[index]), nil
}

func (s *Service) WithdrawDocumentAsk(customerID, requestID string, actor auth.User) (Customer, error) {
	if !auth.HasPermission(actor, "kyc.review") || !auth.CanAccessCustomer(actor, customerID) {
		return Customer{}, errForbiddenCustomer
	}
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return Customer{}, errInvalidDocument
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.persistLocked()

	index := s.indexLocked(strings.TrimSpace(customerID))
	if index < 0 {
		return Customer{}, errCustomerNotFound
	}
	found := false
	for i := range s.customers[index].RequestedDocuments {
		if s.customers[index].RequestedDocuments[i].ID != requestID {
			continue
		}
		found = true
		s.customers[index].RequestedDocuments[i].Status = "withdrawn"
		break
	}
	if !found {
		return Customer{}, errDocumentNotFound
	}
	s.customers[index] = normalizeCustomer(s.customers[index])
	if s.recorder != nil {
		s.recorder.Record(actor.Email, "customers.document_request_withdrawn", customerID, "Additional KYC requirement withdrawn.")
	}
	s.notifyChange("customers")
	return s.customers[index], nil
}

func verificationReadiness(customer Customer) (bool, string) {
	customer = normalizeCustomer(customer)
	return verificationReadyForNormalizedCustomer(customer)
}

func (s *Service) SetVerification(customerID string, payload VerificationRequest, actor auth.User) (Customer, error) {
	if !auth.HasPermission(actor, "kyc.review") {
		return Customer{}, errForbiddenCustomer
	}
	status := strings.ToLower(strings.TrimSpace(payload.Status))
	if status != "verified" && status != "unverified" {
		return Customer{}, errInvalidVerification
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.persistLocked()

	index := s.indexLocked(customerID)
	if index < 0 {
		return Customer{}, errCustomerNotFound
	}
	s.customers[index] = normalizeCustomer(s.customers[index])
	if status == "verified" {
		if ready, _ := verificationReadiness(s.customers[index]); !ready {
			return Customer{}, errVerificationNotReady
		}
	}

	s.customers[index].VerificationStatus = status
	if note := strings.TrimSpace(payload.Note); note != "" {
		s.customers[index].IdentityNote = note
	}
	if cnic := strings.TrimSpace(payload.CNIC); cnic != "" {
		s.customers[index].CNIC = cnic
	}
	if status == "verified" {
		s.customers[index].VerifiedAt = time.Now().UTC().Format(time.RFC3339)
		s.customers[index].VerifiedByUserID = actor.ID
		s.customers[index].VerificationInviteOpen = false
	} else {
		s.customers[index].VerifiedAt = ""
		s.customers[index].VerifiedByUserID = ""
	}
	s.customers[index] = normalizeCustomer(s.customers[index])
	if s.recorder != nil {
		s.recorder.Record(actor.Email, "customers.verification_updated", customerID, "Verification set to "+status)
	}
	s.notifyChange("customers")
	return s.customers[index], nil
}

func (s *Service) CreateDocumentRequest(customerID string, payload CreateDocumentRequestPayload, actor auth.User, portalURL string) (DocumentRequest, string, error) {
	if !auth.HasPermission(actor, "kyc.review") {
		return DocumentRequest{}, "", errForbiddenCustomer
	}

	kinds := payload.Kinds
	if len(kinds) == 0 {
		kinds = append([]string{}, requiredKYCKinds...)
	}
	normalized := make([]string, 0, len(kinds))
	seen := map[string]struct{}{}
	for _, kind := range kinds {
		next := normalizeDocumentKind(kind)
		if _, exists := seen[next]; exists {
			continue
		}
		seen[next] = struct{}{}
		normalized = append(normalized, next)
	}
	if len(normalized) == 0 {
		return DocumentRequest{}, "", errInvalidDocument
	}
	days := payload.ExpiresInDays
	if days <= 0 || days > 30 {
		days = 7
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.persistLocked()

	index := s.indexLocked(strings.TrimSpace(customerID))
	if index < 0 {
		return DocumentRequest{}, "", errCustomerNotFound
	}
	if normalizeVerificationStatus(s.customers[index].VerificationStatus) == "verified" {
		return DocumentRequest{}, "", errInvalidVerification
	}

	now := time.Now().UTC()
	nowText := now.Format(time.RFC3339)
	for i := range s.documentRequests {
		if s.documentRequests[i].CustomerID != customerID {
			continue
		}
		switch documentRequestStatus(s.documentRequests[i]) {
		case "active":
			s.documentRequests[i].Status = "revoked"
			s.documentRequests[i].RevokedAt = nowText
		case "expired":
			s.documentRequests[i].Status = "expired"
		}
	}

	token := randomRequestToken()
	note := strings.TrimSpace(payload.Note)
	if note == "" {
		note = "Please complete identity verification for your ChakuChuri account."
	}
	request := DocumentRequest{
		ID:          "kyc_req_" + randomRequestToken()[:16],
		Token:       token,
		TokenHash:   hashRequestToken(token),
		TenantID:    s.customers[index].TenantID,
		CustomerID:  s.customers[index].ID,
		Kinds:       normalized,
		Status:      "active",
		Note:        note,
		ExpiresAt:   now.Add(time.Duration(days) * 24 * time.Hour).Format(time.RFC3339),
		CreatedAt:   nowText,
		CreatedBy:   actor.Email,
		CompanyName: s.customers[index].CompanyName,
	}

	s.customers[index].IdentityNote = note
	s.customers[index].VerificationInviteOpen = true
	s.customers[index].VerificationStatus = "unverified"
	s.customers[index].VerifiedAt = ""
	s.customers[index].VerifiedByUserID = ""
	s.documentRequests = append([]DocumentRequest{request}, s.documentRequests...)

	if s.recorder != nil {
		s.recorder.Record(actor.Email, "customers.document_request_created", customerID, "Secure verification link created; previous active links revoked.")
	}
	s.notifyChange("customers")
	link := strings.TrimRight(portalURL, "/") + "/verify/" + token
	return documentRequestResponse(request), link, nil
}

func (s *Service) PublicDocumentRequest(token string) (DocumentRequest, Customer, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	request, customerIndex, err := s.validatePublicRequestLocked(token, "", false)
	if err != nil && !errors.Is(err, errRequestClosed) {
		return DocumentRequest{}, Customer{}, err
	}
	if errors.Is(err, errRequestClosed) && documentRequestStatus(request) != "submitted" {
		return DocumentRequest{}, Customer{}, err
	}
	if customerIndex < 0 {
		return DocumentRequest{}, Customer{}, errCustomerNotFound
	}
	request.CompanyName = s.customers[customerIndex].CompanyName
	return documentRequestResponse(request), normalizeCustomer(s.customers[customerIndex]), nil
}

func (s *Service) ValidatePublicDocumentUpload(token, kind string) (DocumentRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	requireKind := strings.TrimSpace(kind) != ""
	request, _, err := s.validatePublicRequestLocked(token, kind, requireKind)
	if err != nil {
		return DocumentRequest{}, err
	}
	return request, nil
}

func (s *Service) AttachPublicDocument(token string, payload AttachDocumentRequest) (Customer, DocumentRequest, error) {
	fileID := strings.TrimSpace(payload.FileID)
	if fileID == "" {
		return Customer{}, DocumentRequest{}, errInvalidDocument
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.persistLocked()

	requireKind := strings.TrimSpace(payload.RequestID) == ""
	request, customerIndex, err := s.validatePublicRequestLocked(token, payload.Kind, requireKind)
	if err != nil {
		return Customer{}, DocumentRequest{}, err
	}
	if s.documentFileValidator != nil && !s.documentFileValidator(fileID, request.CustomerID) {
		return Customer{}, DocumentRequest{}, errInvalidDocument
	}

	kind := normalizeDocumentKind(payload.Kind)
	requestID := strings.TrimSpace(payload.RequestID)
	docs := append([]CustomerDocument{}, s.customers[customerIndex].Documents...)
	now := time.Now().UTC().Format(time.RFC3339)
	label := documentKindLabel(kind, payload.Label)
	if requestID != "" {
		asks := s.customers[customerIndex].RequestedDocuments
		foundAsk := false
		for _, ask := range asks {
			if ask.ID == requestID && strings.EqualFold(ask.Status, "open") {
				foundAsk = true
				if strings.TrimSpace(ask.Label) != "" {
					label = ask.Label
				}
				kind = firstNonEmpty(normalizeDocumentKind(ask.Kind), "other")
				break
			}
		}
		if !foundAsk {
			return Customer{}, DocumentRequest{}, errInvalidDocument
		}
	}
	doc := CustomerDocument{
		ID:                "doc_" + randomRequestToken()[:16],
		Kind:              kind,
		Label:             label,
		FileID:            fileID,
		OriginalName:      firstNonEmpty(strings.TrimSpace(payload.OriginalName), fileID),
		Status:            "draft",
		UploadedAt:        now,
		RequestID:         requestID,
		DocumentRequestID: request.ID,
	}
	if kind != "other" && requestID == "" {
		filtered := docs[:0]
		for _, existing := range docs {
			if existing.Kind != kind {
				filtered = append(filtered, existing)
			}
		}
		docs = append(filtered, doc)
	} else if requestID != "" {
		filtered := docs[:0]
		for _, existing := range docs {
			if existing.RequestID != requestID {
				filtered = append(filtered, existing)
			}
		}
		docs = append(filtered, doc)
	} else {
		docs = append(docs, doc)
	}
	s.customers[customerIndex].Documents = docs
	if s.recorder != nil {
		s.recorder.Record("document-link", "customers.document_uploaded", request.CustomerID, "KYC document drafted through secure link.")
	}
	s.notifyChange("customers")
	return normalizeCustomer(s.customers[customerIndex]), documentRequestResponse(request), nil
}

func (s *Service) SubmitPublicDocuments(token string) (Customer, DocumentRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.persistLocked()

	request, customerIndex, err := s.validatePublicRequestLocked(token, "", false)
	if err != nil {
		return Customer{}, DocumentRequest{}, err
	}
	docs := append([]CustomerDocument{}, s.customers[customerIndex].Documents...)
	byKind := map[string]CustomerDocument{}
	for _, doc := range docs {
		if doc.Kind != "other" {
			byKind[doc.Kind] = doc
		}
	}
	for _, kind := range requiredKYCKinds {
		if !documentRequestAllowsKind(request, kind) {
			continue
		}
		doc, ok := byKind[kind]
		if !ok || strings.TrimSpace(doc.FileID) == "" || normalizeDocStatus(doc.Status) == "rejected" {
			return Customer{}, DocumentRequest{}, errInvalidDocument
		}
		if normalizeDocStatus(doc.Status) == "accepted" {
			continue
		}
		if normalizeDocStatus(doc.Status) != "draft" || doc.DocumentRequestID != request.ID {
			return Customer{}, DocumentRequest{}, errInvalidDocument
		}
	}
	for _, ask := range activeRequestedDocuments(s.customers[customerIndex].RequestedDocuments) {
		if !strings.EqualFold(ask.Status, "open") {
			continue
		}
		fulfilled := false
		for _, doc := range docs {
			if doc.RequestID == ask.ID && strings.TrimSpace(doc.FileID) != "" && normalizeDocStatus(doc.Status) != "rejected" {
				if normalizeDocStatus(doc.Status) == "accepted" || (normalizeDocStatus(doc.Status) == "draft" && doc.DocumentRequestID == request.ID) {
					fulfilled = true
					break
				}
			}
		}
		if !fulfilled {
			return Customer{}, DocumentRequest{}, errInvalidDocument
		}
	}

	hasDraft := false
	for i := range docs {
		if docs[i].DocumentRequestID == request.ID && normalizeDocStatus(docs[i].Status) == "draft" {
			docs[i].Status = "submitted"
			hasDraft = true
		}
	}
	if !hasDraft {
		return Customer{}, DocumentRequest{}, errInvalidDocument
	}

	asks := append([]RequestedDocument{}, s.customers[customerIndex].RequestedDocuments...)
	for i := range asks {
		if !strings.EqualFold(asks[i].Status, "open") {
			continue
		}
		fulfilled := false
		for _, doc := range docs {
			if doc.RequestID == asks[i].ID && strings.TrimSpace(doc.FileID) != "" && normalizeDocStatus(doc.Status) == "submitted" {
				fulfilled = true
				break
			}
		}
		if fulfilled {
			asks[i].Status = "submitted"
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	s.customers[customerIndex].Documents = docs
	s.customers[customerIndex].RequestedDocuments = asks
	s.customers[customerIndex].DocumentsSubmittedAt = now
	s.customers[customerIndex].VerificationStatus = "unverified"
	s.customers[customerIndex].VerificationInviteOpen = false
	requestIndex := s.documentRequestIndexLocked(token)
	if requestIndex < 0 {
		return Customer{}, DocumentRequest{}, errRequestNotFound
	}
	s.documentRequests[requestIndex].Status = "submitted"
	s.documentRequests[requestIndex].SubmittedAt = now
	s.documentRequests[requestIndex].UsedAt = now
	if s.recorder != nil {
		s.recorder.Record("document-link", "customers.documents_submitted", request.CustomerID, "KYC package submitted through secure link.")
	}
	s.notifyChange("customers")
	return normalizeCustomer(s.customers[customerIndex]), documentRequestResponse(s.documentRequests[requestIndex]), nil
}

func (s *Service) validatePublicRequestLocked(token, kind string, requireKind bool) (DocumentRequest, int, error) {
	requestIndex := s.documentRequestIndexLocked(token)
	if requestIndex < 0 {
		return DocumentRequest{}, -1, errRequestNotFound
	}
	request := s.documentRequests[requestIndex]
	switch documentRequestStatus(request) {
	case "expired":
		return request, -1, errRequestExpired
	case "submitted", "revoked":
		customerIndex := s.indexLocked(request.CustomerID)
		return request, customerIndex, errRequestClosed
	}
	customerIndex := s.indexLocked(request.CustomerID)
	if customerIndex < 0 {
		return request, -1, errCustomerNotFound
	}
	if normalizeVerificationStatus(s.customers[customerIndex].VerificationStatus) == "verified" {
		return request, customerIndex, errRequestClosed
	}
	if !requireKind {
		return request, customerIndex, nil
	}

	normalizedKind := normalizeDocumentKind(kind)
	if !documentRequestAllowsKind(request, normalizedKind) {
		return request, customerIndex, errInvalidDocument
	}
	requestDocumentCount := 0
	for _, doc := range s.customers[customerIndex].Documents {
		if doc.DocumentRequestID == request.ID {
			requestDocumentCount++
		}
		if normalizedKind != "other" && doc.Kind == normalizedKind && !docIsEditable(doc.Status) {
			return request, customerIndex, errInvalidDocument
		}
	}
	if requestDocumentCount >= 10 {
		return request, customerIndex, errInvalidDocument
	}
	return request, customerIndex, nil
}

func (s *Service) documentRequestIndexLocked(token string) int {
	token = strings.TrimSpace(token)
	if token == "" {
		return -1
	}
	for i, request := range s.documentRequests {
		if documentRequestTokenMatches(request, token) {
			return i
		}
	}
	return -1
}

func normalizeLoadedDocumentRequests(customers []Customer, requests []DocumentRequest) []DocumentRequest {
	if requests == nil {
		return []DocumentRequest{}
	}
	submittedByCustomer := map[string]string{}
	tenantByCustomer := map[string]string{}
	for _, customer := range customers {
		submittedByCustomer[customer.ID] = customer.DocumentsSubmittedAt
		tenantByCustomer[customer.ID] = customer.TenantID
	}
	for i := range requests {
		if requests[i].TenantID == "" {
			requests[i].TenantID = tenantByCustomer[requests[i].CustomerID]
		}
		if requests[i].TokenHash == "" && requests[i].Token != "" {
			requests[i].TokenHash = hashRequestToken(requests[i].Token)
		}
		if requests[i].ID == "" {
			hash := requests[i].TokenHash
			if len(hash) > 16 {
				hash = hash[:16]
			}
			requests[i].ID = "kyc_req_" + hash
		}
		if strings.TrimSpace(requests[i].Status) == "" {
			requests[i].Status = "active"
			if requests[i].UsedAt != "" && requests[i].UsedAt == submittedByCustomer[requests[i].CustomerID] {
				requests[i].Status = "submitted"
				requests[i].SubmittedAt = requests[i].UsedAt
			}
		}
		requests[i].Status = documentRequestStatus(requests[i])
	}
	return requests
}

func documentRequestAllowsKind(request DocumentRequest, kind string) bool {
	for _, allowedKind := range request.Kinds {
		if normalizeDocumentKind(allowedKind) == normalizeDocumentKind(kind) {
			return true
		}
	}
	return false
}

func documentRequestStatus(request DocumentRequest) string {
	status := strings.ToLower(strings.TrimSpace(request.Status))
	if strings.TrimSpace(request.RevokedAt) != "" || status == "revoked" {
		return "revoked"
	}
	if strings.TrimSpace(request.SubmittedAt) != "" || status == "submitted" {
		return "submitted"
	}
	if status == "expired" || expired(request.ExpiresAt) {
		return "expired"
	}
	return "active"
}

func documentRequestResponse(request DocumentRequest) DocumentRequest {
	request.Status = documentRequestStatus(request)
	request.Token = ""
	request.TokenHash = ""
	request.TenantID = ""
	return request
}

func documentRequestTokenMatches(request DocumentRequest, token string) bool {
	if strings.TrimSpace(request.TokenHash) != "" {
		return subtle.ConstantTimeCompare([]byte(request.TokenHash), []byte(hashRequestToken(token))) == 1
	}
	return subtle.ConstantTimeCompare([]byte(request.Token), []byte(token)) == 1
}

func hashRequestToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func (s *Service) indexLocked(customerID string) int {
	for i, customer := range s.customers {
		if customer.ID == customerID {
			return i
		}
	}
	return -1
}

func (s *Service) lookupLocked(customerID string) (Customer, bool) {
	index := s.indexLocked(customerID)
	if index < 0 {
		return Customer{}, false
	}
	return s.customers[index], true
}

func expired(expiresAt string) bool {
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(expiresAt))
	if err != nil {
		return true
	}
	return time.Now().UTC().After(parsed)
}

func randomRequestToken() string {
	buffer := make([]byte, 24)
	if _, err := rand.Read(buffer); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return base64.RawURLEncoding.EncodeToString(buffer)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
