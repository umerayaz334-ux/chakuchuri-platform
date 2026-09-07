package workflow

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"chakuchuri/backend/internal/audit"
	"chakuchuri/backend/internal/auth"
	"chakuchuri/backend/internal/platform/config"
	"chakuchuri/backend/internal/platform/httpx"
)

type Product struct {
	ID          string `json:"id"`
	CustomerID  string `json:"customerId"`
	SKU         string `json:"sku"`
	Name        string `json:"name"`
	Stock       int    `json:"stock"`
	Reserved    int    `json:"reserved"`
	Image       string `json:"image"`
	ImageFileID string `json:"imageFileId,omitempty"`
	UpdatedAt   string `json:"updatedAt"`
}

type Quotation struct {
	ID              string   `json:"id"`
	CustomerID      string   `json:"customerId"`
	ProductID       string   `json:"productId,omitempty"`
	ProductName     string   `json:"productName"`
	Quantity        int      `json:"quantity"`
	ImageName       string   `json:"imageName,omitempty"`
	ImageFileID     string   `json:"imageFileId,omitempty"`
	ImageNames      []string `json:"imageNames,omitempty"`
	ImageFileIDs    []string `json:"imageFileIds,omitempty"`
	Notes           string   `json:"notes"`
	Steel           string   `json:"steel"`
	Tang            string   `json:"tang"`
	BladeThickness  string   `json:"bladeThickness"`
	HandleMaterial  string   `json:"handleMaterial"`
	Sheath          string   `json:"sheath"`
	Finish          string   `json:"finish"`
	Status          string   `json:"status"`
	TotalAmount     int      `json:"totalAmount"`
	DepositRequired int      `json:"depositRequired"`
	ExpectedDate    string   `json:"expectedDate"`
	AdminNote       string   `json:"adminNote"`
	CreatedAt       string   `json:"createdAt"`
	History         []Update `json:"history"`
}

type ManufacturingOrder struct {
	ID              string   `json:"id"`
	CustomerID      string   `json:"customerId"`
	QuotationID     string   `json:"quotationId"`
	ProductName     string   `json:"productName"`
	Quantity        int      `json:"quantity"`
	ImageName       string   `json:"imageName,omitempty"`
	ImageFileID     string   `json:"imageFileId,omitempty"`
	ImageNames      []string `json:"imageNames,omitempty"`
	ImageFileIDs    []string `json:"imageFileIds,omitempty"`
	Status          string   `json:"status"`
	CurrentStage    string   `json:"currentStage"`
	Progress        int      `json:"progress"`
	ExpectedDate    string   `json:"expectedDate"`
	TotalAmount     int      `json:"totalAmount"`
	DepositRequired int      `json:"depositRequired"`
	PaidAmount      int      `json:"paidAmount"`
	BalanceDue      int      `json:"balanceDue"`
	Stages          []Stage  `json:"stages"`
	History         []Update `json:"history"`
	CreatedAt       string   `json:"createdAt"`
}

type Stage struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Date   string `json:"date,omitempty"`
}

type RateSheet struct {
	ID           string `json:"id"`
	Courier      string `json:"courier"`
	Service      string `json:"service"`
	Zone         string `json:"zone"`
	Weight       string `json:"weight"`
	Price        int    `json:"price"`
	Status       string `json:"status"`
	SourceFileID string `json:"sourceFileId,omitempty"`
	SourceName   string `json:"sourceName,omitempty"`
}

type ShippingRequest struct {
	Pricing         *ShippingPricing `json:"pricing,omitempty"`
	ID              string           `json:"id"`
	CustomerID      string           `json:"customerId"`
	ManufacturingID string           `json:"manufacturingId,omitempty"`
	Type            string           `json:"type"`
	Courier         string           `json:"courier"`
	Service         string           `json:"service"`
	Destination     string           `json:"destination"`
	Zone            string           `json:"zone"`
	Weight          string           `json:"weight"`
	Status          string           `json:"status"`
	Tracking        string           `json:"tracking,omitempty"`
	QuotedAmount    int              `json:"quotedAmount"`
	CreatedAt       string           `json:"createdAt"`
	History         []Update         `json:"history"`
}

type Payment struct {
	IdempotencyKey  string              `json:"idempotencyKey,omitempty"`
	ID              string              `json:"id"`
	CustomerID      string              `json:"customerId"`
	ManufacturingID string              `json:"manufacturingId,omitempty"`
	ShippingID      string              `json:"shippingId,omitempty"`
	Type            string              `json:"type"`
	Amount          int                 `json:"amount"`
	Status          string              `json:"status"`
	ProofName       string              `json:"proofName"`
	ProofFileID     string              `json:"proofFileId,omitempty"`
	Note            string              `json:"note"`
	CreatedAt       string              `json:"createdAt"`
	ConfirmedAt     string              `json:"confirmedAt,omitempty"`
	ReversedAt      string              `json:"reversedAt,omitempty"`
	ReversalReason  string              `json:"reversalReason,omitempty"`
	Allocations     []PaymentAllocation `json:"allocations,omitempty"`
	CreditLeft      int                 `json:"creditLeft,omitempty"`
}

type LedgerEntry struct {
	ID         string `json:"id"`
	CustomerID string `json:"customerId"`
	EntryNo    string `json:"entryNo"`
	SourceType string `json:"sourceType"`
	SourceID   string `json:"sourceId"`
	Debit      int    `json:"debit"`
	Credit     int    `json:"credit"`
	Currency   string `json:"currency"`
	Note       string `json:"note"`
	PostedAt   string `json:"postedAt"`
}

type Conversation struct {
	ID                string    `json:"id"`
	CustomerID        string    `json:"customerId"`
	Subject           string    `json:"subject,omitempty"`
	Status            string    `json:"status"`
	LastMessageAt     string    `json:"lastMessageAt,omitempty"`
	LastAuthor        string    `json:"lastAuthor,omitempty"`
	UnreadForAdmin    int       `json:"unreadForAdmin"`
	UnreadForCustomer int       `json:"unreadForCustomer"`
	Messages          []Message `json:"messages"`
	MessagePagination PageInfo  `json:"messagePagination"`
}

type Message struct {
	ID          string              `json:"id"`
	Author      string              `json:"author"`
	AuthorRole  string              `json:"authorRole,omitempty"`
	Body        string              `json:"body"`
	Attachments []MessageAttachment `json:"attachments,omitempty"`
	CreatedAt   string              `json:"createdAt"`
}

type MessageAttachment struct {
	FileID       string `json:"fileId"`
	Name         string `json:"name"`
	MimeType     string `json:"mimeType"`
	ByteSize     int64  `json:"byteSize"`
	URL          string `json:"url"`
	ThumbnailURL string `json:"thumbnailUrl,omitempty"`
}

type CallRequest struct {
	ID                 string   `json:"id"`
	CustomerID         string   `json:"customerId"`
	ConversationID     string   `json:"conversationId,omitempty"`
	Subject            string   `json:"subject"`
	Status             string   `json:"status"`
	CallType           string   `json:"callType"`
	RoomID             string   `json:"roomId,omitempty"`
	InitiatorUserID    string   `json:"initiatorUserId,omitempty"`
	InitiatorName      string   `json:"initiatorName"`
	InitiatorRole      string   `json:"initiatorRole"`
	RecipientName      string   `json:"recipientName"`
	AnsweredByUserID   string   `json:"answeredByUserId,omitempty"`
	AnsweredBy         string   `json:"answeredBy,omitempty"`
	CreatedAt          string   `json:"createdAt"`
	UpdatedAt          string   `json:"updatedAt,omitempty"`
	RingExpiresAt      string   `json:"ringExpiresAt,omitempty"`
	StartedAt          string   `json:"startedAt,omitempty"`
	EndedAt            string   `json:"endedAt,omitempty"`
	DurationSeconds    int      `json:"durationSeconds"`
	CustomerLastSeenAt string   `json:"customerLastSeenAt,omitempty"`
	AdminLastSeenAt    string   `json:"adminLastSeenAt,omitempty"`
	History            []Update `json:"history"`
}

type CallSignal struct {
	ID         string                 `json:"id"`
	CallID     string                 `json:"callId"`
	SignalNo   int                    `json:"signalNo"`
	SenderID   string                 `json:"senderId"`
	SenderRole string                 `json:"senderRole"`
	SignalType string                 `json:"signalType"`
	Payload    map[string]interface{} `json:"payload"`
	CreatedAt  string                 `json:"createdAt"`
}

type Update struct {
	Label     string `json:"label"`
	Detail    string `json:"detail"`
	Actor     string `json:"actor"`
	CreatedAt string `json:"createdAt"`
}

type PlatformSettings struct {
	ThemePreset             string   `json:"themePreset"`
	PrimaryColor            string   `json:"primaryColor"`
	AccentColor             string   `json:"accentColor"`
	SurfaceColor            string   `json:"surfaceColor"`
	DefaultLanguage         string   `json:"defaultLanguage"`
	EnabledLanguages        []string `json:"enabledLanguages"`
	AllowUserLanguageChoice bool     `json:"allowUserLanguageChoice"`
	UpdatedAt               string   `json:"updatedAt,omitempty"`
	UpdatedBy               string   `json:"updatedBy,omitempty"`
}

type Dashboard struct {
	Products         []Product              `json:"products"`
	Quotations       []Quotation            `json:"quotations"`
	Manufacturing    []ManufacturingOrder   `json:"manufacturing"`
	RateSheets       []RateSheet            `json:"rateSheets"`
	Shipping         []ShippingRequest      `json:"shipping"`
	Payments         []Payment              `json:"payments"`
	Ledger           []LedgerEntry          `json:"ledger"`
	Conversations    []Conversation         `json:"conversations"`
	Calls            []CallRequest          `json:"calls"`
	Presence         []auth.UserPresence    `json:"presence"`
	Notices          []CustomerNotice       `json:"notices"`
	FeaturedProducts []FeaturedProduct      `json:"featuredProducts"`
	Metrics          map[string]interface{} `json:"metrics"`
	Pagination       map[string]PageInfo    `json:"pagination,omitempty"`
}

type PageInfo struct {
	Loaded  int  `json:"loaded"`
	Total   int  `json:"total"`
	HasMore bool `json:"hasMore"`
}

type WorkspacePage struct {
	Scope      string      `json:"scope"`
	Items      interface{} `json:"items"`
	Pagination PageInfo    `json:"pagination"`
}

type ConversationMessagePage struct {
	ConversationID string    `json:"conversationId"`
	Messages       []Message `json:"messages"`
	Pagination     PageInfo  `json:"pagination"`
}

type QuoteRequest struct {
	CustomerID   string   `json:"customerId"`
	ProductID    string   `json:"productId"`
	ProductName  string   `json:"productName"`
	Quantity     int      `json:"quantity"`
	ImageName    string   `json:"imageName"`
	ImageFileID  string   `json:"imageFileId,omitempty"`
	ImageNames   []string `json:"imageNames,omitempty"`
	ImageFileIDs []string `json:"imageFileIds,omitempty"`
	Notes        string   `json:"notes"`
}

type PriceQuoteRequest struct {
	ProductName     string `json:"productName"`
	UnitPrice       int    `json:"unitPrice"`
	TotalAmount     int    `json:"totalAmount"`
	DepositRequired int    `json:"depositRequired"`
	ExpectedDate    string `json:"expectedDate"`
	Steel           string `json:"steel"`
	Tang            string `json:"tang"`
	BladeThickness  string `json:"bladeThickness"`
	HandleMaterial  string `json:"handleMaterial"`
	Sheath          string `json:"sheath"`
	Finish          string `json:"finish"`
	AdminNote       string `json:"adminNote"`
}

type OrderUpdateRequest struct {
	Detail       string `json:"detail"`
	ExpectedDate string `json:"expectedDate"`
}

type OrderEditRequest struct {
	ProductName  string `json:"productName"`
	Quantity     int    `json:"quantity"`
	UnitPrice    int    `json:"unitPrice"`
	TotalAmount  int    `json:"totalAmount"`
	ExpectedDate string `json:"expectedDate"`
	Reason       string `json:"reason"`
	Confirmation string `json:"confirmation"`
}

type CancelOrderRequest struct {
	Reason       string `json:"reason"`
	Confirmation string `json:"confirmation"`
}

type PaymentAllocation struct {
	ShippingID      string `json:"shippingId,omitempty"`
	ManufacturingID string `json:"manufacturingId"`
	Amount          int    `json:"amount"`
}

type PaymentRequest struct {
	IdempotencyKey  string              `json:"idempotencyKey,omitempty"`
	CustomerID      string              `json:"customerId"`
	ManufacturingID string              `json:"manufacturingId"`
	ShippingID      string              `json:"shippingId"`
	Type            string              `json:"type"`
	Amount          int                 `json:"amount"`
	ProofName       string              `json:"proofName"`
	ProofFileID     string              `json:"proofFileId,omitempty"`
	Note            string              `json:"note"`
	Allocations     []PaymentAllocation `json:"allocations"`
}

type PaymentConfirmation struct {
	Payment Payment              `json:"payment"`
	Orders  []ManufacturingOrder `json:"orders,omitempty"`
	Ledger  *LedgerEntry         `json:"ledger,omitempty"`
}

type PaymentReversalRequest struct {
	Reason       string `json:"reason"`
	Confirmation string `json:"confirmation"`
}

type RateSheetRequest struct {
	Courier      string `json:"courier"`
	Service      string `json:"service"`
	Zone         string `json:"zone"`
	Weight       string `json:"weight"`
	Price        int    `json:"price"`
	Status       string `json:"status"`
	SourceFileID string `json:"sourceFileId"`
	SourceName   string `json:"sourceName"`
}
type ShippingRequestPayload struct {
	pricing         *ShippingPricing
	CustomerID      string `json:"customerId"`
	ManufacturingID string `json:"manufacturingId"`
	Type            string `json:"type"`
	Courier         string `json:"courier"`
	Service         string `json:"service"`
	Destination     string `json:"destination"`
	Zone            string `json:"zone"`
	Weight          string `json:"weight"`
}

type ProductRequest struct {
	CustomerID  string `json:"customerId"`
	SKU         string `json:"sku"`
	Name        string `json:"name"`
	Stock       int    `json:"stock"`
	Image       string `json:"image"`
	ImageFileID string `json:"imageFileId,omitempty"`
}

type MessageRequest struct {
	CustomerID    string   `json:"customerId"`
	Body          string   `json:"body"`
	AttachmentIDs []string `json:"attachmentIds"`
}

type CallRequestPayload struct {
	CustomerID     string `json:"customerId"`
	ConversationID string `json:"conversationId"`
	Subject        string `json:"subject"`
	RecipientName  string `json:"recipientName"`
}

type CallActionRequest struct {
	Status string `json:"status"`
	Note   string `json:"note"`
}

type CallSignalRequest struct {
	SignalType string                 `json:"signalType"`
	Payload    map[string]interface{} `json:"payload"`
}

type EmailEvent struct {
	EventKey   string
	Trigger    string
	CustomerID string
	EntityID   string
	Data       map[string]string
}

type Service struct {
	mu                  sync.RWMutex
	authService         *auth.Service
	recorder            *audit.Recorder
	repository          Repository
	workspaceEvents     *workspaceEventHub
	products            []Product
	quotations          []Quotation
	manufacturing       []ManufacturingOrder
	rateSheets          []RateSheet
	shipping            []ShippingRequest
	payments            []Payment
	ledger              []LedgerEntry
	conversations       []Conversation
	calls               []CallRequest
	callSignals         []CallSignal
	notices             []CustomerNotice
	featured            []FeaturedProduct
	settings            PlatformSettings
	attachmentResolver  func(string, auth.User) (MessageAttachment, bool)
	emailEventPublisher func(EmailEvent)
	webrtc              config.WebRTC
	lastSequenceDay     string
	sequence            int
	startOnce           sync.Once
}

func NewService(authService *auth.Service, recorder *audit.Recorder) *Service {
	now := time.Now().UTC()
	service := &Service{
		authService:     authService,
		recorder:        recorder,
		workspaceEvents: newWorkspaceEventHub(),
		settings:        defaultPlatformSettings(),
		webrtc:          config.Config{}.WebRTC(),
		products: []Product{
			{ID: "prod_chef_kit", CustomerID: "cust_abc_export", SKU: "KCH-1001", Name: "Chef knife export set", Stock: 140, Reserved: 22, Image: "chef-set.webp", UpdatedAt: now.Add(-6 * time.Hour).Format(time.RFC3339)},
			{ID: "prod_outdoor_axe", CustomerID: "cust_abc_export", SKU: "AXE-208", Name: "Outdoor axe", Stock: 36, Reserved: 8, Image: "axe.webp", UpdatedAt: now.Add(-18 * time.Hour).Format(time.RFC3339)},
			{ID: "prod_sword_410", CustomerID: "cust_northern_trading", SKU: "SWR-410", Name: "Decorative sword", Stock: 18, Reserved: 2, Image: "sword.webp", UpdatedAt: now.Add(-26 * time.Hour).Format(time.RFC3339)},
		},
		rateSheets: []RateSheet{
			{ID: "rate_fedex_z7_5kg", Courier: "FedEx", Service: "Duty paid premium", Zone: "7", Weight: "0-5 kg", Price: 18800, Status: "Active"},
			{ID: "rate_dhl_z7_5kg", Courier: "DHL", Service: "Express exporter", Zone: "7", Weight: "0-5 kg", Price: 20400, Status: "Active"},
			{ID: "rate_ups_z6_5kg", Courier: "UPS", Service: "Saver", Zone: "6", Weight: "0-5 kg", Price: 17600, Status: "Active"},
		},
	}
	service.seed(now)
	return service
}

