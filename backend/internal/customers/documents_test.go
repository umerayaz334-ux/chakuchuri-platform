package customers

import (
	"errors"
	"strings"
	"testing"

	"chakuchuri/backend/internal/auth"
)

func TestDocumentRequestLifecycleRevokesAndClosesLinks(t *testing.T) {
	service := kycTestService()
	reviewer := auth.User{ID: "user_owner", Role: "Owner", Email: "owner@example.com"}

	first, firstURL, err := service.CreateDocumentRequest("cust_test", CreateDocumentRequestPayload{
		Kinds:         append([]string{}, requiredKYCKinds...),
		ExpiresInDays: 7,
	}, reviewer, "http://portal.test")
	if err != nil {
		t.Fatalf("create first request: %v", err)
	}
	if first.Token != "" || first.Status != "active" {
		t.Fatalf("request response exposed a token or wrong status: %#v", first)
	}
	firstToken := strings.TrimPrefix(firstURL, "http://portal.test/verify/")

	_, secondURL, err := service.CreateDocumentRequest("cust_test", CreateDocumentRequestPayload{
		Kinds:         append([]string{}, requiredKYCKinds...),
		ExpiresInDays: 7,
	}, reviewer, "http://portal.test")
	if err != nil {
		t.Fatalf("create replacement request: %v", err)
	}
	if _, _, err := service.PublicDocumentRequest(firstToken); !errors.Is(err, errRequestClosed) {
		t.Fatalf("old request should be closed, got %v", err)
	}

	secondToken := strings.TrimPrefix(secondURL, "http://portal.test/verify/")
	for _, kind := range requiredKYCKinds {
		if _, err := service.ValidatePublicDocumentUpload(secondToken, kind); err != nil {
			t.Fatalf("validate %s: %v", kind, err)
		}
		if _, _, err := service.AttachPublicDocument(secondToken, AttachDocumentRequest{
			FileID:       "file_" + kind,
			Kind:         kind,
			OriginalName: kind + ".jpg",
		}); err != nil {
			t.Fatalf("attach %s: %v", kind, err)
		}
	}
	_, submittedRequest, err := service.SubmitPublicDocuments(secondToken)
	if err != nil {
		t.Fatalf("submit package: %v", err)
	}
	if submittedRequest.Status != "submitted" || submittedRequest.SubmittedAt == "" {
		t.Fatalf("request was not closed as submitted: %#v", submittedRequest)
	}
	if _, err := service.ValidatePublicDocumentUpload(secondToken, "selfie"); !errors.Is(err, errRequestClosed) {
		t.Fatalf("submitted link accepted another upload: %v", err)
	}
	if _, _, err := service.SubmitPublicDocuments(secondToken); !errors.Is(err, errRequestClosed) {
		t.Fatalf("submitted link accepted a second submit: %v", err)
	}
	if publicRequest, _, err := service.PublicDocumentRequest(secondToken); err != nil || publicRequest.Status != "submitted" {
		t.Fatalf("submitted link should return a closed summary, request=%#v err=%v", publicRequest, err)
	}
	if service.documentRequests[0].TokenHash == "" || service.documentRequests[0].TokenHash == secondToken {
		t.Fatalf("request token was not hashed: %#v", service.documentRequests[0])
	}
}

func TestVerificationRequiresAcceptedDocuments(t *testing.T) {
	service := kycTestService()
	reviewer := auth.User{ID: "user_owner", Role: "Owner", Email: "owner@example.com"}
	service.customers[0].Documents = []CustomerDocument{
		{ID: "front", Kind: "cnic_front", FileID: "file_front", Status: "accepted"},
		{ID: "back", Kind: "cnic_back", FileID: "file_back", Status: "submitted"},
		{ID: "selfie", Kind: "selfie", FileID: "file_selfie", Status: "accepted"},
	}

	if _, err := service.SetVerification("cust_test", VerificationRequest{Status: "verified"}, reviewer); !errors.Is(err, errVerificationNotReady) {
		t.Fatalf("unreviewed document should block verification, got %v", err)
	}
	if service.customers[0].Documents[1].Status != "submitted" {
		t.Fatal("verification attempt silently accepted an unreviewed document")
	}

	service.customers[0].Documents[1].Status = "accepted"
	service.customers[0].RequestedDocuments = []RequestedDocument{{ID: "ask_trade", Label: "Trade license", Status: "fulfilled"}}
	service.customers[0].Documents = append(service.customers[0].Documents, CustomerDocument{
		ID: "trade", Kind: "other", RequestID: "ask_trade", FileID: "file_trade", Status: "submitted",
	})
	if _, err := service.SetVerification("cust_test", VerificationRequest{Status: "verified"}, reviewer); !errors.Is(err, errVerificationNotReady) {
		t.Fatalf("unreviewed additional request should block verification, got %v", err)
	}
	service.customers[0].Documents[3].Status = "accepted"
	verified, err := service.SetVerification("cust_test", VerificationRequest{Status: "verified"}, reviewer)
	if err != nil {
		t.Fatalf("verify reviewed package: %v", err)
	}
	if verified.VerificationStatus != "verified" || verified.VerifiedAt == "" || verified.VerifiedByUserID != reviewer.ID {
		t.Fatalf("verification metadata is incomplete: %#v", verified)
	}
}