func (s *Service) seed(now time.Time) {
	s.quotations = []Quotation{
		{
			ID: "Q-24091", CustomerID: "cust_abc_export", ProductID: "prod_chef_kit", ProductName: "Damascus chef knife export batch", Quantity: 500,
			ImageName: "chef-drawing.webp", Notes: "Customer requested satin finish and retail packaging.", Steel: "D2", Tang: "Full tang",
			BladeThickness: "3.2 mm", HandleMaterial: "Rosewood", Sheath: "Leather", Finish: "Satin",
			Status: "Accepted", TotalAmount: 420000, DepositRequired: 126000, ExpectedDate: "2026-09-15", AdminNote: "Material reserved.",
			CreatedAt: now.AddDate(0, 0, -10).Format(time.RFC3339),
			History:   []Update{{Label: "Quote accepted", Detail: "Customer accepted quotation and paid deposit.", Actor: "Customer", CreatedAt: now.AddDate(0, 0, -9).Format(time.RFC3339)}},
		},
		{
			ID: "Q-24094", CustomerID: "cust_abc_export", ProductName: "Custom camping axe sample", Quantity: 25, ImageName: "axe-sample.jpg",
			Notes: "Need export-ready packing and target price confirmation.", Status: "Requested", CreatedAt: now.Add(-4 * time.Hour).Format(time.RFC3339),
			History: []Update{{Label: "Quote requested", Detail: "Customer submitted a new quotation request.", Actor: "Customer", CreatedAt: now.Add(-4 * time.Hour).Format(time.RFC3339)}},
		},
	}
	s.manufacturing = []ManufacturingOrder{
		{
			ID: "MFG-24091", CustomerID: "cust_abc_export", QuotationID: "Q-24091", ProductName: "Damascus chef knife export batch", Quantity: 500,
			Status: "Production", CurrentStage: "Assembly", Progress: 64, ExpectedDate: "2026-09-15", TotalAmount: 420000,
			DepositRequired: 126000, PaidAmount: 126000, BalanceDue: 294000, CreatedAt: now.AddDate(0, 0, -9).Format(time.RFC3339),
			Stages: []Stage{
				{Name: "Order confirmed", Status: "done", Date: "2026-08-23"},
				{Name: "Deposit received", Status: "done", Date: "2026-08-24"},
				{Name: "Production started", Status: "done", Date: "2026-08-25"},
				{Name: "Assembly", Status: "active", Date: "2026-09-01"},
				{Name: "Quality check", Status: "pending"},
				{Name: "Ready for delivery", Status: "pending"},
			},
			History: []Update{
				{Label: "Assembly in progress", Detail: "Main assembly stage is active and on schedule.", Actor: "Production", CreatedAt: now.Add(-2 * time.Hour).Format(time.RFC3339)},
			},
		},
	}
	s.shipping = []ShippingRequest{
		{
			ID: "SHIP-8102", CustomerID: "cust_abc_export", ManufacturingID: "MFG-24091", Type: "Manufactured goods", Courier: "FedEx",
			Service: "Duty paid premium", Destination: "Texas, USA", Zone: "7", Weight: "4.8 kg", Status: "In transit",
			Tracking: "782100900021", QuotedAmount: 18800, CreatedAt: now.AddDate(0, 0, -2).Format(time.RFC3339),
			History: []Update{{Label: "Pickup done", Detail: "Parcel moved through export channel.", Actor: "Shipping", CreatedAt: now.AddDate(0, 0, -1).Format(time.RFC3339)}},
		},
	}
	s.payments = []Payment{
		{ID: "PAY-5001", CustomerID: "cust_abc_export", ManufacturingID: "MFG-24091", Type: "Deposit", Amount: 126000, Status: "Confirmed", ProofName: "deposit-receipt.jpg", CreatedAt: now.AddDate(0, 0, -9).Format(time.RFC3339), ConfirmedAt: now.AddDate(0, 0, -9).Add(2 * time.Hour).Format(time.RFC3339)},
		{ID: "PAY-5002", CustomerID: "cust_abc_export", ShippingID: "SHIP-8102", Type: "Shipping", Amount: 18800, Status: "Waiting confirmation", ProofName: "shipping-transfer.png", CreatedAt: now.Add(-90 * time.Minute).Format(time.RFC3339)},
	}
	s.conversations = []Conversation{
		{ID: "CONV-1001", CustomerID: "cust_abc_export", Status: "Open", Messages: []Message{
			{ID: "MSG-1", Author: "Customer", Body: "Please confirm duty paid premium for Zone 7.", CreatedAt: now.Add(-22 * time.Minute).Format(time.RFC3339)},
			{ID: "MSG-2", Author: "Admin", Body: "FedEx premium is active. Rate is Rs 18,800 for this weight band.", CreatedAt: now.Add(-18 * time.Minute).Format(time.RFC3339)},
		}},
	}
	s.calls = []CallRequest{
		{
			ID: "CALL-001", CustomerID: "cust_abc_export", ConversationID: "CONV-1001", Subject: "Shipping rate discussion",
			Status: "Completed", CallType: "audio", InitiatorUserID: "usr_customer", InitiatorName: "Customer",
			InitiatorRole: "Customer", RecipientName: "ChakuChuri Support", AnsweredBy: "ChakuChuri Admin",
			CreatedAt: now.Add(-22 * time.Minute).Format(time.RFC3339), UpdatedAt: now.Add(-12 * time.Minute).Format(time.RFC3339),
			StartedAt: now.Add(-21 * time.Minute).Format(time.RFC3339), EndedAt: now.Add(-12 * time.Minute).Format(time.RFC3339), DurationSeconds: 540,
		},
	}
	s.ledger = mergedLedgerEntries(nil, s.manufacturing, s.shipping, s.payments)
}

func defaultPlatformSettings() PlatformSettings {
	return PlatformSettings{
		ThemePreset:             "Jade",
		PrimaryColor:            "#0f766e",
		AccentColor:             "#c58b35",
		SurfaceColor:            "#f6f8f7",
		DefaultLanguage:         "en",
		EnabledLanguages:        []string{"en", "ur"},
		AllowUserLanguageChoice: true,
	}
}

func normalizePlatformSettings(value PlatformSettings) (PlatformSettings, error) {
	normalized := defaultPlatformSettings()
	presets := []string{"Jade", "Cobalt", "Graphite", "Burgundy", "Custom"}
	if preset := strings.TrimSpace(value.ThemePreset); preset != "" {
		normalized.ThemePreset = ""
		for _, option := range presets {
			if strings.EqualFold(option, preset) {
				normalized.ThemePreset = option
				break
			}
		}
		if normalized.ThemePreset == "" {
			return PlatformSettings{}, errValidation
		}
	}

	colors := []struct {
		value    string
		fallback string
		target   *string
	}{
		{value.PrimaryColor, normalized.PrimaryColor, &normalized.PrimaryColor},
		{value.AccentColor, normalized.AccentColor, &normalized.AccentColor},
		{value.SurfaceColor, normalized.SurfaceColor, &normalized.SurfaceColor},
	}
	for _, color := range colors {
		candidate := strings.ToLower(strings.TrimSpace(color.value))
		if candidate == "" {
			candidate = color.fallback
		}
		if !isHexColor(candidate) {
			return PlatformSettings{}, errValidation
		}
		*color.target = candidate
	}

	language := strings.ToLower(strings.TrimSpace(value.DefaultLanguage))
	if language == "" {
		language = normalized.DefaultLanguage
	}
	if language != "en" && language != "ur" {
		return PlatformSettings{}, errValidation
	}
	normalized.DefaultLanguage = language
	normalized.EnabledLanguages = []string{"en", "ur"}
	normalized.AllowUserLanguageChoice = value.AllowUserLanguageChoice
	normalized.UpdatedAt = strings.TrimSpace(value.UpdatedAt)
	normalized.UpdatedBy = strings.TrimSpace(value.UpdatedBy)
	return normalized, nil
}

func isHexColor(value string) bool {
	if len(value) != 7 || value[0] != '#' {
		return false
	}
	_, err := strconv.ParseUint(value[1:], 16, 24)
	return err == nil
}

func copyPlatformSettings(value PlatformSettings) PlatformSettings {
	value.EnabledLanguages = append([]string(nil), value.EnabledLanguages...)
	return value
}

func (s *Service) PlatformSettings() PlatformSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return copyPlatformSettings(s.settings)
}

func (s *Service) UpdatePlatformSettings(payload PlatformSettings, actor auth.User) (PlatformSettings, error) {
	if !auth.HasPermission(actor, "platform.settings.manage") {
		return PlatformSettings{}, errForbidden
	}
	next, err := normalizePlatformSettings(payload)
	if err != nil {
		return PlatformSettings{}, err
	}
	next.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	next.UpdatedBy = actorName(actor, actor.ID)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.settings = next
	s.persistLocked()
	s.record(actor, "platform.settings_updated", "platform", "Platform theme and language settings updated.")
	s.notifyWorkspaceUpsert("platform", "", copyPlatformSettings(next))
	return copyPlatformSettings(next), nil
}
func (s *Service) RemapCustomerID(fromID, toID string) int {
	fromID = strings.TrimSpace(fromID)
	toID = strings.TrimSpace(toID)
	if fromID == "" || toID == "" || fromID == toID {
		return 0
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	changed := 0
	rewrite := func(value *string) {
		if value != nil && *value == fromID {
			*value = toID
			changed++
		}
	}

	for i := range s.products {
		rewrite(&s.products[i].CustomerID)
	}
	for i := range s.quotations {
		rewrite(&s.quotations[i].CustomerID)
	}
	for i := range s.manufacturing {
		rewrite(&s.manufacturing[i].CustomerID)
	}
	for i := range s.shipping {
		rewrite(&s.shipping[i].CustomerID)
	}
	for i := range s.payments {
		rewrite(&s.payments[i].CustomerID)
	}
	for i := range s.ledger {
		rewrite(&s.ledger[i].CustomerID)
	}
	for i := range s.calls {
		rewrite(&s.calls[i].CustomerID)
	}

	// Merge conversations that would collide after remap.
	keep := make([]Conversation, 0, len(s.conversations))
	byCustomer := map[string]int{}
	for _, conversation := range s.conversations {
		if conversation.CustomerID == fromID {
			conversation.CustomerID = toID
			changed++
		}
		if existing, ok := byCustomer[conversation.CustomerID]; ok {
			keep[existing].Messages = append(keep[existing].Messages, conversation.Messages...)
			if conversation.LastMessageAt > keep[existing].LastMessageAt {
				keep[existing].LastMessageAt = conversation.LastMessageAt
				keep[existing].LastAuthor = conversation.LastAuthor
			}
			keep[existing].UnreadForAdmin += conversation.UnreadForAdmin
			keep[existing].UnreadForCustomer += conversation.UnreadForCustomer
			continue
		}
		byCustomer[conversation.CustomerID] = len(keep)
		keep = append(keep, conversation)
	}
	s.conversations = keep

	if changed > 0 {
		s.persistLocked()
	}
	return changed
}

func (s *Service) DashboardForUser(user auth.User) Dashboard {
	if s.authService != nil {
		s.authService.Heartbeat(user.ID)
	}

	// Keep auth + workspace notify outside s.mu to avoid AB-BA deadlocks with
	// StartCall / presence (workflow lock then auth lock) and auth notify paths.
	s.mu.Lock()
	expired := s.expireRingingCallsLocked(time.Now().UTC())
	dirty := len(expired) > 0

	allCustomers, allowedCustomers := auth.CustomerScope(user)
	// Orders and payments are persisted separately, so older data can contain
	// valid confirmed receipts with stale allocation/balance fields. Rebuild the
	// customer allocation view before copying it into any workspace response.
	// This keeps the ledger, order cards and customer credit on the same basis.
	accountCustomers := map[string]bool{}
	for _, order := range s.manufacturing {
		if allCustomers || allowedCustomers[order.CustomerID] {
			accountCustomers[order.CustomerID] = true
		}
	}
	for _, payment := range s.payments {
		if allCustomers || allowedCustomers[payment.CustomerID] {
			accountCustomers[payment.CustomerID] = true
		}
	}
	for customerID := range accountCustomers {
		if !s.customerOrderBalancesNeedSyncLocked(customerID) {
			continue
		}
		s.resyncCustomerOrderBalancesLocked(customerID, "", user, time.Now().UTC())
		dirty = true
		s.record(user, "accounting.balances_rebuilt", customerID, "Order balances and payment allocations rebuilt from confirmed receipts.")
	}
	if dirty {
		s.persistLocked()
	}

	products := filterProducts(s.products, allCustomers, allowedCustomers)
	quotes := filterQuotes(s.quotations, allCustomers, allowedCustomers)
	orders := filterOrders(s.manufacturing, allCustomers, allowedCustomers)
	shipping := filterShipping(s.shipping, allCustomers, allowedCustomers)
	payments := filterPayments(s.payments, allCustomers, allowedCustomers)
	ledger := filterLedger(s.ledger, allCustomers, allowedCustomers)
	conversations := filterConversations(s.conversations, allCustomers, allowedCustomers)
	calls := filterCalls(s.calls, allCustomers, allowedCustomers)
	rateSheets := append([]RateSheet(nil), s.rateSheets...)
	notices := filterNoticesForUser(append([]CustomerNotice(nil), s.notices...), user)
	featured := filterFeaturedForUser(append([]FeaturedProduct(nil), s.featured...), user)
	s.mu.Unlock()

	for _, call := range expired {
		s.notifyWorkspaceUpsert("calls", call.CustomerID, call)
	}

	presence := []auth.UserPresence{}
	if s.authService != nil {
		presence = s.authService.PresenceFor(user)
	}

	return Dashboard{
		Products:         products,
		Quotations:       quotes,
		Manufacturing:    orders,
		RateSheets:       rateSheets,
		Shipping:         shipping,
		Payments:         payments,
		Ledger:           ledger,
		Conversations:    conversations,
		Calls:            calls,
		Presence:         presence,
		Notices:          notices,
		FeaturedProducts: featured,
		Metrics:          buildMetrics(quotes, orders, shipping, payments, ledger, calls),
	}
}

const defaultWorkspacePageSize = 20
const maxWorkspacePageSize = 100

func pagedSlice[T any](rows []T, offset, limit int) ([]T, PageInfo) {
	if offset < 0 {
		offset = 0
	}
	if limit < 1 {
		limit = defaultWorkspacePageSize
	}
	if limit > maxWorkspacePageSize {
		limit = maxWorkspacePageSize
	}
	if offset > len(rows) {
		offset = len(rows)
	}
	end := offset + limit
	if end > len(rows) {
		end = len(rows)
	}
	page := append([]T(nil), rows[offset:end]...)
	return page, PageInfo{Loaded: end, Total: len(rows), HasMore: end < len(rows)}
}

func paginateDashboard(dashboard Dashboard, limit int) Dashboard {
	dashboard.Pagination = map[string]PageInfo{}
	dashboard.Products, dashboard.Pagination["products"] = pagedSlice(dashboard.Products, 0, limit)
	dashboard.Quotations, dashboard.Pagination["quotations"] = pagedSlice(dashboard.Quotations, 0, limit)
	dashboard.Manufacturing, dashboard.Pagination["manufacturing"] = pagedSlice(dashboard.Manufacturing, 0, limit)
	dashboard.RateSheets, dashboard.Pagination["rateSheets"] = pagedSlice(dashboard.RateSheets, 0, limit)
	dashboard.Shipping, dashboard.Pagination["shipping"] = pagedSlice(dashboard.Shipping, 0, limit)
	dashboard.Payments, dashboard.Pagination["payments"] = pagedSlice(dashboard.Payments, 0, limit)
	dashboard.Ledger, dashboard.Pagination["ledger"] = pagedSlice(dashboard.Ledger, 0, limit)
	dashboard.Conversations, dashboard.Pagination["conversations"] = pagedSlice(dashboard.Conversations, 0, limit)
	dashboard.Conversations = compactConversations(dashboard.Conversations, defaultWorkspacePageSize)
	dashboard.Calls, dashboard.Pagination["calls"] = pagedSlice(dashboard.Calls, 0, limit)
	return dashboard
}

func workspacePage(dashboard Dashboard, scope string, offset, limit int) (WorkspacePage, bool) {
	var items interface{}
	var page PageInfo
	switch scope {
	case "products":
		items, page = pagedSlice(dashboard.Products, offset, limit)
	case "quotations":
		items, page = pagedSlice(dashboard.Quotations, offset, limit)
	case "manufacturing":
		items, page = pagedSlice(dashboard.Manufacturing, offset, limit)
	case "rateSheets":
		items, page = pagedSlice(dashboard.RateSheets, offset, limit)
	case "shipping":
		items, page = pagedSlice(dashboard.Shipping, offset, limit)
	case "payments":
		items, page = pagedSlice(dashboard.Payments, offset, limit)
	case "ledger":
		items, page = pagedSlice(dashboard.Ledger, offset, limit)
	case "conversations":
		conversations, conversationPage := pagedSlice(dashboard.Conversations, offset, limit)
		items = compactConversations(conversations, defaultWorkspacePageSize)
		page = conversationPage
	case "calls":
		items, page = pagedSlice(dashboard.Calls, offset, limit)
	default:
		return WorkspacePage{}, false
	}
	return WorkspacePage{Scope: scope, Items: items, Pagination: page}, true
}

func (s *Service) CreateQuote(payload QuoteRequest, actor auth.User) (Quotation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.persistLocked()

	customerID := firstNonEmpty(payload.CustomerID, actor.CustomerID, "cust_abc_export")
	if !auth.CanAccessCustomer(actor, customerID) {
		return Quotation{}, errForbidden
	}
	if payload.Quantity < 1 {
		return Quotation{}, errValidation
	}

	imageIDs := quoteReferenceIDs(payload)
	imageNames, err := s.resolveQuoteReferencesLocked(imageIDs, actor)
	if err != nil {
		return Quotation{}, err
	}
	name := strings.TrimSpace(payload.ProductName)
	if name == "" {
		name = s.productName(payload.ProductID)
	}
	if name == "" {
		name = "Product consultation"
	}

	now := time.Now().UTC()
	quote := Quotation{
		ID:           s.nextID("Q"),
		CustomerID:   customerID,
		ProductID:    strings.TrimSpace(payload.ProductID),
		ProductName:  name,
		Quantity:     payload.Quantity,
		ImageName:    imageNames[0],
		ImageFileID:  imageIDs[0],
		ImageNames:   imageNames,
		ImageFileIDs: imageIDs,
		Notes:        strings.TrimSpace(payload.Notes),
		Status:       "Requested",
		CreatedAt:    now.Format(time.RFC3339),
		History: []Update{{
			Label:     "Consultation requested",
			Detail:    fmt.Sprintf("Customer shared %d product photo(s) for %d pieces.", len(imageIDs), payload.Quantity),
			Actor:     actorName(actor, "Customer"),
			CreatedAt: now.Format(time.RFC3339),
		}},
	}
	s.quotations = append([]Quotation{quote}, s.quotations...)
	s.record(actor, "quotes.requested", quote.ID, fmt.Sprintf("Quote consultation requested with %d product photo(s).", len(imageIDs)))
	s.notifyWorkspaceUpsert("quotations", quote.CustomerID, quote)
	return quote, nil
}

func (s *Service) PriceQuote(id string, payload PriceQuoteRequest, actor auth.User) (Quotation, error) {
	s.mu.Lock()

	for index := range s.quotations {
		if s.quotations[index].ID != id {
			continue
		}
		quote := &s.quotations[index]
		if !auth.CanAccessCustomer(actor, quote.CustomerID) {
			s.mu.Unlock()
			return Quotation{}, errForbidden
		}
		if quote.Status != "Requested" && quote.Status != "Priced" {
			s.mu.Unlock()
			return Quotation{}, errValidation
		}
		totalAmount := payload.TotalAmount
		if payload.UnitPrice > 0 {
			if quote.Quantity < 1 || payload.UnitPrice > 2_000_000_000/quote.Quantity {
				s.mu.Unlock()
				return Quotation{}, errValidation
			}
			totalAmount = payload.UnitPrice * quote.Quantity
		}
		if totalAmount < 1 {
			s.mu.Unlock()
			return Quotation{}, errValidation
		}
		expectedDate := strings.TrimSpace(payload.ExpectedDate)
		if expectedDate != "" {
			if _, err := time.Parse("2006-01-02", expectedDate); err != nil {
				s.mu.Unlock()
				return Quotation{}, errValidation
			}
		}
		if productName := strings.TrimSpace(payload.ProductName); productName != "" {
			quote.ProductName = productName
		}
		if strings.TrimSpace(quote.ProductName) == "" || quote.ProductName == "Product consultation" {
			s.mu.Unlock()
			return Quotation{}, errValidation
		}

		quote.TotalAmount = totalAmount
		deposit := payload.DepositRequired
		if deposit < 0 || deposit > totalAmount {
			s.mu.Unlock()
			return Quotation{}, errValidation
		}
		quote.DepositRequired = deposit
		quote.ExpectedDate = expectedDate
		quote.Steel = firstNonEmpty(payload.Steel, "D2")
		quote.Tang = firstNonEmpty(payload.Tang, "Full tang")
		quote.BladeThickness = firstNonEmpty(payload.BladeThickness, "3.0 mm")
		quote.HandleMaterial = firstNonEmpty(payload.HandleMaterial, "Customer choice")
		quote.Sheath = firstNonEmpty(payload.Sheath, "Leather")
		quote.Finish = firstNonEmpty(payload.Finish, "Satin")
		quote.AdminNote = strings.TrimSpace(payload.AdminNote)
		quote.Status = "Priced"
		quote.History = append([]Update{{
			Label:     "Formal quote prepared",
			Detail:    "Product specifications, price and production schedule were confirmed after consultation.",
			Actor:     actorName(actor, "Admin"),
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
		}}, quote.History...)
		s.record(actor, "quotes.priced", id, "Formal quote prepared after product consultation.")
		result := *quote
		emailEvent := EmailEvent{
			EventKey:   fmt.Sprintf("quote:%s:priced:%d", quote.ID, len(quote.History)),
			Trigger:    "quote.priced",
			CustomerID: quote.CustomerID,
			EntityID:   quote.ID,
			Data: map[string]string{
				"quote_id": quote.ID, "product_name": quote.ProductName, "quantity": strconv.Itoa(quote.Quantity),
				"unit_price":   fmt.Sprintf("PKR %d", unitPriceForQuote(quote.TotalAmount, quote.Quantity)),
				"total_amount": fmt.Sprintf("PKR %d", quote.TotalAmount), "expected_date": quote.ExpectedDate,
			},
		}
		s.persistLocked()
		s.mu.Unlock()

		s.notifyWorkspaceUpsert("quotations", result.CustomerID, result)
		s.publishEmailEvent(emailEvent)
		return result, nil
	}
	s.mu.Unlock()
	return Quotation{}, errNotFound
}

func (s *Service) AcceptQuote(id string, actor auth.User) (ManufacturingOrder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	mutation, err := s.beginFinancialMutationLocked()
	if err != nil {
		return ManufacturingOrder{}, err
	}
	defer mutation.rollback()

	for index := range s.quotations {
		if s.quotations[index].ID != id {
			continue
		}
		quote := s.quotations[index]
		if !auth.CanAccessCustomer(actor, quote.CustomerID) {
			return ManufacturingOrder{}, errForbidden
		}
		if quote.Status != "Priced" && quote.Status != "Accepted" {
			return ManufacturingOrder{}, errValidation
		}
		// One public code: the quotation id becomes the manufacturing order id.
		if order, ok := findOrderByQuote(s.manufacturing, quote.ID); ok {
			return order, nil
		}
		if order, ok := findOrderByID(s.manufacturing, quote.ID); ok {
			return order, nil
		}
		now := time.Now().UTC()
		depositStatus := "pending"
		status := "Deposit pending"
		progress := 28
		if quote.DepositRequired == 0 {
			depositStatus = "done"
			status = "Confirmed"
			progress = 43
		}
		order := ManufacturingOrder{
			ID:              quote.ID,
			CustomerID:      quote.CustomerID,
			QuotationID:     quote.ID,
			ProductName:     quote.ProductName,
			Quantity:        quote.Quantity,
			ImageName:       quote.ImageName,
			ImageFileID:     quote.ImageFileID,
			ImageNames:      append([]string(nil), quote.ImageNames...),
			ImageFileIDs:    append([]string(nil), quote.ImageFileIDs...),
			Status:          status,
			CurrentStage:    "Awaiting deposit",
			Progress:        progress,
			ExpectedDate:    quote.ExpectedDate,
			TotalAmount:     quote.TotalAmount,
			DepositRequired: quote.DepositRequired,
			PaidAmount:      0,
			BalanceDue:      quote.TotalAmount,
			CreatedAt:       now.Format(time.RFC3339),
			Stages: []Stage{
				{Name: "Quote accepted", Status: "done", Date: now.Format("2006-01-02")},
				{Name: "Deposit received", Status: depositStatus},
				{Name: "Production started", Status: "pending"},
				{Name: "Quality check", Status: "pending"},
				{Name: "Ready for delivery", Status: "pending"},
				{Name: "Completed", Status: "pending"},
			},
			History: []Update{{Label: "Order created", Detail: "Customer accepted the quotation.", Actor: actorName(actor, "Customer"), CreatedAt: now.Format(time.RFC3339)}},
		}
		if quote.DepositRequired == 0 {
			order.CurrentStage = "Starting Production"
			order.History[0].Detail = "Customer proceeded to order. No deposit was required."
		}
		s.quotations[index].Status = "Accepted"
		s.quotations[index].History = append([]Update{{Label: "Quote accepted", Detail: "Customer proceeded to order.", Actor: actorName(actor, "Customer"), CreatedAt: now.Format(time.RFC3339)}}, s.quotations[index].History...)
		s.manufacturing = append([]ManufacturingOrder{order}, s.manufacturing...)
		s.postLedgerEntryLocked(actor, order.CustomerID, "manufacturing_order", order.ID, order.TotalAmount, 0, "Manufacturing order total posted.", now.Format(time.RFC3339))
		if err := mutation.commit(actor); err != nil {
			return ManufacturingOrder{}, err
		}
		s.record(actor, "quotes.accepted", id, "Quote accepted and order created.")
		changes := []WorkspaceChange{
			workspaceUpsert("quotations", quote.CustomerID, s.quotations[index]),
			workspaceUpsert("manufacturing", order.CustomerID, order),
		}
		for _, entry := range s.ledger {
			if entry.SourceType == "manufacturing_order" && entry.SourceID == order.ID {
				changes = append(changes, workspaceUpsert("ledger", order.CustomerID, entry))
				break
			}
		}
		s.notifyWorkspaceChanges(changes...)
		s.publishEmailEvent(EmailEvent{
			EventKey: "quote:" + quote.ID + ":accepted", Trigger: "quote.accepted",
			CustomerID: order.CustomerID, EntityID: quote.ID,
			Data: map[string]string{
				"quote_id": quote.ID, "order_id": order.ID, "product_name": order.ProductName,
				"quantity": strconv.Itoa(order.Quantity), "total_amount": fmt.Sprintf("PKR %d", order.TotalAmount),
				"expected_date": order.ExpectedDate, "balance_due": fmt.Sprintf("PKR %d", order.BalanceDue),
			},
		})
		if order.BalanceDue > 0 {
			s.publishEmailEvent(EmailEvent{
				EventKey: fmt.Sprintf("order:%s:payment-due:%d", order.ID, now.UnixNano()), Trigger: "payment.due",
				CustomerID: order.CustomerID, EntityID: order.ID,
				Data: map[string]string{
					"order_id": order.ID, "product_name": order.ProductName,
					"balance_due": fmt.Sprintf("PKR %d", order.BalanceDue),
				},
			})
		}
		return order, nil
	}
	return ManufacturingOrder{}, errNotFound
}

func (s *Service) RejectQuote(id string, actor auth.User) (Quotation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.persistLocked()
	for index := range s.quotations {
		if s.quotations[index].ID == id {
			if !auth.CanAccessCustomer(actor, s.quotations[index].CustomerID) {
				return Quotation{}, errForbidden
			}
			s.quotations[index].Status = "Rejected"
			s.quotations[index].History = append([]Update{{
				Label:     "Quote rejected",
				Detail:    "Customer rejected the quotation.",
				Actor:     actorName(actor, "Customer"),
				CreatedAt: time.Now().UTC().Format(time.RFC3339),
			}}, s.quotations[index].History...)
			s.record(actor, "quotes.rejected", id, "Quote rejected.")
			s.notifyWorkspaceUpsert("quotations", s.quotations[index].CustomerID, s.quotations[index])
			return s.quotations[index], nil
		}
	}
	return Quotation{}, errNotFound
}

func (s *Service) MoveOrder(id string, stage string, actor auth.User) (ManufacturingOrder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	mutation, err := s.beginFinancialMutationLocked()
	if err != nil {
		return ManufacturingOrder{}, err
	}
	defer mutation.rollback()

	canonicalStage, ok := canonicalOrderStage(stage)
	if !ok {
		return ManufacturingOrder{}, errValidation
	}
	for index := range s.manufacturing {
		if s.manufacturing[index].ID != id {
			continue
		}
		if !auth.CanAccessCustomer(actor, s.manufacturing[index].CustomerID) {
			return ManufacturingOrder{}, errForbidden
		}
		if orderIsClosed(s.manufacturing[index]) {
			return ManufacturingOrder{}, errOrderClosed
		}
		order := &s.manufacturing[index]
		if err := s.requireAccountingReadyLocked(order.CustomerID); err != nil {
			return ManufacturingOrder{}, err
		}

		if outstanding := depositOutstanding(*order); outstanding > 0 {
			return ManufacturingOrder{}, &orderPaymentRequiredError{Kind: "deposit", Amount: outstanding}
		}
		if canonicalStage == "Completed" && order.BalanceDue > 0 {
			return ManufacturingOrder{}, &orderPaymentRequiredError{Kind: "balance", Amount: order.BalanceDue}
		}
		now := time.Now().UTC()
		applyOrderStage(order, canonicalStage, now)
		order.History = append([]Update{{
			Label:     "Production stage updated",
			Detail:    "Order moved to " + canonicalStage + ".",
			Actor:     actorName(actor, "Admin"),
			CreatedAt: now.Format(time.RFC3339),
		}}, order.History...)
		if err := mutation.commit(actor); err != nil {
			return ManufacturingOrder{}, err
		}
		s.record(actor, "manufacturing.stage_changed", id, "Manufacturing order stage updated.")
		s.notifyWorkspaceUpsert("manufacturing", order.CustomerID, *order)
		if canonicalStage == "Ready for delivery" {
			s.publishEmailEvent(EmailEvent{
				EventKey: "order:" + order.ID + ":ready", Trigger: "order.ready",
				CustomerID: order.CustomerID, EntityID: order.ID,
				Data: map[string]string{
					"order_id": order.ID, "product_name": order.ProductName,
					"balance_due": fmt.Sprintf("PKR %d", order.BalanceDue),
				},
			})
		}
		return *order, nil
	}
	return ManufacturingOrder{}, errNotFound
}

func (s *Service) EditOrder(id string, payload OrderEditRequest, actor auth.User) (ManufacturingOrder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	mutation, err := s.beginFinancialMutationLocked()
	if err != nil {
		return ManufacturingOrder{}, err
	}
	defer mutation.rollback()

	productName := strings.TrimSpace(payload.ProductName)
	expectedDate := strings.TrimSpace(payload.ExpectedDate)
	reason := strings.TrimSpace(payload.Reason)
	if strings.TrimSpace(payload.Confirmation) != strings.TrimSpace(id) {
		return ManufacturingOrder{}, errConfirmationRequired
	}
	if productName == "" || payload.Quantity < 1 || payload.UnitPrice < 1 || expectedDate == "" || reason == "" {
		return ManufacturingOrder{}, errValidation
	}
	if payload.Quantity > 1_000_000 || payload.UnitPrice > 2_000_000_000/payload.Quantity {
		return ManufacturingOrder{}, errValidation
	}
	if _, err := time.Parse("2006-01-02", expectedDate); err != nil {
		return ManufacturingOrder{}, errValidation
	}
	totalAmount := payload.Quantity * payload.UnitPrice

	for index := range s.manufacturing {
		order := &s.manufacturing[index]
		if order.ID != id {
			continue
		}
		if !auth.CanAccessCustomer(actor, order.CustomerID) {
			return ManufacturingOrder{}, errForbidden
		}
		if err := s.requireAccountingReadyLocked(order.CustomerID); err != nil {
			return ManufacturingOrder{}, err
		}
		if orderIsClosed(*order) {
			return ManufacturingOrder{}, errOrderClosed
		}
		if order.ProductName == productName && order.Quantity == payload.Quantity && order.TotalAmount == totalAmount && order.ExpectedDate == expectedDate {
			return ManufacturingOrder{}, errValidation
		}

		oldProductName := order.ProductName
		oldQuantity := order.Quantity
		oldTotal := order.TotalAmount
		oldDate := order.ExpectedDate
		now := time.Now().UTC()
		order.ProductName = productName
		order.Quantity = payload.Quantity
		order.TotalAmount = totalAmount
		order.ExpectedDate = expectedDate
		if order.DepositRequired > totalAmount {
			order.DepositRequired = totalAmount
		}
		if order.PaidAmount > totalAmount {
			order.PaidAmount = totalAmount
		}
		order.BalanceDue = totalAmount - order.PaidAmount
		if order.BalanceDue < 0 {
			order.BalanceDue = 0
		}
		order.History = append([]Update{{
			Label:     "Order details amended",
			Detail:    fmt.Sprintf("Verified change: %s; %s x %d (%d total) became %s x %d (%d total); expected date %s became %s.", reason, oldProductName, oldQuantity, oldTotal, productName, payload.Quantity, totalAmount, oldDate, expectedDate),
			Actor:     actorName(actor, "Admin"),
			CreatedAt: now.Format(time.RFC3339),
		}}, order.History...)

		changes := []WorkspaceChange{workspaceUpsert("manufacturing", order.CustomerID, *order)}
		difference := totalAmount - oldTotal
		if difference != 0 {
			sourceID := order.ID + "-" + now.Format("20060102T150405.000000000")
			note := "Verified order value adjustment: " + reason
			if difference > 0 {
				s.postLedgerEntryLocked(actor, order.CustomerID, "manufacturing_adjustment", sourceID, difference, 0, note, now.Format(time.RFC3339))
			} else {
				s.postLedgerEntryLocked(actor, order.CustomerID, "manufacturing_adjustment", sourceID, 0, -difference, note, now.Format(time.RFC3339))
			}
			if entry, ok := findLedgerEntry(s.ledger, "manufacturing_adjustment", sourceID); ok {
				changes = append(changes, workspaceUpsert("ledger", order.CustomerID, entry))
			}
		}
		// Rebuild open-order allocation after value change so PaidAmount/CreditLeft stay consistent.
		touched := s.resyncCustomerOrderBalancesLocked(order.CustomerID, "", actor, now)
		for _, row := range touched {
			changes = append(changes, workspaceUpsert("manufacturing", row.CustomerID, row))
		}
		for _, payment := range s.payments {
			if payment.CustomerID != order.CustomerID {
				continue
			}
			copyPayment := payment
			changes = append(changes, workspaceUpsert("payments", payment.CustomerID, copyPayment))
		}
		if err := mutation.commit(actor); err != nil {
			return ManufacturingOrder{}, err
		}
		s.record(actor, "manufacturing.order_amended", id, "Order details changed after typed-ID verification. Reason: "+reason)
		s.notifyWorkspaceChanges(changes...)
		s.publishEmailEvent(EmailEvent{
			EventKey: "order:" + order.ID + ":amended:" + now.Format("20060102T150405.000000000"),
			Trigger:  "order.amended", CustomerID: order.CustomerID, EntityID: order.ID,
			Data: map[string]string{
				"order_id": order.ID, "product_name": order.ProductName, "quantity": strconv.Itoa(order.Quantity),
				"total_amount": fmt.Sprintf("PKR %d", order.TotalAmount), "expected_date": order.ExpectedDate,
				"change_reason": reason,
			},
		})
		// Return the post-resync order snapshot.
		for _, row := range s.manufacturing {
			if row.ID == id {
				return row, nil
			}
		}
		return *order, nil
	}
	return ManufacturingOrder{}, errNotFound
}

func (s *Service) CancelOrder(id string, payload CancelOrderRequest, actor auth.User) (ManufacturingOrder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	mutation, err := s.beginFinancialMutationLocked()
	if err != nil {
		return ManufacturingOrder{}, err
	}
	defer mutation.rollback()

	reason := strings.TrimSpace(payload.Reason)
	if strings.TrimSpace(payload.Confirmation) != strings.TrimSpace(id) {
		return ManufacturingOrder{}, errConfirmationRequired
	}
	if reason == "" {
		return ManufacturingOrder{}, errValidation
	}

	for index := range s.manufacturing {
		order := &s.manufacturing[index]
		if order.ID != id {
			continue
		}
		if !auth.CanAccessCustomer(actor, order.CustomerID) {
			return ManufacturingOrder{}, errForbidden
		}
		if err := s.requireAccountingReadyLocked(order.CustomerID); err != nil {
			return ManufacturingOrder{}, err
		}
		if strings.EqualFold(order.Status, "Completed") {
			return ManufacturingOrder{}, errOrderClosed
		}
		if strings.EqualFold(order.Status, "Cancelled") {
			return *order, nil
		}

		now := time.Now().UTC()
		order.Status = "Cancelled"
		order.CurrentStage = "Cancelled"
		order.BalanceDue = 0
		order.Progress = 0
		for stageIndex := range order.Stages {
			if !strings.EqualFold(order.Stages[stageIndex].Status, "done") && !strings.EqualFold(order.Stages[stageIndex].Status, "completed") {
				order.Stages[stageIndex].Status = "cancelled"
			}
		}
		creditNotice := "No confirmed payment was attached to this order."
		if order.PaidAmount > 0 {
			creditNotice = fmt.Sprintf("Confirmed payments of %d remain as customer credit until refunded or reallocated.", order.PaidAmount)
		}
		order.History = append([]Update{{
			Label:     "Order cancelled",
			Detail:    "Verified cancellation: " + reason + ". The order charge was reversed. " + creditNotice,
			Actor:     actorName(actor, "Admin"),
			CreatedAt: now.Format(time.RFC3339),
		}}, order.History...)

		changes := []WorkspaceChange{workspaceUpsert("manufacturing", order.CustomerID, *order)}
		sourceID := order.ID
		s.postLedgerEntryLocked(actor, order.CustomerID, "manufacturing_cancellation", sourceID, 0, order.TotalAmount, "Manufacturing order charge reversed after verified cancellation: "+reason, now.Format(time.RFC3339))
		if entry, ok := findLedgerEntry(s.ledger, "manufacturing_cancellation", sourceID); ok {
			changes = append(changes, workspaceUpsert("ledger", order.CustomerID, entry))
		}
		for paymentIndex := range s.payments {
			payment := &s.payments[paymentIndex]
			if payment.ManufacturingID != order.ID || strings.EqualFold(payment.Status, "Confirmed") || strings.EqualFold(payment.Status, "Cancelled") {
				continue
			}
			payment.Status = "Cancelled"
			payment.Note = firstNonEmpty(payment.Note, "Cancelled because the related manufacturing order was cancelled.")
			changes = append(changes, workspaceUpsert("payments", payment.CustomerID, *payment))
		}
		// Release allocations that targeted this cancelled order onto remaining open balances / credit.
		for paymentIndex := range s.payments {
			payment := &s.payments[paymentIndex]
			if payment.CustomerID != order.CustomerID || payment.Status != "Confirmed" || strings.TrimSpace(payment.ShippingID) != "" {
				continue
			}
			kept := make([]PaymentAllocation, 0, len(payment.Allocations))
			for _, allocation := range payment.Allocations {
				if allocation.ManufacturingID == order.ID {
					continue
				}
				kept = append(kept, allocation)
			}
			payment.Allocations = kept
			if payment.ManufacturingID == order.ID {
				payment.ManufacturingID = ""
			}
		}
		touched := s.resyncCustomerOrderBalancesLocked(order.CustomerID, "", actor, now)
		for _, row := range touched {
			changes = append(changes, workspaceUpsert("manufacturing", row.CustomerID, row))
		}
		for _, payment := range s.payments {
			if payment.CustomerID != order.CustomerID {
				continue
			}
			copyPayment := payment
			changes = append(changes, workspaceUpsert("payments", payment.CustomerID, copyPayment))
		}
		if err := mutation.commit(actor); err != nil {
			return ManufacturingOrder{}, err
		}
		s.record(actor, "manufacturing.order_cancelled", id, "Order cancelled after typed-ID verification; accounting charge reversed. Reason: "+reason)
		s.notifyWorkspaceChanges(changes...)
		s.publishEmailEvent(EmailEvent{
			EventKey: "order:" + order.ID + ":cancelled", Trigger: "order.cancelled",
			CustomerID: order.CustomerID, EntityID: order.ID,
			Data: map[string]string{
				"order_id": order.ID, "product_name": order.ProductName,
				"total_amount":        fmt.Sprintf("PKR %d", order.TotalAmount),
				"customer_credit":     fmt.Sprintf("PKR %d", order.PaidAmount),
				"cancellation_reason": reason,
			},
		})
		for _, row := range s.manufacturing {
			if row.ID == id {
				return row, nil
			}
		}
		return *order, nil
	}
	return ManufacturingOrder{}, errNotFound
}

func (s *Service) UpdateOrder(id string, payload OrderUpdateRequest, actor auth.User) (ManufacturingOrder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	mutation, err := s.beginFinancialMutationLocked()
	if err != nil {
		return ManufacturingOrder{}, err
	}
	defer mutation.rollback()

	detail := strings.TrimSpace(payload.Detail)
	expectedDate := strings.TrimSpace(payload.ExpectedDate)
	if expectedDate != "" {
		if _, err := time.Parse("2006-01-02", expectedDate); err != nil {
			return ManufacturingOrder{}, errValidation
		}
	}
	for index := range s.manufacturing {
		order := &s.manufacturing[index]
		if order.ID != id {
			continue
		}
		if !auth.CanAccessCustomer(actor, order.CustomerID) {
			return ManufacturingOrder{}, errForbidden
		}
		if orderIsClosed(*order) {
			return ManufacturingOrder{}, errOrderClosed
		}
		dateChanged := expectedDate != "" && expectedDate != order.ExpectedDate
		if detail == "" && !dateChanged {
			return ManufacturingOrder{}, errValidation
		}
		if dateChanged {
			order.ExpectedDate = expectedDate
		}
		label := "Production update"
		if detail == "" {
			label = "Expected date updated"
			detail = "Expected completion date changed to " + expectedDate + "."
		} else if dateChanged {
			detail += " Expected completion: " + expectedDate + "."
		}
		now := time.Now().UTC().Format(time.RFC3339)
		order.History = append([]Update{{Label: label, Detail: detail, Actor: actorName(actor, "Admin"), CreatedAt: now}}, order.History...)
		if err := mutation.commit(actor); err != nil {
			return ManufacturingOrder{}, err
		}
		s.record(actor, "manufacturing.updated", id, "Production update published.")
		s.notifyWorkspaceUpsert("manufacturing", order.CustomerID, *order)
		return *order, nil
	}
	return ManufacturingOrder{}, errNotFound
}

func canonicalOrderStage(value string) (string, bool) {
	for _, stage := range []string{"Confirmed", "Production", "Quality check", "Ready for delivery", "Completed"} {
		if strings.EqualFold(strings.TrimSpace(value), stage) {
			return stage, true
		}
	}
	return "", false
}

func applyOrderStage(order *ManufacturingOrder, stage string, now time.Time) {
	progressByStage := map[string]int{
		"Confirmed":          43,
		"Production":         64,
		"Quality check":      78,
		"Ready for delivery": 86,
		"Completed":          100,
	}
	timelineName := map[string]string{
		"Confirmed":          "Deposit received",
		"Production":         "Production started",
		"Quality check":      "Quality check",
		"Ready for delivery": "Ready for delivery",
		"Completed":          "Completed",
	}[stage]
	targetIndex := -1
	for index := range order.Stages {
		if strings.EqualFold(order.Stages[index].Name, timelineName) {
			targetIndex = index
			break
		}
	}
	stamp := now.Format("2006-01-02")
	for index := range order.Stages {
		item := &order.Stages[index]
		switch {
		case stage == "Completed" || index < targetIndex || (stage == "Confirmed" && index == targetIndex):
			item.Status = "done"
		case index == targetIndex:
			item.Status = "active"
		default:
			item.Status = "pending"
		}
		if index == targetIndex && item.Date == "" {
			item.Date = stamp
		}
	}
	order.Status = stage
	order.CurrentStage = timelineName
	if stage == "Confirmed" {
		order.CurrentStage = "Confirmed"
	}
	order.Progress = progressByStage[stage]
}

func (s *Service) SaveRateSheet(payload RateSheetRequest, actor auth.User) (RateSheet, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.persistLocked()

	rate, err := rateSheetFromPayload(payload)
	if err != nil {
		return RateSheet{}, err
	}
	s.upsertRateSheetLocked(rate, actor, false)
	s.notifyWorkspaceUpsert("rateSheets", "", rate)
	return rate, nil
}

func (s *Service) ImportRateSheets(payloads []RateSheetRequest, actor auth.User) ([]RateSheet, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.persistLocked()

	if len(payloads) == 0 {
		return nil, errValidation
	}
	saved := make([]RateSheet, 0, len(payloads))
	for _, payload := range payloads {
		rate, err := rateSheetFromPayload(payload)
		if err != nil {
			return nil, err
		}
		s.upsertRateSheetLocked(rate, actor, true)
		saved = append(saved, rate)
	}
	s.record(actor, "ratesheets.imported", "ratesheets", "Rate sheet rows imported.")
	changes := make([]WorkspaceChange, 0, len(saved))
	for _, rate := range saved {
		changes = append(changes, workspaceUpsert("rateSheets", "", rate))
	}
	s.notifyWorkspaceChanges(changes...)
	return saved, nil
}

func rateSheetFromPayload(payload RateSheetRequest) (RateSheet, error) {
	if strings.TrimSpace(payload.Courier) == "" || strings.TrimSpace(payload.Zone) == "" || payload.Price < 1 {
		return RateSheet{}, errValidation
	}
	return RateSheet{
		ID:           "rate_" + safeID(payload.Courier+"_"+payload.Service+"_z"+payload.Zone+"_"+payload.Weight),
		Courier:      strings.TrimSpace(payload.Courier),
		Service:      firstNonEmpty(payload.Service, "Duty paid premium"),
		Zone:         strings.TrimSpace(payload.Zone),
		Weight:       firstNonEmpty(payload.Weight, "0-5 kg"),
		Price:        payload.Price,
		Status:       firstNonEmpty(payload.Status, "Active"),
		SourceFileID: strings.TrimSpace(payload.SourceFileID),
		SourceName:   strings.TrimSpace(payload.SourceName),
	}, nil
}

func (s *Service) upsertRateSheetLocked(rate RateSheet, actor auth.User, quiet bool) {
	for index := range s.rateSheets {
		if s.rateSheets[index].ID == rate.ID {
			s.rateSheets[index] = rate
			if !quiet {
				s.record(actor, "ratesheets.updated", rate.ID, "Rate sheet row updated.")
			}
			return
		}
	}
	s.rateSheets = append([]RateSheet{rate}, s.rateSheets...)
	if !quiet {
		s.record(actor, "ratesheets.created", rate.ID, "Rate sheet row created.")
	}
}
func (s *Service) CreateShipping(payload ShippingRequestPayload, actor auth.User) (ShippingRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	mutation, err := s.beginFinancialMutationLocked()
	if err != nil {
		return ShippingRequest{}, err
	}
	defer mutation.rollback()

	if strings.TrimSpace(payload.Destination) == "" {
		return ShippingRequest{}, errValidation
	}
	customerID := firstNonEmpty(payload.CustomerID, actor.CustomerID, "cust_abc_export")
	if !auth.CanAccessCustomer(actor, customerID) {
		return ShippingRequest{}, errForbidden
	}
	if strings.TrimSpace(payload.ManufacturingID) != "" {
		ownerID, ok := s.manufacturingCustomerID(payload.ManufacturingID)
		if !ok {
			return ShippingRequest{}, errNotFound
		}
		if ownerID != customerID {
			return ShippingRequest{}, errForbidden
		}
	}
	courier := firstNonEmpty(payload.Courier, "FedEx")
	service := firstNonEmpty(payload.Service, "Duty paid premium")
	zone := firstNonEmpty(payload.Zone, "7")
	weight := firstNonEmpty(payload.Weight, "0-5 kg")
	amount := 0
	if payload.pricing != nil {
		amount = payload.pricing.Amount
	} else {
		amount, err = s.matchingShippingRate(courier, service, zone, weight)
		if err != nil {
			return ShippingRequest{}, err
		}
	}
	now := time.Now().UTC()
	row := ShippingRequest{
		ID: s.nextID("SHIP"), CustomerID: customerID, ManufacturingID: payload.ManufacturingID, Type: firstNonEmpty(payload.Type, "Outside product"),
		Courier: courier, Service: service, Destination: payload.Destination, Zone: zone, Weight: weight, Status: "Rate quoted",
		QuotedAmount: amount, Pricing: payload.pricing, CreatedAt: now.Format(time.RFC3339),
		History: []Update{{Label: "Shipping requested", Detail: "Customer requested courier service through ChakuChuri.", Actor: actorName(actor, "Customer"), CreatedAt: now.Format(time.RFC3339)}},
	}
	s.shipping = append([]ShippingRequest{row}, s.shipping...)
	s.postLedgerEntryLocked(actor, row.CustomerID, "shipping_request", row.ID, row.QuotedAmount, 0, "Shipping service charge posted.", now.Format(time.RFC3339))
	if err := mutation.commit(actor); err != nil {
		return ShippingRequest{}, err
	}
	s.record(actor, "shipping.requested", row.ID, "Shipping request created.")
	changes := []WorkspaceChange{workspaceUpsert("shipping", row.CustomerID, row)}
	for _, entry := range s.ledger {
		if entry.SourceType == "shipping_request" && entry.SourceID == row.ID {
			changes = append(changes, workspaceUpsert("ledger", row.CustomerID, entry))
			break
		}
	}
	s.notifyWorkspaceChanges(changes...)
	return row, nil
}

func (s *Service) DispatchShipping(id string, tracking string, actor auth.User) (ShippingRequest, error) {
	return s.UpdateShipping(id, ShippingUpdatePayload{
		Status:   "In transit",
		Tracking: tracking,
		Note:     "Courier movement started.",
	}, actor)
}

type ShippingUpdatePayload struct {
	QuotedAmount *int   `json:"quotedAmount,omitempty"`
	Confirmation string `json:"confirmation,omitempty"`
	Status       string `json:"status"`
	Tracking     string `json:"tracking"`
	Destination  string `json:"destination"`
	Courier      string `json:"courier"`
	Service      string `json:"service"`
	Zone         string `json:"zone"`
	Weight       string `json:"weight"`
	Note         string `json:"note"`
}

var shippingStatusOrder = []string{
	"Booked",
	"Confirmed",
	"Label created",
	"Picked up",
	"In transit",
	"Out for delivery",
	"Delivered",
}

func normalizeShippingStatus(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	lower := strings.ToLower(trimmed)
	switch {
	case strings.Contains(lower, "cancel"):
		return "Cancelled"
	case strings.Contains(lower, "out for"):
		return "Out for delivery"
	case strings.Contains(lower, "deliver"):
		return "Delivered"
	case strings.Contains(lower, "transit") || strings.Contains(lower, "dispatch"):
		return "In transit"
	case strings.Contains(lower, "pick"):
		return "Picked up"
	case strings.Contains(lower, "label"):
		return "Label created"
	case strings.Contains(lower, "confirm"):
		return "Confirmed"
	case strings.Contains(lower, "book") || strings.Contains(lower, "quote") || strings.Contains(lower, "request") || strings.Contains(lower, "pending"):
		return "Booked"
	default:
		for _, status := range shippingStatusOrder {
			if strings.EqualFold(status, trimmed) {
				return status
			}
		}
		return trimmed
	}
}

func shippingStatusIndex(status string) int {
	normalized := normalizeShippingStatus(status)
	for index, item := range shippingStatusOrder {
		if item == normalized {
			return index
		}
	}
	return -1
}

func (s *Service) UpdateShipping(id string, payload ShippingUpdatePayload, actor auth.User) (ShippingRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	mutation, err := s.beginFinancialMutationLocked()
	if err != nil {
		return ShippingRequest{}, err
	}
	defer mutation.rollback()

	if !auth.HasPermission(actor, "shipping.manage") {
		return ShippingRequest{}, errForbidden
	}

	for index := range s.shipping {
		row := &s.shipping[index]
		if row.ID != id {
			continue
		}
		if !auth.CanAccessCustomer(actor, row.CustomerID) {
			return ShippingRequest{}, errForbidden
		}

		current := normalizeShippingStatus(row.Status)
		if current == "" {
			current = "Booked"
		}
		closed := current == "Delivered" || current == "Cancelled"
		nextStatus := normalizeShippingStatus(payload.Status)
		if nextStatus != "" && closed && nextStatus != current {
			return ShippingRequest{}, errValidation
		}
		if nextStatus != "" && nextStatus != "Cancelled" && nextStatus != current {
			currentIndex := shippingStatusIndex(current)
			nextIndex := shippingStatusIndex(nextStatus)
			if nextIndex < 0 {
				return ShippingRequest{}, errValidation
			}
			// Allow same status, one step forward, or jump forward within the pipeline.
			if currentIndex >= 0 && nextIndex < currentIndex {
				return ShippingRequest{}, errValidation
			}
		}

		commercialChange := (payload.Courier != "" && payload.Courier != row.Courier) || (payload.Service != "" && payload.Service != row.Service) || (payload.Zone != "" && payload.Zone != row.Zone) || (payload.Weight != "" && payload.Weight != row.Weight)
		priceChange := payload.QuotedAmount != nil && *payload.QuotedAmount != row.QuotedAmount
		if (commercialChange || priceChange) && (closed || nextStatus == "Cancelled" || payload.QuotedAmount == nil || *payload.QuotedAmount < 1 || *payload.QuotedAmount > maxMoneyAmount || payload.Confirmation != row.ID || strings.TrimSpace(payload.Note) == "") {
			return ShippingRequest{}, errValidation
		}
		changed := false
		now := time.Now().UTC()
		if priceChange {
			difference := *payload.QuotedAmount - row.QuotedAmount
			s.postLedgerEntryLocked(actor, row.CustomerID, "shipping_adjustment", row.ID+"-"+now.Format("20060102T150405.000000000"), max(difference, 0), max(-difference, 0), "Shipping price adjustment: "+payload.Note, now.Format(time.RFC3339))
			row.QuotedAmount = *payload.QuotedAmount
			changed = true
		}
		details := make([]string, 0, 6)

		if tracking := strings.TrimSpace(payload.Tracking); tracking != "" && tracking != row.Tracking {
			row.Tracking = tracking
			details = append(details, "Tracking set to "+tracking+".")
			changed = true
		}
		if destination := strings.TrimSpace(payload.Destination); destination != "" && destination != row.Destination {
			row.Destination = destination
			details = append(details, "Destination updated.")
			changed = true
		}
		if courier := strings.TrimSpace(payload.Courier); courier != "" && courier != row.Courier {
			row.Courier = courier
			details = append(details, "Courier set to "+courier+".")
			changed = true
		}
		if service := strings.TrimSpace(payload.Service); service != "" && service != row.Service {
			row.Service = service
			details = append(details, "Service set to "+service+".")
			changed = true
		}
		if zone := strings.TrimSpace(payload.Zone); zone != "" && zone != row.Zone {
			row.Zone = zone
			details = append(details, "Zone set to "+zone+".")
			changed = true
		}
		if weight := strings.TrimSpace(payload.Weight); weight != "" && weight != row.Weight {
			row.Weight = weight
			details = append(details, "Weight set to "+weight+".")
			changed = true
		}

		statusChanged := nextStatus != "" && nextStatus != current
		if statusChanged {
			if nextStatus == "Cancelled" {
				s.postLedgerEntryLocked(actor, row.CustomerID, "shipping_cancellation", row.ID, 0, row.QuotedAmount, "Shipping charge reversed on cancellation. "+payload.Note, now.Format(time.RFC3339))
			}
			row.Status = nextStatus
			details = append(details, "Status moved to "+nextStatus+".")
			changed = true
		}
		note := strings.TrimSpace(payload.Note)
		if note != "" {
			details = append(details, note)
			changed = true
		}
		if !changed {
			return *row, nil
		}

		label := "Shipment updated"
		switch {
		case nextStatus == "Cancelled":
			label = "Shipment cancelled"
		case nextStatus == "Delivered":
			label = "Shipment delivered"
		case nextStatus == "In transit":
			label = "Shipment dispatched"
		case nextStatus == "Out for delivery":
			label = "Out for delivery"
		case nextStatus == "Picked up":
			label = "Shipment picked up"
		case nextStatus == "Label created":
			label = "Shipping label created"
		case nextStatus == "Confirmed":
			label = "Shipment confirmed"
		case note != "" && !statusChanged:
			label = "Timeline note added"
		}

		detail := strings.Join(details, " ")
		if detail == "" {
			detail = "Shipment details were updated."
		}
		row.History = append([]Update{{
			Label: label, Detail: detail, Actor: actorName(actor, "Admin"), CreatedAt: now.Format(time.RFC3339),
		}}, row.History...)
		if err := mutation.commit(actor); err != nil {
			return ShippingRequest{}, err
		}
		s.record(actor, "shipping.updated", row.ID, label)
		if priceChange || (statusChanged && nextStatus == "Cancelled") {
			s.notifyAccountingRecordsLocked(row.CustomerID)
		}
		s.notifyWorkspaceUpsert("shipping", row.CustomerID, *row)

		if nextStatus == "In transit" {
			s.publishEmailEvent(EmailEvent{
				EventKey: fmt.Sprintf("shipment:%s:dispatched:%d", row.ID, now.UnixNano()),
				Trigger:  "shipment.dispatched", CustomerID: row.CustomerID, EntityID: row.ID,
				Data: map[string]string{
					"shipment_id": row.ID, "courier": row.Courier, "tracking_number": firstNonEmpty(row.Tracking, "Pending"),
					"shipment_status": row.Status, "order_id": row.ManufacturingID,
				},
			})
		}
		return *row, nil
	}
	return ShippingRequest{}, errNotFound
}

func unitPriceForQuote(totalAmount, quantity int) int {
	if quantity < 1 {
		return totalAmount
	}
	return totalAmount / quantity
}

func (s *Service) CreatePayment(payload PaymentRequest, actor auth.User) (Payment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	mutation, err := s.beginFinancialMutationLocked()
	if err != nil {
		return Payment{}, err
	}
	defer mutation.rollback()

	customerID := firstNonEmpty(payload.CustomerID, actor.CustomerID, "cust_abc_export")
	if !auth.CanAccessCustomer(actor, customerID) {
		return Payment{}, errForbidden
	}
	manufacturingID := strings.TrimSpace(payload.ManufacturingID)
	shippingID := strings.TrimSpace(payload.ShippingID)
	if manufacturingID != "" {
		ownerID, ok := s.manufacturingCustomerID(manufacturingID)
		if !ok {
			return Payment{}, errNotFound
		}
		if ownerID != customerID {
			return Payment{}, errForbidden
		}
		for _, order := range s.manufacturing {
			if order.ID == manufacturingID && strings.EqualFold(order.Status, "Cancelled") {
				return Payment{}, errOrderClosed
			}
		}
	}
	if shippingID != "" {
		ownerID, ok := s.shippingCustomerID(shippingID)
		if !ok {
			return Payment{}, errNotFound
		}
		if ownerID != customerID {
			return Payment{}, errForbidden
		}
	}
	if payload.Amount < 1 || payload.Amount > maxMoneyAmount || (manufacturingID != "" && shippingID != "") {
		return Payment{}, errValidation
	}
	paymentType := strings.TrimSpace(payload.Type)
	if paymentType == "" {
		if shippingID != "" {
			paymentType = "Shipping"
		} else {
			paymentType = "Account payment"
		}
	}
	key, err := paymentSubmissionKey(payload)
	if err != nil {
		return Payment{}, err
	}
	for _, prior := range s.payments {
		if key != "" && prior.CustomerID == customerID && prior.IdempotencyKey == key {
			if prior.Amount != payload.Amount || prior.ManufacturingID != manufacturingID || prior.ShippingID != shippingID || prior.ProofFileID != strings.TrimSpace(payload.ProofFileID) || prior.Type != paymentType || strings.TrimSpace(prior.Note) != strings.TrimSpace(payload.Note) {
				return Payment{}, errPaymentConflict
			}
			return prior, nil
		}
	}
	now := time.Now().UTC()
	payment := Payment{
		IdempotencyKey: key,
		ID:             s.nextID("PAY"), CustomerID: customerID, ManufacturingID: manufacturingID, ShippingID: shippingID,
		Type: paymentType, Amount: payload.Amount, Status: "Waiting confirmation", ProofName: strings.TrimSpace(payload.ProofName), ProofFileID: strings.TrimSpace(payload.ProofFileID),
		Note: strings.TrimSpace(payload.Note), CreatedAt: now.Format(time.RFC3339),
	}
	s.payments = append([]Payment{payment}, s.payments...)
	if err := mutation.commit(actor); err != nil {
		return Payment{}, err
	}
	s.record(actor, "payments.submitted", payment.ID, "Payment proof submitted.")
	s.notifyWorkspaceUpsert("payments", payment.CustomerID, payment)
	return payment, nil
}

func (s *Service) ConfirmPayment(id string, actor auth.User) (PaymentConfirmation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	mutation, err := s.beginFinancialMutationLocked()
	if err != nil {
		return PaymentConfirmation{}, err
	}
	defer mutation.rollback()

	for index := range s.payments {
		if s.payments[index].ID != id {
			continue
		}
		payment := &s.payments[index]
		if !auth.CanAccessCustomer(actor, payment.CustomerID) {
			return PaymentConfirmation{}, errForbidden
		}
		if payment.Status == "Confirmed" {
			return PaymentConfirmation{Payment: *payment}, nil
		}
		if err := s.requireAccountingReadyLocked(payment.CustomerID); err != nil {
			return PaymentConfirmation{}, err
		}
		if strings.EqualFold(payment.Status, "Cancelled") {
			return PaymentConfirmation{}, errOrderClosed
		}
		if payment.ManufacturingID != "" {
			for _, order := range s.manufacturing {
				if order.ID == payment.ManufacturingID && strings.EqualFold(order.Status, "Cancelled") {
					return PaymentConfirmation{}, errOrderClosed
				}
			}
		}

		now := time.Now().UTC()
		payment.Status = "Confirmed"
		payment.ConfirmedAt = now.Format(time.RFC3339)
		remaining := 0
		allocations := make([]PaymentAllocation, 0)
		touched := make([]ManufacturingOrder, 0)

		touched = s.resyncCustomerOrderBalancesLocked(payment.CustomerID, payment.ID, actor, now)
		remaining = payment.CreditLeft
		allocations = append(allocations, payment.Allocations...)

		ledgerNote := "Payment confirmed by company."
		if strings.EqualFold(strings.TrimSpace(payment.Type), "Admin payment") {
			ledgerNote = firstNonEmpty(strings.TrimSpace(payment.Note), "Admin added")
		}
		if remaining > 0 && payment.ShippingID == "" {
			ledgerNote += fmt.Sprintf(" Rs %d kept as customer credit.", remaining)
		}
		s.postLedgerEntryLocked(actor, payment.CustomerID, "payment", payment.ID, 0, payment.Amount, ledgerNote, now.Format(time.RFC3339))
		if err := mutation.commit(actor); err != nil {
			return PaymentConfirmation{}, err
		}
		s.record(actor, "payments.confirmed", id, "Payment confirmed.")

		result := PaymentConfirmation{Payment: *payment, Orders: touched}
		changes := []WorkspaceChange{workspaceUpsert("payments", payment.CustomerID, *payment)}
		for _, order := range touched {
			changes = append(changes, workspaceUpsert("manufacturing", order.CustomerID, order))
		}
		// Resync rewrites allocations on older payments for this customer — push those too.
		for _, row := range s.payments {
			if row.CustomerID != payment.CustomerID || row.ID == payment.ID {
				continue
			}
			copyPayment := row
			changes = append(changes, workspaceUpsert("payments", row.CustomerID, copyPayment))
		}
		for _, entry := range s.ledger {
			if entry.SourceType == "payment" && entry.SourceID == payment.ID {
				copyEntry := entry
				result.Ledger = &copyEntry
				changes = append(changes, workspaceUpsert("ledger", payment.CustomerID, entry))
				break
			}
		}
		s.notifyWorkspaceChanges(changes...)
		cancelReminderOrders := make([]string, 0, len(touched))
		orderIDs := make([]string, 0, len(touched))
		primaryOrderID := strings.TrimSpace(payment.ManufacturingID)
		for _, order := range touched {
			orderIDs = append(orderIDs, order.ID)
			cancelReminderOrders = append(cancelReminderOrders, order.ID)
			if primaryOrderID == "" {
				primaryOrderID = order.ID
			}
		}
		for _, allocation := range payment.Allocations {
			if allocation.ManufacturingID == "" {
				continue
			}
			found := false
			for _, id := range orderIDs {
				if id == allocation.ManufacturingID {
					found = true
					break
				}
			}
			if !found {
				orderIDs = append(orderIDs, allocation.ManufacturingID)
				cancelReminderOrders = append(cancelReminderOrders, allocation.ManufacturingID)
			}
			if primaryOrderID == "" {
				primaryOrderID = allocation.ManufacturingID
			}
		}
		if primaryOrderID != "" {
			found := false
			for _, id := range cancelReminderOrders {
				if id == primaryOrderID {
					found = true
					break
				}
			}
			if !found {
				cancelReminderOrders = append(cancelReminderOrders, primaryOrderID)
			}
		}
		s.publishEmailEvent(EmailEvent{
			EventKey: "payment:" + payment.ID + ":confirmed", Trigger: "payment.confirmed",
			CustomerID: payment.CustomerID, EntityID: payment.ID,
			Data: map[string]string{
				"payment_id": payment.ID, "payment_type": payment.Type,
				"payment_amount": fmt.Sprintf("PKR %d", payment.Amount),
				"payment_status": payment.Status, "order_id": primaryOrderID,
				"order_ids":              strings.Join(orderIDs, ","),
				"cancel_reminder_orders": strings.Join(cancelReminderOrders, ","),
				"credit_left":            fmt.Sprintf("PKR %d", payment.CreditLeft),
			},
		})
		for _, order := range touched {
			if order.BalanceDue < 1 {
				continue
			}
			s.publishEmailEvent(EmailEvent{
				EventKey: fmt.Sprintf("order:%s:payment-due:%d", order.ID, now.UnixNano()),
				Trigger:  "payment.due", CustomerID: order.CustomerID, EntityID: order.ID,
				Data: map[string]string{
					"order_id": order.ID, "product_name": order.ProductName,
					"balance_due": fmt.Sprintf("PKR %d", order.BalanceDue),
				},
			})
		}
		return result, nil
	}
	return PaymentConfirmation{}, errNotFound
}

func (s *Service) ReversePayment(id string, payload PaymentReversalRequest, actor auth.User) (PaymentConfirmation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	mutation, err := s.beginFinancialMutationLocked()
	if err != nil {
		return PaymentConfirmation{}, err
	}
	defer mutation.rollback()

	reason := strings.TrimSpace(payload.Reason)
	if strings.TrimSpace(payload.Confirmation) != strings.TrimSpace(id) || reason == "" {
		return PaymentConfirmation{}, errConfirmationRequired
	}
	for index := range s.payments {
		payment := &s.payments[index]
		if payment.ID != id {
			continue
		}
		if !auth.CanAccessCustomer(actor, payment.CustomerID) {
			return PaymentConfirmation{}, errForbidden
		}
		if payment.Status == "Reversed" {
			return PaymentConfirmation{Payment: *payment}, nil
		}
		if payment.Status != "Confirmed" {
			return PaymentConfirmation{}, errValidation
		}

		now := time.Now().UTC()
		payment.Status = "Reversed"
		payment.ReversedAt = now.Format(time.RFC3339)
		payment.ReversalReason = reason
		payment.Allocations = nil
		payment.CreditLeft = 0
		s.postLedgerEntryLocked(actor, payment.CustomerID, "payment_reversal", payment.ID, payment.Amount, 0, "Confirmed payment reversed after verification: "+reason, now.Format(time.RFC3339))
		touched := s.resyncCustomerOrderBalancesLocked(payment.CustomerID, "", actor, now)
		if err := mutation.commit(actor); err != nil {
			return PaymentConfirmation{}, err
		}
		s.record(actor, "payments.reversed", payment.ID, "Confirmed payment reversed after typed-ID verification. Reason: "+reason)

		result := PaymentConfirmation{Payment: *payment, Orders: touched}
		changes := []WorkspaceChange{workspaceUpsert("payments", payment.CustomerID, *payment)}
		for _, order := range touched {
			changes = append(changes, workspaceUpsert("manufacturing", order.CustomerID, order))
		}
		for _, entry := range s.ledger {
			if entry.SourceType == "payment_reversal" && entry.SourceID == payment.ID {
				copyEntry := entry
				result.Ledger = &copyEntry
				changes = append(changes, workspaceUpsert("ledger", payment.CustomerID, entry))
				break
			}
		}
		s.notifyWorkspaceChanges(changes...)
		return result, nil
	}
	return PaymentConfirmation{}, errNotFound
}
func (s *Service) applyPaymentWaterfallLocked(customerID string, preferredOrderID string, amount int, actor auth.User, now time.Time, recordHistory bool) (int, []PaymentAllocation, []ManufacturingOrder) {
	if amount < 1 {
		return amount, nil, nil
	}
	indexes := make([]int, 0)
	for index := range s.manufacturing {
		order := s.manufacturing[index]
		if order.CustomerID != customerID || orderIsClosed(order) || order.BalanceDue < 1 {
			continue
		}
		indexes = append(indexes, index)
	}
	sort.SliceStable(indexes, func(i, j int) bool {
		left := s.manufacturing[indexes[i]]
		right := s.manufacturing[indexes[j]]
		if preferredOrderID != "" {
			if left.ID == preferredOrderID && right.ID != preferredOrderID {
				return true
			}
			if right.ID == preferredOrderID && left.ID != preferredOrderID {
				return false
			}
		}
		// Deposit-pending orders first, then oldest open balance.
		leftDeposit := left.Status == "Deposit pending"
		rightDeposit := right.Status == "Deposit pending"
		if leftDeposit != rightDeposit {
			return leftDeposit
		}
		return left.CreatedAt < right.CreatedAt
	})

	remaining := amount
	allocations := make([]PaymentAllocation, 0)
	touched := make([]ManufacturingOrder, 0)
	for _, index := range indexes {
		if remaining < 1 {
			break
		}
		order := &s.manufacturing[index]
		before := order.BalanceDue
		applied := remaining
		if applied > order.BalanceDue {
			applied = order.BalanceDue
		}
		if applied < 1 {
			continue
		}
		order.PaidAmount += applied
		order.BalanceDue -= applied
		if order.BalanceDue < 0 {
			order.BalanceDue = 0
		}
		remaining -= applied
		if depositOutstanding(*order) == 0 && (order.Status == "Deposit pending" || strings.EqualFold(order.CurrentStage, "Awaiting deposit")) {
			order.Status = "Confirmed"
			order.CurrentStage = "Confirmed"
			order.Progress = 43
			markStage(order.Stages, "Deposit received", "done", now.Format("2006-01-02"))
		}
		if recordHistory {
			detail := fmt.Sprintf("Rs %d applied to this order (balance %d → %d).", applied, before, order.BalanceDue)
			order.History = append([]Update{{
				Label: "Payment applied", Detail: detail, Actor: actorName(actor, "Admin"), CreatedAt: now.Format(time.RFC3339),
			}}, order.History...)
		}
		allocations = append(allocations, PaymentAllocation{ManufacturingID: order.ID, Amount: applied})
		touched = append(touched, *order)
	}
	return remaining, allocations, touched
}

func (s *Service) resetOpenOrderBalancesLocked(customerID string) {
	for index := range s.manufacturing {
		order := &s.manufacturing[index]
		if order.CustomerID != customerID || orderIsClosed(*order) {
			continue
		}
		order.PaidAmount = 0
		order.BalanceDue = order.TotalAmount
		if order.BalanceDue < 0 {
			order.BalanceDue = 0
		}
	}
}

func (s *Service) completedOrderIDsLocked(customerID string) map[string]bool {
	out := map[string]bool{}
	for _, order := range s.manufacturing {
		if order.CustomerID == customerID && strings.EqualFold(order.Status, "Completed") {
			out[order.ID] = true
		}
	}
	return out
}

func paymentAllocationSum(payment Payment) int {
	sum := 0
	for _, allocation := range payment.Allocations {
		sum += allocation.Amount
	}
	return sum
}

func paymentFrozenToCompleted(payment Payment, completed map[string]bool) ([]PaymentAllocation, int) {
	frozen := make([]PaymentAllocation, 0)
	sum := 0
	for _, allocation := range payment.Allocations {
		if !completed[allocation.ManufacturingID] || allocation.Amount < 1 {
			continue
		}
		frozen = append(frozen, allocation)
		sum += allocation.Amount
	}
	return frozen, sum
}

func (s *Service) customerOrderBalancesNeedSyncLocked(customerID string) bool {
	completed := s.completedOrderIDsLocked(customerID)
	activeShipping := map[string]bool{}
	for _, shipment := range s.shipping {
		if shipment.CustomerID == customerID && !strings.EqualFold(shipment.Status, "Cancelled") {
			activeShipping[shipment.ID] = true
		}
	}
	openPaidByID := map[string]int{}
	for _, order := range s.manufacturing {
		if order.CustomerID != customerID || orderIsClosed(order) {
			continue
		}
		if order.PaidAmount+order.BalanceDue != order.TotalAmount {
			return true
		}
		if order.PaidAmount < 0 || order.BalanceDue < 0 || order.DepositRequired > order.TotalAmount {
			return true
		}
		openPaidByID[order.ID] = order.PaidAmount
	}

	allocToOpen := map[string]int{}
	hasUnallocated := false
	for _, payment := range s.payments {
		if payment.CustomerID != customerID || payment.Status != "Confirmed" {
			continue
		}
		allocSum := paymentAllocationSum(payment)
		_, frozenSum := paymentFrozenToCompleted(payment, completed)
		if payment.Amount > 0 && len(payment.Allocations) == 0 {
			hasUnallocated = true
			continue
		}
		if allocSum+payment.CreditLeft != payment.Amount {
			return true
		}
		if frozenSum > payment.Amount {
			return true
		}
		if payment.ShippingID != "" {
			if !activeShipping[payment.ShippingID] {
				// A cancelled shipment releases its payment to customer credit.
				if len(payment.Allocations) == 0 && payment.CreditLeft == payment.Amount {
					continue
				}
				return true
			}
			if len(payment.Allocations) != 1 || payment.Allocations[0].ShippingID != payment.ShippingID || payment.Allocations[0].ManufacturingID != "" || payment.Allocations[0].Amount != payment.Amount || payment.CreditLeft != 0 {
				return true
			}
			continue
		}
		for _, allocation := range payment.Allocations {
			if completed[allocation.ManufacturingID] {
				continue
			}
			if _, open := openPaidByID[allocation.ManufacturingID]; open {
				allocToOpen[allocation.ManufacturingID] += allocation.Amount
				continue
			}
			if allocation.ManufacturingID != "" {
				// Allocation points at a missing or cancelled order — needs rebuild.
				return true
			}
		}
	}

	if hasUnallocated && len(openPaidByID) > 0 {
		return true
	}
	for id, paid := range openPaidByID {
		if allocToOpen[id] != paid {
			return true
		}
	}
	for id := range allocToOpen {
		if _, ok := openPaidByID[id]; !ok {
			return true
		}
	}
	return false
}

func (s *Service) repairOpenOrderBalanceFieldsLocked(customerID string) []ManufacturingOrder {
	out := make([]ManufacturingOrder, 0)
	for index := range s.manufacturing {
		order := &s.manufacturing[index]
		if order.CustomerID != customerID || orderIsClosed(*order) {
			continue
		}
		if order.DepositRequired > order.TotalAmount {
			order.DepositRequired = order.TotalAmount
		}
		if order.PaidAmount < 0 {
			order.PaidAmount = 0
		}
		if order.PaidAmount > order.TotalAmount {
			order.PaidAmount = order.TotalAmount
		}
		due := order.TotalAmount - order.PaidAmount
		if due < 0 {
			due = 0
		}
		if order.BalanceDue != due {
			order.BalanceDue = due
		}
		out = append(out, *order)
	}
	return out
}

func (s *Service) resyncCustomerOrderBalancesLocked(customerID string, historyPaymentID string, actor auth.User, now time.Time) []ManufacturingOrder {
	return s.allocateCustomerLocked(customerID, historyPaymentID, actor, now, false)
}

func depositOutstanding(order ManufacturingOrder) int {
	if order.DepositRequired <= 0 {
		return 0
	}
	left := order.DepositRequired - order.PaidAmount
	if left < 0 {
		return 0
	}
	return left
}

func (s *Service) CreateProduct(payload ProductRequest, actor auth.User) (Product, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.persistLocked()
	if strings.TrimSpace(payload.Name) == "" {
		return Product{}, errValidation
	}
	customerID := firstNonEmpty(payload.CustomerID, actor.CustomerID, "cust_abc_export")
	if !auth.CanAccessCustomer(actor, customerID) {
		return Product{}, errForbidden
	}
	now := time.Now().UTC()
	product := Product{
		ID: "prod_" + safeID(payload.Name) + "_" + now.Format("150405"), CustomerID: customerID,
		SKU: firstNonEmpty(payload.SKU, "SKU-"+now.Format("1504")), Name: strings.TrimSpace(payload.Name), Stock: payload.Stock,
		Image: strings.TrimSpace(payload.Image), ImageFileID: strings.TrimSpace(payload.ImageFileID), UpdatedAt: now.Format(time.RFC3339),
	}
	s.products = append([]Product{product}, s.products...)
	s.record(actor, "products.created", product.ID, "Customer product record created.")
	s.notifyWorkspaceUpsert("products", product.CustomerID, product)
	return product, nil
}

func (s *Service) SetMessageAttachmentResolver(resolver func(string, auth.User) (MessageAttachment, bool)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.attachmentResolver = resolver
}

func (s *Service) SetEmailEventPublisher(publisher func(EmailEvent)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.emailEventPublisher = publisher
}

func (s *Service) SendMessage(payload MessageRequest, actor auth.User) (Conversation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.persistLocked()

	customerID := firstNonEmpty(payload.CustomerID, actor.CustomerID, "cust_abc_export")
	if !auth.CanAccessCustomer(actor, customerID) {
		return Conversation{}, errForbidden
	}
	body := strings.TrimSpace(payload.Body)
	attachments, err := s.resolveMessageAttachmentsLocked(payload.AttachmentIDs, actor)
	if err != nil {
		return Conversation{}, err
	}
	if body == "" && len(attachments) == 0 {
		return Conversation{}, errValidation
	}

	now := time.Now().UTC().Format(time.RFC3339)
	message := Message{
		ID:          s.nextID("MSG"),
		Author:      actorName(actor, "Customer"),
		AuthorRole:  supportAuthorRole(actor),
		Body:        body,
		Attachments: attachments,
		CreatedAt:   now,
	}
	for index := range s.conversations {
		if s.conversations[index].CustomerID == customerID {
			s.conversations[index] = appendConversationMessage(s.conversations[index], message, actor)
			s.record(actor, "messages.sent", s.conversations[index].ID, messageAuditDetail(message))
			conversation := compactConversation(s.conversations[index], 0, defaultWorkspacePageSize)
			s.notifyWorkspaceUpsert("conversations", conversation.CustomerID, conversation)
			return conversation, nil
		}
	}
	conversation := appendConversationMessage(Conversation{
		ID: "CONV-" + time.Now().UTC().Format("150405"), CustomerID: customerID, Subject: "Support conversation", Status: "Open",
	}, message, actor)
	s.conversations = append([]Conversation{conversation}, s.conversations...)
	s.record(actor, "messages.sent", conversation.ID, "Conversation started. "+messageAuditDetail(message))
	conversation = compactConversation(conversation, 0, defaultWorkspacePageSize)
	s.notifyWorkspaceUpsert("conversations", conversation.CustomerID, conversation)
	return conversation, nil
}

func (s *Service) MarkConversationRead(id string, actor auth.User) (Conversation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.persistLocked()

	id = strings.TrimSpace(id)
	for index := range s.conversations {
		conversation := s.conversations[index]
		if conversation.ID != id {
			continue
		}
		if !auth.CanAccessCustomer(actor, conversation.CustomerID) {
			return Conversation{}, errForbidden
		}
		if auth.IsCustomerRole(actor) {
			conversation.UnreadForCustomer = 0
		} else {
			conversation.UnreadForAdmin = 0
		}
		s.conversations[index] = conversation
		conversation = compactConversation(conversation, 0, defaultWorkspacePageSize)
		s.notifyWorkspaceUpsert("conversations", conversation.CustomerID, conversation)
		return conversation, nil
	}
	return Conversation{}, errNotFound
}

func (s *Service) ConversationMessages(id string, offset, limit int, actor auth.User) (ConversationMessagePage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	id = strings.TrimSpace(id)
	for _, conversation := range s.conversations {
		if conversation.ID != id {
			continue
		}
		if !auth.CanAccessCustomer(actor, conversation.CustomerID) {
			return ConversationMessagePage{}, errForbidden
		}
		compact := compactConversation(conversation, offset, limit)
		return ConversationMessagePage{
			ConversationID: compact.ID,
			Messages:       compact.Messages,
			Pagination:     compact.MessagePagination,
		}, nil
	}
	return ConversationMessagePage{}, errNotFound
}

func quoteReferenceIDs(payload QuoteRequest) []string {
	seen := map[string]bool{}
	ids := make([]string, 0, len(payload.ImageFileIDs)+1)
	for _, value := range append(payload.ImageFileIDs, payload.ImageFileID) {
		id := strings.TrimSpace(value)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids
}

func (s *Service) resolveQuoteReferencesLocked(ids []string, actor auth.User) ([]string, error) {
	if len(ids) == 0 || len(ids) > 5 || s.attachmentResolver == nil {
		return nil, errValidation
	}
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		attachment, ok := s.attachmentResolver(id, actor)
		if !ok || strings.TrimSpace(attachment.FileID) == "" {
			return nil, errForbidden
		}
		if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(attachment.MimeType)), "image/") {
			return nil, errValidation
		}
		name := strings.TrimSpace(attachment.Name)
		if name == "" {
			name = "Product reference"
		}
		names = append(names, name)
	}
	return names, nil
}
func (s *Service) resolveMessageAttachmentsLocked(ids []string, actor auth.User) ([]MessageAttachment, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	if len(ids) > 5 || s.attachmentResolver == nil {
		return nil, errValidation
	}
	seen := map[string]bool{}
	attachments := make([]MessageAttachment, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		attachment, ok := s.attachmentResolver(id, actor)
		if !ok || strings.TrimSpace(attachment.FileID) == "" {
			return nil, errForbidden
		}
		seen[id] = true
		attachments = append(attachments, attachment)
	}
	if len(attachments) == 0 {
		return nil, errValidation
	}
	return attachments, nil
}