func TestKYCMetadataAndFilesRequireReviewPermission(t *testing.T) {
	service := kycTestService()
	service.customers[0].CNIC = "12345-1234567-1"
	service.customers[0].IdentityNote = "Sensitive note"
	service.customers[0].Documents = []CustomerDocument{{
		ID: "front", Kind: "cnic_front", FileID: "file_front", Status: "draft",
	}}

	staff := auth.User{ID: "support", Role: "Support Agent", Permissions: []string{"customers.manage"}}
	redacted := customerForActor(service.customers[0], staff)
	if redacted.CNIC != "" || redacted.IdentityNote != "" || len(redacted.Documents) != 0 {
		t.Fatalf("KYC metadata leaked to staff without review permission: %#v", redacted)
	}
	if service.CanAccessDocument(staff, "file_front") {
		t.Fatal("staff without kyc.review accessed KYC media")
	}

	reviewer := auth.User{ID: "reviewer", Role: "Manager", Permissions: []string{"kyc.review"}}
	if !service.CanAccessDocument(reviewer, "file_front") {
		t.Fatal("KYC reviewer could not access KYC media")
	}
	customer := auth.User{ID: "customer", Role: "Customer", CustomerID: "cust_test"}
	if !service.CanAccessDocument(customer, "file_front") {
		t.Fatal("customer could not preview own draft")
	}
	service.customers[0].Documents[0].Status = "submitted"
	if service.CanAccessDocument(customer, "file_front") {
		t.Fatal("customer could preview submitted KYC media")
	}
}

func TestLegacyFulfilledAskWithoutEvidenceIsWithdrawn(t *testing.T) {
	service := kycTestService()
	reviewer := auth.User{ID: "user_owner", Role: "Owner", Email: "owner@example.com"}
	service.customers[0].Documents = acceptedRequiredKYCDocuments()
	service.customers[0].RequestedDocuments = []RequestedDocument{{
		ID: "ask_ntn", Label: "NTN", Kind: "other", Status: "fulfilled",
	}}

	normalized := normalizeCustomer(service.customers[0])
	if normalized.RequestedDocuments[0].Status != "withdrawn" {
		t.Fatalf("stale fulfilled request should be withdrawn, got %#v", normalized.RequestedDocuments[0])
	}
	if normalized.VerificationStage != "ready_to_verify" {
		t.Fatalf("stale request blocked verification stage: %s", normalized.VerificationStage)
	}

	verified, err := service.SetVerification("cust_test", VerificationRequest{Status: "verified"}, reviewer)
	if err != nil {
		t.Fatalf("verify after stale request repair: %v", err)
	}
	if verified.VerificationStatus != "verified" || verified.RequestedDocuments[0].Status != "withdrawn" {
		t.Fatalf("verification did not preserve repaired history: %#v", verified)
	}
}