func messageAuditDetail(message Message) string {
	if len(message.Attachments) == 0 {
		return "Message sent."
	}
	return fmt.Sprintf("Message sent with %d attachment(s).", len(message.Attachments))
}

func (s *Service) StartCall(payload CallRequestPayload, actor auth.User) (CallRequest, error) {
	customerID := firstNonEmpty(payload.CustomerID, actor.CustomerID)
	if customerID == "" {
		return CallRequest{}, errValidation
	}
	if !auth.CanAccessCustomer(actor, customerID) {
		return CallRequest{}, errForbidden
	}
	// Online checks before workflow lock — never hold s.mu while taking auth locks.
	if s.authService != nil {
		if auth.IsCustomerRole(actor) {
			if !s.authService.IsSupportOnline() {
				return CallRequest{}, errCalleeOffline
			}
		} else if !s.authService.IsCustomerOnline(customerID) {
			return CallRequest{}, errCalleeOffline
		}
	}

	s.mu.Lock()
	expired := s.expireRingingCallsLocked(time.Now().UTC())

	conversationID := strings.TrimSpace(payload.ConversationID)
	if conversationID != "" && !s.conversationBelongsToCustomerLocked(conversationID, customerID) {
		s.mu.Unlock()
		return CallRequest{}, errForbidden
	}
	if conflict := s.activeCallConflictLocked(customerID, actor.ID); conflict != "" {
		s.mu.Unlock()
		return CallRequest{}, errCallBusy
	}

	now := time.Now().UTC()
	initiatorName := actorName(actor, "Customer")
	recipientName := strings.TrimSpace(payload.RecipientName)
	if recipientName == "" {
		if auth.IsCustomerRole(actor) {
			recipientName = "ChakuChuri Support"
		} else {
			recipientName = "Customer"
		}
	}
	row := CallRequest{
		ID:              s.nextID("CALL"),
		CustomerID:      customerID,
		ConversationID:  conversationID,
		Subject:         firstNonEmpty(payload.Subject, "Audio call"),
		Status:          "Ringing",
		CallType:        "audio",
		InitiatorUserID: actor.ID,
		InitiatorName:   initiatorName,
		InitiatorRole:   directCallRole(actor),
		RecipientName:   recipientName,
		CreatedAt:       now.Format(time.RFC3339),
		UpdatedAt:       now.Format(time.RFC3339),
		RingExpiresAt:   now.Add(defaultRingTimeout).Format(time.RFC3339),
		History: []Update{{
			Label:     "Call started",
			Detail:    initiatorName + " started an audio call.",
			Actor:     initiatorName,
			CreatedAt: now.Format(time.RFC3339),
		}},
	}
	s.calls = append([]CallRequest{row}, s.calls...)
	s.record(actor, "calls.started", row.ID, "Direct audio call started.")
	s.persistLocked()
	s.mu.Unlock()

	for _, missed := range expired {
		s.notifyWorkspaceUpsert("calls", missed.CustomerID, missed)
	}
	s.notifyWorkspaceUpsert("calls", row.CustomerID, row)
	return row, nil
}

func (s *Service) activeCallConflictLocked(customerID, actorID string) string {
	for _, call := range s.calls {
		if call.Status != "Ringing" && call.Status != "In call" {
			continue
		}
		if call.CustomerID == customerID {
			return call.ID
		}
		if actorID != "" && (call.InitiatorUserID == actorID || call.AnsweredByUserID == actorID) {
			return call.ID
		}
	}
	return ""
}

func (s *Service) UpdateCallStatus(id string, payload CallActionRequest, actor auth.User) (CallRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.persistLocked()

	expired := s.expireRingingCallsLocked(time.Now().UTC())
	for _, missed := range expired {
		s.notifyWorkspaceUpsert("calls", missed.CustomerID, missed)
	}

	status := normalizeCallStatus(payload.Status)
	if status == "" {
		return CallRequest{}, errValidation
	}
	index, call, ok := s.findCallLocked(id)
	if !ok {
		return CallRequest{}, errNotFound
	}
	if !canUseCall(actor, call) {
		return CallRequest{}, errForbidden
	}
	if isTerminalCallStatus(call.Status) {
		if call.Status == status {
			return call, nil
		}
		return CallRequest{}, errInvalidCallTransition
	}

	previous := call.Status
	now := time.Now().UTC().Format(time.RFC3339)
	initiatedByActor := call.InitiatorUserID != "" && call.InitiatorUserID == actor.ID
	switch status {
	case "In call":
		if previous != "Ringing" || initiatedByActor {
			return CallRequest{}, errInvalidCallTransition
		}
		call.RoomID = firstNonEmpty(call.RoomID, callRoomID(call.ID))
		call.StartedAt = firstNonEmpty(call.StartedAt, now)
		call.AnsweredByUserID = actor.ID
		call.AnsweredBy = actorName(actor, "Support")
	case "Declined":
		if previous != "Ringing" || initiatedByActor {
			return CallRequest{}, errInvalidCallTransition
		}
	case "Cancelled":
		if previous != "Ringing" || (call.InitiatorUserID != "" && !initiatedByActor) {
			return CallRequest{}, errInvalidCallTransition
		}
	case "Missed":
		if previous != "Ringing" {
			return CallRequest{}, errInvalidCallTransition
		}
	case "Completed":
		if previous != "In call" {
			return CallRequest{}, errInvalidCallTransition
		}
	default:
		return CallRequest{}, errInvalidCallTransition
	}

	call.Status = status
	call.UpdatedAt = now
	if isTerminalCallStatus(status) {
		call.EndedAt = firstNonEmpty(call.EndedAt, now)
		call.DurationSeconds = callDurationSeconds(call)
	}
	detail := firstNonEmpty(payload.Note, callStatusDetail(status))
	call.History = append([]Update{{
		Label:     callStatusLabel(status),
		Detail:    detail,
		Actor:     actorName(actor, "Support"),
		CreatedAt: now,
	}}, call.History...)
	s.calls[index] = call
	s.record(actor, "calls."+strings.ReplaceAll(strings.ToLower(status), " ", "_"), call.ID, detail)
	s.notifyWorkspaceUpsert("calls", call.CustomerID, call)
	if previous == "In call" && isTerminalCallStatus(status) {
		notifyCallRoomEnded(call)
	}
	return call, nil
}