func TestSubmittedAdditionalEvidenceStaysInReviewUntilAccepted(t *testing.T) {
	service := kycTestService()
	reviewer := auth.User{ID: "user_owner", Role: "Owner", Email: "owner@example.com"}
	service.customers[0].Documents = append(acceptedRequiredKYCDocuments(), CustomerDocument{
		ID: "trade", Kind: "other", RequestID: "ask_trade", FileID: "file_trade", Status: "submitted",
	})
	service.customers[0].RequestedDocuments = []RequestedDocument{{
		ID: "ask_trade", Label: "Trade license", Kind: "other", Status: "fulfilled",
	}}

	normalized := normalizeCustomer(service.customers[0])
	if normalized.RequestedDocuments[0].Status != "submitted" || normalized.VerificationStage != "in_review" {
		t.Fatalf("submitted evidence should be in review: %#v", normalized)
	}
	if _, err := service.SetVerification("cust_test", VerificationRequest{Status: "verified"}, reviewer); !errors.Is(err, errVerificationNotReady) {
		t.Fatalf("unreviewed extra evidence should block verification, got %v", err)
	}

	reviewed, err := service.ReviewDocument("cust_test", "trade", ReviewDocumentPayload{Status: "accepted"}, reviewer)
	if err != nil {
		t.Fatalf("accept extra evidence: %v", err)
	}
	if reviewed.RequestedDocuments[0].Status != "accepted" || reviewed.VerificationStage != "ready_to_verify" {
		t.Fatalf("accepted evidence did not make the case ready: %#v", reviewed)
	}
	if _, err := service.SetVerification("cust_test", VerificationRequest{Status: "verified"}, reviewer); err != nil {
		t.Fatalf("verify reviewed package: %v", err)
	}
}

func TestReviewerCanWithdrawAdditionalRequirement(t *testing.T) {
	service := kycTestService()
	reviewer := auth.User{ID: "user_owner", Role: "Owner", Email: "owner@example.com"}
	service.customers[0].Documents = append(acceptedRequiredKYCDocuments(), CustomerDocument{
		ID: "ntn", Kind: "other", RequestID: "ask_ntn", FileID: "file_ntn", Status: "submitted",
	})
	service.customers[0].RequestedDocuments = []RequestedDocument{{
		ID: "ask_ntn", Label: "NTN", Kind: "other", Status: "submitted",
	}}

	withdrawn, err := service.WithdrawDocumentAsk("cust_test", "ask_ntn", reviewer)
	if err != nil {
		t.Fatalf("withdraw requirement: %v", err)
	}
	if withdrawn.RequestedDocuments[0].Status != "withdrawn" || withdrawn.VerificationStage != "ready_to_verify" {
		t.Fatalf("withdrawn evidence still affected live verification: %#v", withdrawn)
	}
	if len(withdrawn.Documents) != 4 {
		t.Fatalf("withdrawal should retain evidence for audit, got %d documents", len(withdrawn.Documents))
	}
	if _, err := service.SetVerification("cust_test", VerificationRequest{Status: "verified"}, reviewer); err != nil {
		t.Fatalf("verify after requirement withdrawal: %v", err)
	}
}

func TestCustomerCannotReopenSubmittedVerification(t *testing.T) {
	service := kycTestService()
	customer := auth.User{ID: "user_customer", Role: "Customer", CustomerID: "cust_test", Email: "customer@example.com"}
	service.customers[0].Documents = acceptedRequiredKYCDocuments()
	for i := range service.customers[0].Documents {
		service.customers[0].Documents[i].Status = "submitted"
	}
	service.customers[0].DocumentsSubmittedAt = "2026-09-05T08:00:00Z"
	service.customers[0].VerificationInviteOpen = true

	if _, err := service.OpenVerificationInvite("cust_test", OpenInvitePayload{}, customer); !errors.Is(err, errVerificationInReview) {
		t.Fatalf("submitted customer reopened verification, got %v", err)
	}
	if service.customers[0].VerificationStage != "in_review" || service.customers[0].VerificationInviteOpen {
		t.Fatalf("submitted case was not normalized to read-only review: %#v", service.customers[0])
	}
}

func acceptedRequiredKYCDocuments() []CustomerDocument {
	return []CustomerDocument{
		{ID: "front", Kind: "cnic_front", FileID: "file_front", Status: "accepted"},
		{ID: "back", Kind: "cnic_back", FileID: "file_back", Status: "accepted"},
		{ID: "selfie", Kind: "selfie", FileID: "file_selfie", Status: "accepted"},
	}
}

func kycTestService() *Service {
	service := NewService(nil)
	service.customers = []Customer{{
		ID:                 "cust_test",
		TenantID:           "tenant_test",
		CompanyName:        "Test Export House",
		VerificationStatus: "unverified",
		Documents:          []CustomerDocument{},
		RequestedDocuments: []RequestedDocument{},
	}}
	service.documentRequests = []DocumentRequest{}
	return service
}