func (s *Service) SetWebRTC(webrtc config.WebRTC) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.webrtc = webrtc
}

// Start runs background call maintenance (ring expiry fanout).
func (s *Service) Start(ctx context.Context) {
	if s == nil {
		return
	}
	s.startOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					s.ExpireRingingCalls()
				}
			}
		}()
	})
}

const defaultRingTimeout = 45 * time.Second

func (s *Service) CallConfig(actor auth.User) map[string]interface{} {
	if s.authService != nil {
		s.authService.TouchOnline(actor.ID)
	}
	s.mu.RLock()
	webrtc := s.webrtc
	s.mu.RUnlock()
	return map[string]interface{}{
		"signalingMode":         "websocket",
		"fallbackSignalingMode": "http-polling",
		"pollMs":                1200,
		"websocketPath":         "/api/realtime/calls",
		"redisFanoutEnabled":    false,
		"ringTimeoutSeconds":    int(defaultRingTimeout / time.Second),
		"turnConfigured":        webrtc.TurnConfigured(),
		"iceServers":            webrtc.IceServers(actor.ID),
	}
}

func (s *Service) ListCallSignals(id string, after int, actor auth.User) ([]CallSignal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	index, call, ok := s.findCallLocked(id)
	if !ok {
		return nil, errNotFound
	}
	if !canUseCall(actor, call) {
		return nil, errForbidden
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if auth.IsCustomerRole(actor) {
		call.CustomerLastSeenAt = now
	} else {
		call.AdminLastSeenAt = now
	}
	s.calls[index] = call

	result := []CallSignal{}
	for _, signal := range s.callSignals {
		if signal.CallID == call.ID && signal.SignalNo > after && signal.SenderID != actor.ID {
			result = append(result, signal)
		}
	}
	return result, nil
}

func (s *Service) SendCallSignal(id string, payload CallSignalRequest, actor auth.User) (CallSignal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.persistLocked()

	index, call, ok := s.findCallLocked(id)
	if !ok {
		return CallSignal{}, errNotFound
	}
	if !canUseCall(actor, call) {
		return CallSignal{}, errForbidden
	}
	if call.Status != "In call" || !validSignalType(payload.SignalType) {
		return CallSignal{}, errValidation
	}
	if payload.Payload == nil {
		payload.Payload = map[string]interface{}{}
	}
	now := time.Now().UTC().Format(time.RFC3339)
	last := 0
	for _, signal := range s.callSignals {
		if signal.CallID == call.ID && signal.SignalNo > last {
			last = signal.SignalNo
		}
	}
	signal := CallSignal{
		ID:         s.nextID("SIG"),
		CallID:     call.ID,
		SignalNo:   last + 1,
		SenderID:   actor.ID,
		SenderRole: supportAuthorRole(actor),
		SignalType: strings.TrimSpace(payload.SignalType),
		Payload:    payload.Payload,
		CreatedAt:  now,
	}
	s.callSignals = append(s.callSignals, signal)
	s.callSignals = trimCallSignals(s.callSignals, 1000)
	if auth.IsCustomerRole(actor) {
		call.CustomerLastSeenAt = now
	} else {
		call.AdminLastSeenAt = now
	}
	s.calls[index] = call
	return signal, nil
}

func (s *Service) HeartbeatCall(id string, actor auth.User) (CallRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	index, call, ok := s.findCallLocked(id)
	if !ok {
		return CallRequest{}, errNotFound
	}
	if !canUseCall(actor, call) {
		return CallRequest{}, errForbidden
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if auth.IsCustomerRole(actor) {
		call.CustomerLastSeenAt = now
	} else {
		call.AdminLastSeenAt = now
	}
	s.calls[index] = call
	return call, nil
}

func (s *Service) EndCall(id string, actor auth.User) (CallRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.persistLocked()

	index, call, ok := s.findCallLocked(id)
	if !ok {
		return CallRequest{}, errNotFound
	}
	if !canUseCall(actor, call) {
		return CallRequest{}, errForbidden
	}
	if isTerminalCallStatus(call.Status) {
		return call, nil
	}

	previous := call.Status
	now := time.Now().UTC().Format(time.RFC3339)
	switch call.Status {
	case "In call":
		call.Status = "Completed"
	case "Ringing":
		if call.InitiatorUserID != "" && call.InitiatorUserID != actor.ID {
			call.Status = "Declined"
		} else {
			call.Status = "Cancelled"
		}
	default:
		return CallRequest{}, errInvalidCallTransition
	}
	call.UpdatedAt = now
	call.EndedAt = now
	call.DurationSeconds = callDurationSeconds(call)
	call.History = append([]Update{{
		Label:     callStatusLabel(call.Status),
		Detail:    callStatusDetail(call.Status),
		Actor:     actorName(actor, "Support"),
		CreatedAt: now,
	}}, call.History...)
	s.calls[index] = call
	s.record(actor, "calls.ended", call.ID, callStatusDetail(call.Status))
	s.notifyWorkspaceUpsert("calls", call.CustomerID, call)
	if previous == "In call" {
		notifyCallRoomEnded(call)
	}
	return call, nil
}

func Register(mux *http.ServeMux, service *Service) {
	service.registerAccounting(mux)
	mux.HandleFunc("/api/realtime/workspace", service.handleWorkspaceSocket)
	mux.HandleFunc("/api/realtime/calls/", service.handleCallSocket)
	mux.HandleFunc("/api/platform/settings", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			httpx.Write(w, r, http.StatusOK, service.PlatformSettings())
		case http.MethodPost:
			user, ok := service.authService.UserFromRequest(r)
			if !ok {
				httpx.Error(w, r, http.StatusUnauthorized, "not_authenticated", "Sign in is required.")
				return
			}
			if !requireAnyPermission(w, r, user, "platform.settings.manage") {
				return
			}
			var payload PlatformSettings
			if !httpx.DecodeJSON(w, r, &payload) {
				return
			}
			settings, err := service.UpdatePlatformSettings(payload, user)
			writeResult(w, r, settings, err)
		default:
			httpx.Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "This platform settings action is not available.")
		}
	})

	mux.HandleFunc("/api/workspace", func(w http.ResponseWriter, r *http.Request) {
		if !httpx.RequireMethod(w, r, http.MethodGet) {
			return
		}
		user, ok := service.authService.UserFromRequest(r)
		if !ok {
			httpx.Error(w, r, http.StatusUnauthorized, "not_authenticated", "Sign in is required.")
			return
		}
		httpx.Write(w, r, http.StatusOK, paginateDashboard(service.DashboardForUser(user), defaultWorkspacePageSize))
	})

	mux.HandleFunc("/api/workspace/page", func(w http.ResponseWriter, r *http.Request) {
		if !httpx.RequireMethod(w, r, http.MethodGet) {
			return
		}
		user, ok := service.authService.UserFromRequest(r)
		if !ok {
			httpx.Error(w, r, http.StatusUnauthorized, "not_authenticated", "Sign in is required.")
			return
		}
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		page, found := workspacePage(service.DashboardForUser(user), r.URL.Query().Get("scope"), offset, limit)
		if !found {
			httpx.Error(w, r, http.StatusBadRequest, "invalid_scope", "Choose a valid workspace list.")
			return
		}
		httpx.Write(w, r, http.StatusOK, page)
	})

	mux.HandleFunc("/api/workflow/", func(w http.ResponseWriter, r *http.Request) {
		user, ok := service.authService.UserFromRequest(r)
		if !ok {
			httpx.Error(w, r, http.StatusUnauthorized, "not_authenticated", "Sign in is required.")
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/api/workflow/")
		switch {
		case path == "quotes" && r.Method == http.MethodPost:
			if !requireAnyPermission(w, r, user, "quotes.create", "quotes.manage") {
				return
			}
			var payload QuoteRequest
			if !httpx.DecodeJSON(w, r, &payload) {
				return
			}
			quote, err := service.CreateQuote(payload, user)
			writeResult(w, r, quote, err)
		case strings.HasPrefix(path, "quotes/") && strings.HasSuffix(path, "/price") && r.Method == http.MethodPost:
			if !requireAnyPermission(w, r, user, "quotes.manage") {
				return
			}
			var payload PriceQuoteRequest
			if !httpx.DecodeJSON(w, r, &payload) {
				return
			}
			quote, err := service.PriceQuote(routeID(path, "quotes", "price"), payload, user)
			writeResult(w, r, quote, err)
		case strings.HasPrefix(path, "quotes/") && strings.HasSuffix(path, "/accept") && r.Method == http.MethodPost:
			if !requireAnyPermission(w, r, user, "quotes.create", "quotes.manage") {
				return
			}
			order, err := service.AcceptQuote(routeID(path, "quotes", "accept"), user)
			writeResult(w, r, order, err)
		case strings.HasPrefix(path, "quotes/") && strings.HasSuffix(path, "/reject") && r.Method == http.MethodPost:
			if !requireAnyPermission(w, r, user, "quotes.create", "quotes.manage") {
				return
			}
			quote, err := service.RejectQuote(routeID(path, "quotes", "reject"), user)
			writeResult(w, r, quote, err)
		case strings.HasPrefix(path, "manufacturing/") && strings.HasSuffix(path, "/edit") && r.Method == http.MethodPost:
			if !requireAnyPermission(w, r, user, "manufacturing.manage") {
				return
			}
			var payload OrderEditRequest
			if !httpx.DecodeJSON(w, r, &payload) {
				return
			}
			order, err := service.EditOrder(routeID(path, "manufacturing", "edit"), payload, user)
			writeResult(w, r, order, err)
		case strings.HasPrefix(path, "manufacturing/") && strings.HasSuffix(path, "/cancel") && r.Method == http.MethodPost:
			if !requireAnyPermission(w, r, user, "manufacturing.manage") {
				return
			}
			var payload CancelOrderRequest
			if !httpx.DecodeJSON(w, r, &payload) {
				return
			}
			order, err := service.CancelOrder(routeID(path, "manufacturing", "cancel"), payload, user)
			writeResult(w, r, order, err)
		case strings.HasPrefix(path, "manufacturing/") && strings.HasSuffix(path, "/update") && r.Method == http.MethodPost:
			if !requireAnyPermission(w, r, user, "manufacturing.manage") {
				return
			}
			var payload OrderUpdateRequest
			if !httpx.DecodeJSON(w, r, &payload) {
				return
			}
			order, err := service.UpdateOrder(routeID(path, "manufacturing", "update"), payload, user)
			writeResult(w, r, order, err)
		case strings.HasPrefix(path, "manufacturing/") && strings.HasSuffix(path, "/stage") && r.Method == http.MethodPost:
			if !requireAnyPermission(w, r, user, "manufacturing.manage") {
				return
			}
			var payload struct {
				Stage string `json:"stage"`
			}
			if !httpx.DecodeJSON(w, r, &payload) {
				return
			}
			order, err := service.MoveOrder(routeID(path, "manufacturing", "stage"), payload.Stage, user)
			writeResult(w, r, order, err)
		case path == "ratesheets" && r.Method == http.MethodPost:
			if !requireAnyPermission(w, r, user, "shipping.manage") {
				return
			}
			var payload RateSheetRequest
			if !httpx.DecodeJSON(w, r, &payload) {
				return
			}
			rate, err := service.SaveRateSheet(payload, user)
			writeResult(w, r, rate, err)
		case path == "shipping" && r.Method == http.MethodPost:
			if !requireAnyPermission(w, r, user, "shipping.create", "shipping.manage") {
				return
			}
			var payload ShippingRequestPayload
			if !httpx.DecodeJSON(w, r, &payload) {
				return
			}
			shipping, err := service.CreateShipping(payload, user)
			writeResult(w, r, shipping, err)
		case strings.HasPrefix(path, "shipping/") && strings.HasSuffix(path, "/dispatch") && r.Method == http.MethodPost:
			if !requireAnyPermission(w, r, user, "shipping.manage") {
				return
			}
			var payload struct {
				Tracking string `json:"tracking"`
			}
			if r.ContentLength != 0 && !httpx.DecodeJSON(w, r, &payload) {
				return
			}
			shipping, err := service.DispatchShipping(routeID(path, "shipping", "dispatch"), payload.Tracking, user)
			writeResult(w, r, shipping, err)
		case strings.HasPrefix(path, "shipping/") && strings.HasSuffix(path, "/update") && r.Method == http.MethodPost:
			if !requireAnyPermission(w, r, user, "shipping.manage") {
				return
			}
			var payload ShippingUpdatePayload
			if !httpx.DecodeJSON(w, r, &payload) {
				return
			}
			shipping, err := service.UpdateShipping(routeID(path, "shipping", "update"), payload, user)
			writeResult(w, r, shipping, err)
		case path == "payments" && r.Method == http.MethodPost:
			if !requireAnyPermission(w, r, user, "payments.create", "payments.manage") {
				return
			}
			var payload PaymentRequest
			if !httpx.DecodeJSON(w, r, &payload) {
				return
			}
			payment, err := service.CreatePayment(payload, user)
			writeResult(w, r, payment, err)
		case strings.HasPrefix(path, "payments/") && strings.HasSuffix(path, "/reverse") && r.Method == http.MethodPost:
			if !requireAnyPermission(w, r, user, "payments.manage") {
				return
			}
			var payload PaymentReversalRequest
			if !httpx.DecodeJSON(w, r, &payload) {
				return
			}
			result, err := service.ReversePayment(routeID(path, "payments", "reverse"), payload, user)
			writeResult(w, r, result, err)
		case strings.HasPrefix(path, "payments/") && strings.HasSuffix(path, "/confirm") && r.Method == http.MethodPost:
			if !requireAnyPermission(w, r, user, "payments.manage") {
				return
			}
			result, err := service.ConfirmPayment(routeID(path, "payments", "confirm"), user)
			writeResult(w, r, result, err)
		case path == "products" && r.Method == http.MethodPost:
			if !requireAnyPermission(w, r, user, "products.manage", "orders.read") {
				return
			}
			var payload ProductRequest
			if !httpx.DecodeJSON(w, r, &payload) {
				return
			}
			product, err := service.CreateProduct(payload, user)
			writeResult(w, r, product, err)
		case path == "notices" && r.Method == http.MethodPost:
			if !canManageCustomerNotices(user) {
				httpx.Error(w, r, http.StatusForbidden, "forbidden", "You do not have permission for this action.")
				return
			}
			var payload CustomerNoticeRequest
			if !httpx.DecodeJSON(w, r, &payload) {
				return
			}
			notice, err := service.CreateCustomerNotice(payload, user)
			writeResult(w, r, notice, err)
		case strings.HasPrefix(path, "notices/") && strings.HasSuffix(path, "/update") && r.Method == http.MethodPost:
			if !canManageCustomerNotices(user) {
				httpx.Error(w, r, http.StatusForbidden, "forbidden", "You do not have permission for this action.")
				return
			}
			var payload CustomerNoticeRequest
			if !httpx.DecodeJSON(w, r, &payload) {
				return
			}
			notice, err := service.UpdateCustomerNotice(routeID(path, "notices", "update"), payload, user)
			writeResult(w, r, notice, err)
		case strings.HasPrefix(path, "notices/") && strings.HasSuffix(path, "/delete") && r.Method == http.MethodPost:
			if !canManageCustomerNotices(user) {
				httpx.Error(w, r, http.StatusForbidden, "forbidden", "You do not have permission for this action.")
				return
			}
			notice, err := service.DeleteCustomerNotice(routeID(path, "notices", "delete"), user)
			writeResult(w, r, notice, err)
		case path == "featured" && r.Method == http.MethodPost:
			if !canManageCustomerNotices(user) {
				httpx.Error(w, r, http.StatusForbidden, "forbidden", "You do not have permission for this action.")
				return
			}
			var payload FeaturedProductRequest
			if !httpx.DecodeJSON(w, r, &payload) {
				return
			}
			item, err := service.CreateFeaturedProduct(payload, user)
			writeResult(w, r, item, err)
		case strings.HasPrefix(path, "featured/") && strings.HasSuffix(path, "/update") && r.Method == http.MethodPost:
			if !canManageCustomerNotices(user) {
				httpx.Error(w, r, http.StatusForbidden, "forbidden", "You do not have permission for this action.")
				return
			}
			var payload FeaturedProductRequest
			if !httpx.DecodeJSON(w, r, &payload) {
				return
			}
			item, err := service.UpdateFeaturedProduct(routeID(path, "featured", "update"), payload, user)
			writeResult(w, r, item, err)
		case strings.HasPrefix(path, "featured/") && strings.HasSuffix(path, "/delete") && r.Method == http.MethodPost:
			if !canManageCustomerNotices(user) {
				httpx.Error(w, r, http.StatusForbidden, "forbidden", "You do not have permission for this action.")
				return
			}
			item, err := service.DeleteFeaturedProduct(routeID(path, "featured", "delete"), user)
			writeResult(w, r, item, err)
		case path == "messages" && r.Method == http.MethodPost:
			if !requireAnyPermission(w, r, user, "chat.use", "chat.manage") {
				return
			}
			var payload MessageRequest
			if !httpx.DecodeJSON(w, r, &payload) {
				return
			}
			conversation, err := service.SendMessage(payload, user)
			writeResult(w, r, conversation, err)
		case strings.HasPrefix(path, "messages/") && strings.HasSuffix(path, "/page") && r.Method == http.MethodGet:
			if !requireAnyPermission(w, r, user, "chat.use", "chat.manage") {
				return
			}
			offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
			limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
			page, err := service.ConversationMessages(routeID(path, "messages", "page"), offset, limit, user)
			writeResult(w, r, page, err)
		case strings.HasPrefix(path, "messages/") && strings.HasSuffix(path, "/read") && r.Method == http.MethodPost:
			if !requireAnyPermission(w, r, user, "chat.use", "chat.manage") {
				return
			}
			conversation, err := service.MarkConversationRead(routeID(path, "messages", "read"), user)
			writeResult(w, r, conversation, err)
		case path == "calls" && r.Method == http.MethodPost:
			if !requireAnyPermission(w, r, user, "calls.start", "calls.manage") {
				return
			}
			var payload CallRequestPayload
			if !httpx.DecodeJSON(w, r, &payload) {
				return
			}
			call, err := service.StartCall(payload, user)
			writeResult(w, r, call, err)
		case path == "calls/config" && r.Method == http.MethodGet:
			if !requireAnyPermission(w, r, user, "calls.start", "calls.manage") {
				return
			}
			httpx.Write(w, r, http.StatusOK, service.CallConfig(user))
		case strings.HasPrefix(path, "calls/") && strings.HasSuffix(path, "/signals") && r.Method == http.MethodGet:
			if !requireAnyPermission(w, r, user, "calls.start", "calls.manage") {
				return
			}
			after, _ := strconv.Atoi(r.URL.Query().Get("after"))
			signals, err := service.ListCallSignals(routeID(path, "calls", "signals"), after, user)
			writeResult(w, r, signals, err)
		case strings.HasPrefix(path, "calls/") && strings.HasSuffix(path, "/signals") && r.Method == http.MethodPost:
			if !requireAnyPermission(w, r, user, "calls.start", "calls.manage") {
				return
			}
			var payload CallSignalRequest
			if !httpx.DecodeJSON(w, r, &payload) {
				return
			}
			signal, err := service.SendCallSignal(routeID(path, "calls", "signals"), payload, user)
			writeResult(w, r, signal, err)
		case strings.HasPrefix(path, "calls/") && strings.HasSuffix(path, "/heartbeat") && r.Method == http.MethodPost:
			if !requireAnyPermission(w, r, user, "calls.start", "calls.manage") {
				return
			}
			call, err := service.HeartbeatCall(routeID(path, "calls", "heartbeat"), user)
			writeResult(w, r, call, err)
		case strings.HasPrefix(path, "calls/") && strings.HasSuffix(path, "/end") && r.Method == http.MethodPost:
			if !requireAnyPermission(w, r, user, "calls.start", "calls.manage") {
				return
			}
			call, err := service.EndCall(routeID(path, "calls", "end"), user)
			writeResult(w, r, call, err)
		case strings.HasPrefix(path, "calls/") && strings.HasSuffix(path, "/status") && r.Method == http.MethodPost:
			if !requireAnyPermission(w, r, user, "calls.start", "calls.manage") {
				return
			}
			var payload CallActionRequest
			if !httpx.DecodeJSON(w, r, &payload) {
				return
			}
			call, err := service.UpdateCallStatus(routeID(path, "calls", "status"), payload, user)
			writeResult(w, r, call, err)
		default:
			httpx.Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "This workflow action is not available.")
		}
	})
}

var errValidation = strconv.ErrSyntax
var errNotFound = strconv.ErrRange
var errForbidden = auth.ErrForbidden
var errInvalidCallTransition = errors.New("invalid call transition")
var errConfirmationRequired = errors.New("order confirmation required")
var errOrderClosed = errors.New("order is closed")
var errCalleeOffline = errors.New("callee offline")
var errCallBusy = errors.New("call busy")

type orderPaymentRequiredError struct {
	Kind   string
	Amount int
}

func (e *orderPaymentRequiredError) Error() string {
	return "order payment required"
}

func writeResult(w http.ResponseWriter, r *http.Request, data interface{}, err error) {
	if errors.Is(err, errPersistence) {
		// Durable financial state must be committed before success is reported.
		httpx.Error(w, r, http.StatusServiceUnavailable, "storage_unavailable", "Changes could not be saved. Please try again.")
		return
	}
	if err == nil {
		httpx.Write(w, r, http.StatusOK, data)
		return
	}
	if errors.Is(err, errAccountingReview) || errors.Is(err, errPaymentConflict) {
		httpx.Error(w, r, http.StatusConflict, "accounting_conflict", err.Error())
		return
	}
	if errors.Is(err, errNotFound) {
		httpx.Error(w, r, http.StatusNotFound, "not_found", "Requested workflow item was not found.")
		return
	}
	if errors.Is(err, errForbidden) {
		httpx.Error(w, r, http.StatusForbidden, "forbidden", "You do not have access to this workflow action.")
		return
	}
	if errors.Is(err, errInvalidCallTransition) {
		httpx.Error(w, r, http.StatusConflict, "call_state_changed", "This call can no longer perform that action.")
		return
	}
	if errors.Is(err, errCalleeOffline) {
		httpx.Error(w, r, http.StatusConflict, "callee_offline", "The other person is offline right now.")
		return
	}
	if errors.Is(err, errCallBusy) {
		httpx.Error(w, r, http.StatusConflict, "call_busy", "There is already an active call for this account.")
		return
	}
	if errors.Is(err, errConfirmationRequired) {
		httpx.Error(w, r, http.StatusConflict, "confirmation_required", "Type the exact order ID to confirm this action.")
		return
	}
	if errors.Is(err, errOrderClosed) {
		httpx.Error(w, r, http.StatusConflict, "order_closed", "Completed or cancelled orders cannot be changed.")
		return
	}
	var paymentRequired *orderPaymentRequiredError
	if errors.As(err, &paymentRequired) {
		if paymentRequired.Kind == "deposit" {
			httpx.Error(w, r, http.StatusConflict, "deposit_required", fmt.Sprintf("Rs %d deposit is still due. Confirm the deposit before updating production.", paymentRequired.Amount))
			return
		}
		httpx.Error(w, r, http.StatusConflict, "balance_due", fmt.Sprintf("Rs %d is still due on this order. Confirm the final payment before marking it complete.", paymentRequired.Amount))
		return
	}
	httpx.Error(w, r, http.StatusBadRequest, "validation_failed", "Please complete the required fields correctly.")
}

func requireAnyPermission(w http.ResponseWriter, r *http.Request, user auth.User, permissions ...string) bool {
	if auth.HasAnyPermission(user, permissions...) {
		return true
	}
	httpx.Error(w, r, http.StatusForbidden, "forbidden", "You do not have access to this workflow action.")
	return false
}

func (s *Service) nextID(prefix string) string {
	day := time.Now().UTC().Format("20060102")
	if s.lastSequenceDay != day {
		s.lastSequenceDay = day
		s.sequence = 0
	}
	s.sequence++
	return prefix + "-" + day[2:] + "-" + strconv.Itoa(1000+s.sequence)
}

func (s *Service) productName(id string) string {
	for _, product := range s.products {
		if product.ID == id {
			return product.Name
		}
	}
	return ""
}

func (s *Service) publishEmailEvent(event EmailEvent) {
	publisher := s.emailEventPublisher
	if publisher == nil {
		return
	}
	event.Data = copyStringMap(event.Data)
	// Always async: Publish resolves recipients via PlatformSettings(), which needs
	// s.mu. Calling it while a workflow mutation already holds s.mu deadlocks the API
	// (seen when sending a formal quotation).
	go func(ev EmailEvent) {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Printf("email event publisher panic trigger=%s entity=%s: %v", ev.Trigger, ev.EntityID, recovered)
			}
		}()
		publisher(ev)
	}(event)
}

func copyStringMap(values map[string]string) map[string]string {
	copyValues := make(map[string]string, len(values))
	for key, value := range values {
		copyValues[key] = value
	}
	return copyValues
}

func (s *Service) record(actor auth.User, action string, entity string, detail string) {
	if s.recorder != nil {
		s.recorder.Record(actorName(actor, "system"), action, entity, detail)
	}
}

func routeID(path string, prefix string, suffix string) string {
	path = strings.TrimPrefix(path, prefix+"/")
	path = strings.TrimSuffix(path, "/"+suffix)
	return strings.Trim(path, "/")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func actorName(user auth.User, fallback string) string {
	if strings.TrimSpace(user.Name) != "" {
		return user.Name
	}
	if strings.TrimSpace(user.Email) != "" {
		return user.Email
	}
	return fallback
}

func supportAuthorRole(user auth.User) string {
	if auth.IsCustomerRole(user) {
		return "Customer"
	}
	return firstNonEmpty(user.Role, "Admin")
}

func appendConversationMessage(conversation Conversation, message Message, actor auth.User) Conversation {
	conversation = normalizeConversation(conversation)
	message.AuthorRole = firstNonEmpty(message.AuthorRole, supportAuthorRole(actor))
	conversation.Messages = append(conversation.Messages, message)
	conversation.Status = firstNonEmpty(conversation.Status, "Open")
	conversation.LastMessageAt = message.CreatedAt
	conversation.LastAuthor = firstNonEmpty(message.AuthorRole, message.Author)
	if auth.IsCustomerRole(actor) {
		conversation.UnreadForAdmin++
		conversation.UnreadForCustomer = 0
	} else {
		conversation.UnreadForCustomer++
		conversation.UnreadForAdmin = 0
	}
	return conversation
}

func normalizeConversation(conversation Conversation) Conversation {
	conversation.Status = firstNonEmpty(conversation.Status, "Open")
	conversation.Subject = firstNonEmpty(conversation.Subject, "Support conversation")
	for index := range conversation.Messages {
		if conversation.Messages[index].AuthorRole == "" {
			conversation.Messages[index].AuthorRole = roleFromAuthor(conversation.Messages[index].Author)
		}
	}
	if len(conversation.Messages) > 0 {
		last := conversation.Messages[len(conversation.Messages)-1]
		conversation.LastMessageAt = firstNonEmpty(conversation.LastMessageAt, last.CreatedAt)
		conversation.LastAuthor = firstNonEmpty(conversation.LastAuthor, firstNonEmpty(last.AuthorRole, last.Author))
	}
	return conversation
}

func compactConversations(conversations []Conversation, limit int) []Conversation {
	result := make([]Conversation, len(conversations))
	for index, conversation := range conversations {
		result[index] = compactConversation(conversation, 0, limit)
	}
	return result
}

func compactConversation(conversation Conversation, offset, limit int) Conversation {
	conversation = normalizeConversation(conversation)
	if offset < 0 {
		offset = 0
	}
	if limit < 1 {
		limit = defaultWorkspacePageSize
	}
	if limit > maxWorkspacePageSize {
		limit = maxWorkspacePageSize
	}
	total := len(conversation.Messages)
	if offset > total {
		offset = total
	}
	end := total - offset
	start := end - limit
	if start < 0 {
		start = 0
	}
	conversation.Messages = append([]Message(nil), conversation.Messages[start:end]...)
	loaded := offset + len(conversation.Messages)
	conversation.MessagePagination = PageInfo{Loaded: loaded, Total: total, HasMore: loaded < total}
	return conversation
}

func roleFromAuthor(author string) string {
	if strings.Contains(strings.ToLower(author), "admin") || strings.Contains(strings.ToLower(author), "support") {
		return "Admin"
	}
	return "Customer"
}

func normalizeCallStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "ringing", "calling":
		return "Ringing"
	case "in call", "active", "answered", "picked":
		return "In call"
	case "completed", "complete", "done", "ended":
		return "Completed"
	case "missed", "no answer", "waiting", "queued", "queue":
		return "Missed"
	case "declined", "rejected":
		return "Declined"
	case "cancelled", "canceled", "cancel":
		return "Cancelled"
	default:
		return ""
	}
}

func normalizeCall(call CallRequest) CallRequest {
	legacyStatus := strings.ToLower(strings.TrimSpace(call.Status))
	call.Status = firstNonEmpty(normalizeCallStatus(call.Status), "Missed")
	call.Subject = firstNonEmpty(call.Subject, "Audio call")
	call.CallType = firstNonEmpty(call.CallType, "audio")
	call.InitiatorName = firstNonEmpty(call.InitiatorName, "Customer")
	call.InitiatorRole = firstNonEmpty(call.InitiatorRole, roleFromAuthor(call.InitiatorName))
	if call.RecipientName == "" {
		if call.InitiatorRole == "Customer" {
			call.RecipientName = "ChakuChuri Support"
		} else {
			call.RecipientName = "Customer"
		}
	}
	if call.Status == "In call" {
		call.RoomID = firstNonEmpty(call.RoomID, callRoomID(call.ID))
	}
	call.UpdatedAt = firstNonEmpty(call.UpdatedAt, call.CreatedAt)
	if call.Status == "Ringing" && call.RingExpiresAt == "" {
		if createdAt, err := time.Parse(time.RFC3339, call.CreatedAt); err == nil {
			call.RingExpiresAt = createdAt.Add(defaultRingTimeout).Format(time.RFC3339)
		}
	}
	if (legacyStatus == "waiting" || legacyStatus == "queued" || legacyStatus == "queue") && call.EndedAt == "" {
		call.EndedAt = firstNonEmpty(call.UpdatedAt, call.CreatedAt)
	}
	if call.History == nil {
		call.History = []Update{}
	}
	if isTerminalCallStatus(call.Status) {
		call.DurationSeconds = callDurationSeconds(call)
	}
	return call
}

// ExpireRingingCalls marks unanswered ringing calls as Missed and fans the
// updates out on the workspace realtime channel so clients stop ringing.
func (s *Service) ExpireRingingCalls() int {
	s.mu.Lock()
	expired := s.expireRingingCallsLocked(time.Now().UTC())
	if len(expired) > 0 {
		s.persistLocked()
	}
	s.mu.Unlock()
	for _, call := range expired {
		s.notifyWorkspaceUpsert("calls", call.CustomerID, call)
	}
	return len(expired)
}

func (s *Service) expireRingingCallsLocked(now time.Time) []CallRequest {
	expired := make([]CallRequest, 0)
	for index := range s.calls {
		s.calls[index] = normalizeCall(s.calls[index])
		call := s.calls[index]
		if call.Status != "Ringing" {
			continue
		}
		expiresAt, err := time.Parse(time.RFC3339, call.RingExpiresAt)
		if err != nil || now.Before(expiresAt) {
			continue
		}
		endedAt := now.Format(time.RFC3339)
		call.Status = "Missed"
		call.UpdatedAt = endedAt
		call.EndedAt = endedAt
		call.DurationSeconds = callDurationSeconds(call)
		call.History = append([]Update{{
			Label: "Missed call", Detail: "The call was not answered.", Actor: "System", CreatedAt: endedAt,
		}}, call.History...)
		s.calls[index] = call
		expired = append(expired, call)
	}
	return expired
}

func (s *Service) conversationBelongsToCustomerLocked(conversationID string, customerID string) bool {
	for _, conversation := range s.conversations {
		if conversation.ID == conversationID {
			return conversation.CustomerID == customerID
		}
	}
	return false
}

func directCallRole(actor auth.User) string {
	if auth.IsCustomerRole(actor) {
		return "Customer"
	}
	return "Admin"
}

func callDurationSeconds(call CallRequest) int {
	if call.DurationSeconds > 0 {
		return call.DurationSeconds
	}
	startedAt, startErr := time.Parse(time.RFC3339, call.StartedAt)
	endedAt, endErr := time.Parse(time.RFC3339, call.EndedAt)
	if startErr != nil || endErr != nil || endedAt.Before(startedAt) {
		return 0
	}
	return int(endedAt.Sub(startedAt).Round(time.Second) / time.Second)
}

func callStatusLabel(status string) string {
	switch normalizeCallStatus(status) {
	case "In call":
		return "Call answered"
	case "Completed":
		return "Call completed"
	case "Missed":
		return "Missed call"
	case "Declined":
		return "Call declined"
	case "Cancelled":
		return "Call cancelled"
	default:
		return "Call updated"
	}
}

func callStatusDetail(status string) string {
	switch normalizeCallStatus(status) {
	case "In call":
		return "The audio call was answered."
	case "Completed":
		return "The audio call ended."
	case "Missed":
		return "The call was not answered."
	case "Declined":
		return "The incoming call was declined."
	case "Cancelled":
		return "The outgoing call was cancelled."
	default:
		return "Call status updated."
	}
}

func (s *Service) findCallLocked(id string) (int, CallRequest, bool) {
	id = strings.TrimSpace(id)
	for index := range s.calls {
		if s.calls[index].ID == id {
			s.calls[index] = normalizeCall(s.calls[index])
			return index, s.calls[index], true
		}
	}
	return 0, CallRequest{}, false
}

func canUseCall(actor auth.User, call CallRequest) bool {
	return auth.CanAccessCustomer(actor, call.CustomerID) && auth.HasAnyPermission(actor, "calls.start", "calls.manage")
}

func callRoomID(callID string) string {
	return "room_" + safeID(callID)
}

func validSignalType(signalType string) bool {
	switch strings.ToLower(strings.TrimSpace(signalType)) {
	case "offer", "answer", "ice", "candidate", "media-state", "reconnect-request":
		return true
	default:
		return false
	}
}

func trimCallSignals(rows []CallSignal, limit int) []CallSignal {
	if limit < 1 || len(rows) == 0 {
		return []CallSignal{}
	}
	if len(rows) <= limit {
		return append([]CallSignal(nil), rows...)
	}
	return append([]CallSignal(nil), rows[len(rows)-limit:]...)
}

func isTerminalCallStatus(status string) bool {
	status = normalizeCallStatus(status)
	return status == "Completed" || status == "Missed" || status == "Declined" || status == "Cancelled"
}

func callSortRank(call CallRequest) int {
	if call.Status == "In call" {
		return 0
	}
	if call.Status == "Ringing" {
		return 1
	}
	return 2
}

func safeID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	builder := strings.Builder{}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') {
			builder.WriteRune(char)
		} else if char == ' ' || char == '-' || char == '_' {
			builder.WriteRune('_')
		}
	}
	cleaned := strings.Trim(builder.String(), "_")
	if cleaned == "" {
		return "item"
	}
	return cleaned
}

func markStage(stages []Stage, name string, status string, date string) {
	for index := range stages {
		if stages[index].Name == name {
			stages[index].Status = status
			stages[index].Date = date
		}
	}
}

func (s *Service) postLedgerEntryLocked(actor auth.User, customerID string, sourceType string, sourceID string, debit int, credit int, note string, postedAt string) {
	if debit <= 0 && credit <= 0 {
		return
	}
	if ledgerHasEntry(s.ledger, sourceType, sourceID, debit, credit) {
		return
	}
	entryNo := s.nextID("LED")
	s.ledger = append([]LedgerEntry{{
		ID:         entryNo,
		CustomerID: customerID,
		EntryNo:    entryNo,
		SourceType: sourceType,
		SourceID:   sourceID,
		Debit:      debit,
		Credit:     credit,
		Currency:   "PKR",
		Note:       note,
		PostedAt:   firstNonEmpty(postedAt, time.Now().UTC().Format(time.RFC3339)),
	}}, s.ledger...)
}

func mergedLedgerEntries(stored []LedgerEntry, orders []ManufacturingOrder, shipping []ShippingRequest, payments []Payment) []LedgerEntry {
	entries := make([]LedgerEntry, 0, len(stored)+len(orders)+len(shipping)+len(payments))
	for _, entry := range stored {
		entry = normalizeLedgerEntry(entry)
		if entry.SourceType == "" || entry.SourceID == "" || (entry.Debit <= 0 && entry.Credit <= 0) {
			continue
		}
		entries = append(entries, entry)
	}
	for _, order := range orders {
		if order.TotalAmount < 1 || ledgerHasSource(entries, "manufacturing_order", order.ID) {
			continue
		}
		// Reconstruct original debit as current total minus posted adjustments (never invent current total alone).
		adjustmentNet := 0
		for _, entry := range entries {
			if entry.SourceType == "manufacturing_adjustment" && strings.HasPrefix(entry.SourceID, order.ID+"-") {
				adjustmentNet += entry.Debit - entry.Credit
			}
		}
		originalDebit := order.TotalAmount - adjustmentNet
		if originalDebit < 1 {
			continue
		}
		entries = append(entries, ledgerFromSource(order.CustomerID, "manufacturing_order", order.ID, originalDebit, 0, "Manufacturing order total posted.", order.CreatedAt))
	}
	for _, shipment := range shipping {
		if shipment.QuotedAmount > 0 && !ledgerHasSource(entries, "shipping_request", shipment.ID) {
			entries = append(entries, ledgerFromSource(shipment.CustomerID, "shipping_request", shipment.ID, shipment.QuotedAmount, 0, "Shipping service charge posted.", shipment.CreatedAt))
		}
	}
	for _, payment := range payments {
		if payment.Amount > 0 && payment.Status == "Confirmed" && !ledgerHasSource(entries, "payment", payment.ID) {
			entries = append(entries, ledgerFromSource(payment.CustomerID, "payment", payment.ID, 0, payment.Amount, "Payment confirmed by company.", firstNonEmpty(payment.ConfirmedAt, payment.CreatedAt)))
		}
	}
	sort.SliceStable(entries, func(i int, j int) bool {
		return entries[i].PostedAt > entries[j].PostedAt
	})
	return entries
}

func ledgerFromSource(customerID string, sourceType string, sourceID string, debit int, credit int, note string, postedAt string) LedgerEntry {
	key := ledgerKey(sourceType, sourceID, debit, credit)
	return LedgerEntry{
		ID:         "ledger_" + safeID(key),
		CustomerID: customerID,
		EntryNo:    strings.ToUpper("LED-" + safeID(sourceType+"-"+sourceID)),
		SourceType: sourceType,
		SourceID:   sourceID,
		Debit:      debit,
		Credit:     credit,
		Currency:   "PKR",
		Note:       note,
		PostedAt:   firstNonEmpty(postedAt, time.Now().UTC().Format(time.RFC3339)),
	}
}

func normalizeLedgerEntry(entry LedgerEntry) LedgerEntry {
	entry.CustomerID = strings.TrimSpace(entry.CustomerID)
	entry.SourceType = strings.TrimSpace(entry.SourceType)
	entry.SourceID = strings.TrimSpace(entry.SourceID)
	entry.Currency = firstNonEmpty(entry.Currency, "PKR")
	entry.PostedAt = firstNonEmpty(entry.PostedAt, time.Now().UTC().Format(time.RFC3339))
	if entry.EntryNo == "" {
		entry.EntryNo = strings.ToUpper("LED-" + safeID(entry.SourceType+"-"+entry.SourceID))
	}
	if entry.ID == "" {
		entry.ID = "ledger_" + safeID(ledgerKey(entry.SourceType, entry.SourceID, entry.Debit, entry.Credit))
	}
	return entry
}

func ledgerHasSource(entries []LedgerEntry, sourceType string, sourceID string) bool {
	sourceType = strings.ToLower(strings.TrimSpace(sourceType))
	sourceID = strings.TrimSpace(sourceID)
	for _, entry := range entries {
		if strings.ToLower(strings.TrimSpace(entry.SourceType)) == sourceType && strings.TrimSpace(entry.SourceID) == sourceID {
			return true
		}
	}
	return false
}

func findLedgerEntry(entries []LedgerEntry, sourceType string, sourceID string) (LedgerEntry, bool) {
	for _, entry := range entries {
		if strings.EqualFold(strings.TrimSpace(entry.SourceType), strings.TrimSpace(sourceType)) && strings.TrimSpace(entry.SourceID) == strings.TrimSpace(sourceID) {
			return entry, true
		}
	}
	return LedgerEntry{}, false
}

func ledgerHasEntry(entries []LedgerEntry, sourceType string, sourceID string, debit int, credit int) bool {
	key := ledgerKey(sourceType, sourceID, debit, credit)
	for _, entry := range entries {
		if ledgerKey(entry.SourceType, entry.SourceID, entry.Debit, entry.Credit) == key {
			return true
		}
	}
	return false
}

func ledgerKey(sourceType string, sourceID string, debit int, credit int) string {
	return strings.ToLower(strings.TrimSpace(sourceType)) + ":" + strings.TrimSpace(sourceID) + ":" + strconv.Itoa(debit) + ":" + strconv.Itoa(credit)
}

func ledgerBalance(entries []LedgerEntry) int {
	balance := 0
	for _, entry := range entries {
		balance += entry.Debit - entry.Credit
	}
	return balance
}
func orderIsClosed(order ManufacturingOrder) bool {
	return strings.EqualFold(order.Status, "Completed") || strings.EqualFold(order.Status, "Cancelled")
}

func findOrderByQuote(orders []ManufacturingOrder, quoteID string) (ManufacturingOrder, bool) {
	quoteID = strings.TrimSpace(quoteID)
	if quoteID == "" {
		return ManufacturingOrder{}, false
	}
	for _, order := range orders {
		if order.QuotationID == quoteID || order.ID == quoteID {
			return order, true
		}
	}
	return ManufacturingOrder{}, false
}

func findOrderByID(orders []ManufacturingOrder, orderID string) (ManufacturingOrder, bool) {
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return ManufacturingOrder{}, false
	}
	for _, order := range orders {
		if order.ID == orderID {
			return order, true
		}
	}
	return ManufacturingOrder{}, false
}

func (s *Service) manufacturingCustomerID(id string) (string, bool) {
	id = strings.TrimSpace(id)
	for _, order := range s.manufacturing {
		if order.ID == id {
			return order.CustomerID, true
		}
	}
	return "", false
}

func (s *Service) shippingCustomerID(id string) (string, bool) {
	id = strings.TrimSpace(id)
	for _, shipment := range s.shipping {
		if shipment.ID == id {
			return shipment.CustomerID, true
		}
	}
	return "", false
}

func visibleCustomer(customerID string, all bool, allowed map[string]bool) bool {
	if all {
		return true
	}
	return allowed[strings.TrimSpace(customerID)]
}
func filterProducts(rows []Product, all bool, allowed map[string]bool) []Product {
	result := []Product{}
	for _, row := range rows {
		if visibleCustomer(row.CustomerID, all, allowed) {
			result = append(result, row)
		}
	}
	return result
}

func filterQuotes(rows []Quotation, all bool, allowed map[string]bool) []Quotation {
	result := []Quotation{}
	for _, row := range rows {
		if visibleCustomer(row.CustomerID, all, allowed) {
			result = append(result, row)
		}
	}
	return result
}

func filterOrders(rows []ManufacturingOrder, all bool, allowed map[string]bool) []ManufacturingOrder {
	result := []ManufacturingOrder{}
	for _, row := range rows {
		if visibleCustomer(row.CustomerID, all, allowed) {
			result = append(result, row)
		}
	}
	return result
}

func filterShipping(rows []ShippingRequest, all bool, allowed map[string]bool) []ShippingRequest {
	result := []ShippingRequest{}
	for _, row := range rows {
		if visibleCustomer(row.CustomerID, all, allowed) {
			result = append(result, row)
		}
	}
	return result
}

func filterPayments(rows []Payment, all bool, allowed map[string]bool) []Payment {
	result := []Payment{}
	for _, row := range rows {
		if visibleCustomer(row.CustomerID, all, allowed) {
			result = append(result, row)
		}
	}
	return result
}

func filterLedger(rows []LedgerEntry, all bool, allowed map[string]bool) []LedgerEntry {
	result := []LedgerEntry{}
	for _, row := range rows {
		if visibleCustomer(row.CustomerID, all, allowed) {
			result = append(result, row)
		}
	}
	return result
}
func filterConversations(rows []Conversation, all bool, allowed map[string]bool) []Conversation {
	result := []Conversation{}
	for _, row := range rows {
		if visibleCustomer(row.CustomerID, all, allowed) {
			result = append(result, normalizeConversation(row))
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].LastMessageAt > result[j].LastMessageAt
	})
	return result
}

func filterCalls(rows []CallRequest, all bool, allowed map[string]bool) []CallRequest {
	result := []CallRequest{}
	for _, row := range rows {
		if visibleCustomer(row.CustomerID, all, allowed) {
			result = append(result, normalizeCall(row))
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		left := result[i]
		right := result[j]
		if callSortRank(left) != callSortRank(right) {
			return callSortRank(left) < callSortRank(right)
		}
		return firstNonEmpty(left.UpdatedAt, left.CreatedAt) > firstNonEmpty(right.UpdatedAt, right.CreatedAt)
	})
	return result
}

func buildMetrics(quotes []Quotation, orders []ManufacturingOrder, shipping []ShippingRequest, payments []Payment, ledger []LedgerEntry, calls []CallRequest) map[string]interface{} {
	pendingQuotes := 0
	activeOrders := 0
	pendingPayments := 0
	balance := 0
	progressTotal := 0
	for _, quote := range quotes {
		if quote.Status == "Requested" || quote.Status == "Priced" {
			pendingQuotes++
		}
	}
	for _, order := range orders {
		if order.Status != "Completed" && order.Status != "Cancelled" {
			activeOrders++
			balance += order.BalanceDue
			progressTotal += order.Progress
		}
	}
	for _, payment := range payments {
		if payment.Status == "Waiting confirmation" {
			pendingPayments++
		}
	}
	averageProgress := 0
	if activeOrders > 0 {
		averageProgress = progressTotal / activeOrders
	}
	accountDue := balance
	if len(ledger) > 0 {
		accountDue = ledgerBalance(ledger)
		if accountDue < 0 {
			accountDue = 0
		}
	}
	metrics := map[string]interface{}{
		"pendingQuotes":   pendingQuotes,
		"activeOrders":    activeOrders,
		"openShipments":   len(shipping),
		"pendingPayments": pendingPayments,
		"balanceDue":      accountDue,
		"ledgerBalance":   ledgerBalance(ledger),
		"ringingCalls":    callStatusCount(calls, "Ringing"),
		"missedCalls":     callStatusCount(calls, "Missed"),
		"activeCalls":     callStatusCount(calls, "In call"),
		"averageProgress": averageProgress,
	}
	addAccountingMetrics(metrics, ledger, payments)
	return metrics
}

func callStatusCount(calls []CallRequest, status string) int {
	count := 0
	for _, call := range calls {
		if normalizeCallStatus(call.Status) == status {
			count++
		}
	}
	return count
}
