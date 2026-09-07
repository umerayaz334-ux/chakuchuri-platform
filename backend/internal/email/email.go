package email

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"chakuchuri/backend/internal/audit"
	"chakuchuri/backend/internal/auth"
)

const (
	StatusQueued    = "Queued"
	StatusScheduled = "Scheduled"
	StatusSending   = "Sending"
	StatusCaptured  = "Captured"
	StatusFailed    = "Failed"
	StatusCancelled = "Cancelled"
)

var (
	errNotFound     = errors.New("email record not found")
	errValidation   = errors.New("email validation failed")
	errDuplicate    = errors.New("email event already queued")
	variablePattern = regexp.MustCompile(`\{\{\s*([a-z0-9_]+)\s*\}\}`)
)

type SenderSettings struct {
	Provider           string `json:"provider"`
	Mode               string `json:"mode"`
	FromName           string `json:"fromName"`
	FromEmail          string `json:"fromEmail"`
	ReplyTo            string `json:"replyTo"`
	Configured         bool   `json:"configured"`
	DomainStatus       string `json:"domainStatus"`
	WebhookStatus      string `json:"webhookStatus"`
	LastVerifiedAt     string `json:"lastVerifiedAt,omitempty"`
	UpdatedAt          string `json:"updatedAt,omitempty"`
	UpdatedBy          string `json:"updatedBy,omitempty"`
	SMTPHost           string `json:"smtpHost,omitempty"`
	SMTPPort           int    `json:"smtpPort,omitempty"`
	SMTPUsername       string `json:"smtpUsername,omitempty"`
	SMTPPassword       string `json:"smtpPassword,omitempty"`
	SMTPImplicitTLS    bool   `json:"smtpImplicitTls,omitempty"`
	PasswordConfigured bool   `json:"passwordConfigured"`
}

type Template struct {
	ID        string   `json:"id"`
	Key       string   `json:"key"`
	Name      string   `json:"name"`
	Category  string   `json:"category"`
	Language  string   `json:"language"`
	Subject   string   `json:"subject"`
	Body      string   `json:"body"`
	Variables []string `json:"variables"`
	Enabled   bool     `json:"enabled"`
	Version   int      `json:"version"`
	UpdatedAt string   `json:"updatedAt"`
	UpdatedBy string   `json:"updatedBy"`
}

type AutomationRule struct {
	ID            string `json:"id"`
	Trigger       string `json:"trigger"`
	Label         string `json:"label"`
	Category      string `json:"category"`
	TemplateKey   string `json:"templateKey"`
	Enabled       bool   `json:"enabled"`
	Timing        string `json:"timing"`
	DelayMinutes  int    `json:"delayMinutes"`
	MaxReminders  int    `json:"maxReminders"`
	Essential     bool   `json:"essential"`
	LastTriggered string `json:"lastTriggered,omitempty"`
	UpdatedAt     string `json:"updatedAt"`
	UpdatedBy     string `json:"updatedBy"`
}

type OutboxItem struct {
	ID               string            `json:"id"`
	DeliveryID       string            `json:"deliveryId"`
	EventKey         string            `json:"eventKey"`
	Trigger          string            `json:"trigger"`
	CustomerID       string            `json:"customerId,omitempty"`
	EntityID         string            `json:"entityId,omitempty"`
	Recipient        string            `json:"recipient"`
	RecipientName    string            `json:"recipientName"`
	TemplateKey      string            `json:"templateKey"`
	TemplateLanguage string            `json:"templateLanguage"`
	Payload          map[string]string `json:"payload"`
	Status           string            `json:"status"`
	Attempts         int               `json:"attempts"`
	ScheduledAt      string            `json:"scheduledAt"`
	CreatedAt        string            `json:"createdAt"`
	UpdatedAt        string            `json:"updatedAt"`
	LastError        string            `json:"lastError,omitempty"`
}

type Delivery struct {
	ID                string `json:"id"`
	OutboxID          string `json:"outboxId"`
	EventKey          string `json:"eventKey"`
	Trigger           string `json:"trigger"`
	CustomerID        string `json:"customerId,omitempty"`
	EntityID          string `json:"entityId,omitempty"`
	Recipient         string `json:"recipient"`
	RecipientName     string `json:"recipientName"`
	TemplateKey       string `json:"templateKey"`
	TemplateLanguage  string `json:"templateLanguage"`
	TemplateVersion   int    `json:"templateVersion"`
	Subject           string `json:"subject"`
	Status            string `json:"status"`
	Provider          string `json:"provider"`
	ProviderMessageID string `json:"providerMessageId,omitempty"`
	Attempts          int    `json:"attempts"`
	CreatedAt         string `json:"createdAt"`
	ScheduledAt       string `json:"scheduledAt"`
	SentAt            string `json:"sentAt,omitempty"`
	LastError         string `json:"lastError,omitempty"`
}

type Metrics struct {
	Queued       int     `json:"queued"`
	Scheduled    int     `json:"scheduled"`
	Captured     int     `json:"captured"`
	Failed       int     `json:"failed"`
	Cancelled    int     `json:"cancelled"`
	SuccessRate  float64 `json:"successRate"`
	LastActivity string  `json:"lastActivity,omitempty"`
}

type Snapshot struct {
	Settings   SenderSettings      `json:"settings"`
	Templates  []Template          `json:"templates"`
	Rules      []AutomationRule    `json:"rules"`
	Outbox     []OutboxItem        `json:"outbox"`
	Deliveries []Delivery          `json:"deliveries"`
	Metrics    Metrics             `json:"metrics"`
	Pagination map[string]PageInfo `json:"pagination"`
}

type PageInfo struct {
	Loaded  int  `json:"loaded"`
	Total   int  `json:"total"`
	HasMore bool `json:"hasMore"`
}

type Recipient struct {
	CustomerID  string
	Name        string
	CompanyName string
	Email       string
	Language    string
}

type BusinessEvent struct {
	EventKey   string
	Trigger    string
	CustomerID string
	EntityID   string
	Data       map[string]string
}

type TemplateUpdate struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
	Enabled bool   `json:"enabled"`
}

type RuleUpdate struct {
	Enabled      bool   `json:"enabled"`
	Timing       string `json:"timing"`
	DelayMinutes int    `json:"delayMinutes"`
	MaxReminders int    `json:"maxReminders"`
}

type SettingsUpdate struct {
	FromName          string `json:"fromName"`
	FromEmail         string `json:"fromEmail"`
	ReplyTo           string `json:"replyTo"`
	Provider          string `json:"provider"`
	SMTPHost          string `json:"smtpHost"`
	SMTPPort          int    `json:"smtpPort"`
	SMTPUsername      string `json:"smtpUsername"`
	SMTPPassword      string `json:"smtpPassword"`
	SMTPImplicitTLS   bool   `json:"smtpImplicitTls"`
	ClearSMTPPassword bool   `json:"clearSmtpPassword"`
}

type PreviewRequest struct {
	TemplateKey string            `json:"templateKey"`
	Language    string            `json:"language"`
	Subject     string            `json:"subject,omitempty"`
	Body        string            `json:"body,omitempty"`
	Context     map[string]string `json:"context"`
}

type Preview struct {
	Subject          string   `json:"subject"`
	Body             string   `json:"body"`
	HtmlBody         string   `json:"htmlBody"`
	MissingVariables []string `json:"missingVariables"`
	TemplateVersion  int      `json:"templateVersion"`
}

type TestRequest struct {
	Recipient   string `json:"recipient"`
	TemplateKey string `json:"templateKey"`
	Language    string `json:"language"`
}

type Message struct {
	FromName  string
	FromEmail string
	ReplyTo   string
	ToName    string
	ToEmail   string
	Subject   string
	TextBody  string
	HtmlBody  string
}

type SendResult struct {
	MessageID string
	Status    string
}

type Mailer interface {
	Name() string
	Verify(context.Context) error
	Send(context.Context, Message) (SendResult, error)
}

type DevelopmentMailer struct{}

func (DevelopmentMailer) Name() string { return "Development sink" }

func (DevelopmentMailer) Verify(context.Context) error { return nil }

func (DevelopmentMailer) Send(_ context.Context, message Message) (SendResult, error) {
	if !looksLikeEmail(message.ToEmail) || strings.TrimSpace(message.Subject) == "" || strings.TrimSpace(message.TextBody) == "" {
		return SendResult{}, errValidation
	}
	return SendResult{
		MessageID: "sink_" + time.Now().UTC().Format("20060102T150405.000000000"),
		Status:    StatusCaptured,
	}, nil
}

type Service struct {
	mu                sync.RWMutex
	authService       *auth.Service
	recorder          *audit.Recorder
	repository        Repository
	mailer            Mailer
	recipientResolver func(string) (Recipient, bool)
	portalURL         string
	supportEmail      string
	supportPhone      string
	settings          SenderSettings
	templates         []Template
	rules             []AutomationRule
	outbox            []OutboxItem
	deliveries        []Delivery
	wake              chan struct{}
	startOnce         sync.Once
}

const staleSendingAfter = 5 * time.Minute

func NewService(authService *auth.Service, recorder *audit.Recorder) *Service {
	now := time.Now().UTC().Format(time.RFC3339)
	return &Service{
		authService:  authService,
		recorder:     recorder,
		mailer:       DevelopmentMailer{},
		portalURL:    "http://127.0.0.1:5170",
		supportEmail: "support@chakuchuri.pk",
		settings: SenderSettings{
			Provider:      "development",
			Mode:          "Development",
			FromName:      "ChakuChuri.pk",
			FromEmail:     "notifications@chakuchuri.pk",
			ReplyTo:       "support@chakuchuri.pk",
			Configured:    true,
			DomainStatus:  "Development sink — messages are captured locally, not delivered to real inboxes",
			WebhookStatus: "Not required for development sink",
			UpdatedAt:     now,
			UpdatedBy:     "system",
		},
		templates: defaultTemplates(now),
		rules:     defaultRules(now),
		wake:      make(chan struct{}, 1),
	}
}

func (s *Service) SetRecipientResolver(resolver func(string) (Recipient, bool)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recipientResolver = resolver
}

func (s *Service) SetDeliveryDefaults(portalURL, supportEmail, supportPhone string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if value := strings.TrimRight(strings.TrimSpace(portalURL), "/"); value != "" {
		s.portalURL = value
	}
	if value := strings.ToLower(strings.TrimSpace(supportEmail)); value != "" {
		s.supportEmail = value
	}
	s.supportPhone = strings.TrimSpace(supportPhone)
}

func (s *Service) Start(ctx context.Context) {
	s.startOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(15 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
				case <-s.wake:
				}
				s.ProcessDue(ctx, 20)
			}
		}()
	})
}

func (s *Service) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshotLocked()
}

func (s *Service) UpdateTemplate(id string, payload TemplateUpdate, actor auth.User) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	subject := strings.TrimSpace(payload.Subject)
	body := strings.TrimSpace(payload.Body)
	if subject == "" || body == "" || len(subject) > 180 || len(body) > 12000 {
		return Snapshot{}, errValidation
	}
	if invalid := invalidVariables(subject + "\n" + body); len(invalid) > 0 {
		return Snapshot{}, fmt.Errorf("%w: unsupported variable %s", errValidation, invalid[0])
	}
	for index := range s.templates {
		if s.templates[index].ID != strings.TrimSpace(id) {
			continue
		}
		now := time.Now().UTC().Format(time.RFC3339)
		s.templates[index].Subject = subject
		s.templates[index].Body = body
		s.templates[index].Enabled = payload.Enabled
		s.templates[index].Version++
		s.templates[index].Variables = variablesIn(subject + "\n" + body)
		s.templates[index].UpdatedAt = now
		s.templates[index].UpdatedBy = actorName(actor)
		s.persistLocked()
		s.record(actor, "email.template_updated", id, "Email template version updated.")
		return s.snapshotLocked(), nil
	}
	return Snapshot{}, errNotFound
}

func (s *Service) UpdateRule(id string, payload RuleUpdate, actor auth.User) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	timing := normalizeTiming(payload.Timing)
	if timing == "" || timing == "Manual approval" || payload.DelayMinutes < 0 || payload.DelayMinutes > 525600 || payload.MaxReminders < 0 || payload.MaxReminders > 12 {
		return Snapshot{}, errValidation
	}
	delayMinutes := payload.DelayMinutes
	if timing == "Immediate" {
		delayMinutes = 0
	}
	for index := range s.rules {
		if s.rules[index].ID != strings.TrimSpace(id) {
			continue
		}
		s.rules[index].Enabled = payload.Enabled
		s.rules[index].Timing = timing
		s.rules[index].DelayMinutes = delayMinutes
		s.rules[index].MaxReminders = payload.MaxReminders
		s.rules[index].UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		s.rules[index].UpdatedBy = actorName(actor)
		s.persistLocked()
		s.record(actor, "email.automation_updated", id, "Email automation rule updated.")
		return s.snapshotLocked(), nil
	}
	return Snapshot{}, errNotFound
}

func (s *Service) UpdateSettings(payload SettingsUpdate, actor auth.User) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	fromName := strings.TrimSpace(payload.FromName)
	fromEmail := strings.ToLower(strings.TrimSpace(payload.FromEmail))
	replyTo := strings.ToLower(strings.TrimSpace(payload.ReplyTo))
	provider := normalizeProvider(payload.Provider)
	if provider == "" {
		provider = normalizeProvider(s.settings.Provider)
	}
	if provider == "" {
		provider = "development"
	}
	if fromName == "" || !looksLikeEmail(fromEmail) || !looksLikeEmail(replyTo) {
		return Snapshot{}, errValidation
	}

	s.settings.FromName = fromName
	s.settings.FromEmail = fromEmail
	s.settings.ReplyTo = replyTo
	s.settings.Provider = provider
	s.settings.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	s.settings.UpdatedBy = actorName(actor)

	if provider == "smtp" {
		host := strings.TrimSpace(payload.SMTPHost)
		port := payload.SMTPPort
		if port < 1 {
			port = 587
		}
		username := strings.TrimSpace(payload.SMTPUsername)
		if host == "" || port > 65535 {
			return Snapshot{}, errValidation
		}
		s.settings.Mode = "SMTP"
		s.settings.SMTPHost = host
		s.settings.SMTPPort = port
		s.settings.SMTPUsername = username
		s.settings.SMTPImplicitTLS = payload.SMTPImplicitTLS || port == 465
		if payload.ClearSMTPPassword {
			s.settings.SMTPPassword = ""
		} else if strings.TrimSpace(payload.SMTPPassword) != "" {
			s.settings.SMTPPassword = payload.SMTPPassword
		}
		if strings.TrimSpace(s.settings.SMTPPassword) == "" && username != "" {
			// Password can be empty for open relays, but warn via status.
			s.settings.DomainStatus = "SMTP configured — verify before sending"
		} else {
			s.settings.DomainStatus = "SMTP ready — verify connection"
		}
		s.settings.WebhookStatus = "Not required for SMTP"
		s.settings.Configured = true
	} else {
		s.settings.Mode = "Development"
		s.settings.DomainStatus = "Development sink — messages are captured locally, not delivered to real inboxes"
		s.settings.WebhookStatus = "Not required for development sink"
		s.settings.Configured = true
	}

	s.rebuildMailerLocked()
	s.persistLocked()
	s.record(actor, "email.sender_updated", "email-settings", "Email sender and connection settings updated.")
	return s.snapshotLocked(), nil
}

func (s *Service) VerifyConnection(ctx context.Context, actor auth.User) (Snapshot, error) {
	s.mu.RLock()
	mailer := s.mailer
	s.mu.RUnlock()
	if mailer == nil {
		return Snapshot{}, errValidation
	}
	if err := mailer.Verify(ctx); err != nil {
		s.record(actor, "email.connection_failed", "email-settings", "Email provider verification failed.")
		return Snapshot{}, fmt.Errorf("%w: %s", errValidation, safeProviderError(err))
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.settings.Configured = true
	s.settings.LastVerifiedAt = time.Now().UTC().Format(time.RFC3339)
	s.settings.UpdatedAt = s.settings.LastVerifiedAt
	s.settings.UpdatedBy = actorName(actor)
	if s.settings.Provider == "smtp" {
		s.settings.DomainStatus = "SMTP verified"
	}
	s.persistLocked()
	s.record(actor, "email.connection_verified", "email-settings", "Email provider connection verified.")
	return s.snapshotLocked(), nil
}

func (s *Service) rebuildMailerLocked() {
	if s.settings.Provider == "smtp" {
		s.mailer = SMTPMailer{
			Host:        s.settings.SMTPHost,
			Port:        s.settings.SMTPPort,
			Username:    s.settings.SMTPUsername,
			Password:    s.settings.SMTPPassword,
			From:        s.settings.FromEmail,
			ImplicitTLS: s.settings.SMTPImplicitTLS || s.settings.SMTPPort == 465,
		}
		return
	}
	s.mailer = DevelopmentMailer{}
}

func (s *Service) publicSettingsLocked() SenderSettings {
	out := s.settings
	out.PasswordConfigured = strings.TrimSpace(s.settings.SMTPPassword) != ""
	out.SMTPPassword = ""
	return out
}

func normalizeProvider(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "development", "dev", "sink":
		return "development"
	case "smtp":
		return "smtp"
	default:
		return ""
	}
}

func (s *Service) Preview(payload PreviewRequest) (Preview, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	template, ok := s.findTemplateLocked(payload.TemplateKey, payload.Language)
	if !ok {
		return Preview{}, errNotFound
	}
	if subject := strings.TrimSpace(payload.Subject); subject != "" {
		template.Subject = subject
	}
	if body := strings.TrimSpace(payload.Body); body != "" {
		template.Body = body
	}
	contextValues := sampleContext()
	contextValues["portal_url"] = firstNonEmpty(s.portalURL, contextValues["portal_url"])
	contextValues["support_email"] = firstNonEmpty(s.supportEmail, s.settings.ReplyTo, contextValues["support_email"])
	if s.supportPhone != "" {
		contextValues["support_phone"] = s.supportPhone
	}
	for key, value := range payload.Context {
		contextValues[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	subject, missingSubject := render(template.Subject, contextValues)
	body, missingBody := render(template.Body, contextValues)
	return Preview{
		Subject:          subject,
		Body:             body,
		HtmlBody:         buildEmailHTML(s.settings.FromName, subject, body, template.Key, contextValues),
		MissingVariables: uniqueSorted(append(missingSubject, missingBody...)),
		TemplateVersion:  template.Version,
	}, nil
}

func (s *Service) SendTest(ctx context.Context, payload TestRequest, actor auth.User) (Snapshot, error) {
	recipient := strings.ToLower(strings.TrimSpace(payload.Recipient))
	if recipient == "" {
		recipient = strings.ToLower(strings.TrimSpace(actor.Email))
	}
	if !looksLikeEmail(recipient) {
		return Snapshot{}, errValidation
	}

	s.mu.Lock()
	template, ok := s.findTemplateLocked(payload.TemplateKey, payload.Language)
	if !ok || !template.Enabled {
		s.mu.Unlock()
		return Snapshot{}, errNotFound
	}
	now := time.Now().UTC()
	item := s.enqueueLocked(BusinessEvent{
		EventKey: "test:" + actor.ID + ":" + now.Format("20060102T150405.000000000"),
		Trigger:  "test.send",
		EntityID: actor.ID,
		Data:     s.previewContextLocked(Recipient{Name: actorName(actor), CompanyName: "ChakuChuri.pk", Email: recipient}),
	}, Recipient{Name: actorName(actor), CompanyName: "ChakuChuri.pk", Email: recipient, Language: normalizeLanguage(payload.Language)}, template.Key, now)
	s.persistLocked()
	s.mu.Unlock()

	s.processItem(ctx, item.ID)
	s.record(actor, "email.test_sent", item.ID, "Test email sent to the development delivery sink.")
	return s.Snapshot(), nil
}

func (s *Service) Publish(event BusinessEvent) (OutboxItem, error) {
	event.EventKey = strings.TrimSpace(event.EventKey)
	event.Trigger = strings.TrimSpace(event.Trigger)
	event.CustomerID = strings.TrimSpace(event.CustomerID)
	event.EntityID = strings.TrimSpace(event.EntityID)
	if event.EventKey == "" || event.Trigger == "" || event.CustomerID == "" {
		return OutboxItem{}, errValidation
	}

	s.mu.Lock()
	for _, item := range s.outbox {
		if item.EventKey == event.EventKey {
			s.mu.Unlock()
			return item, errDuplicate
		}
	}
	rule, ok := s.findRuleLocked(event.Trigger)
	if !ok || !rule.Enabled {
		s.mu.Unlock()
		return OutboxItem{}, nil
	}
	resolver := s.recipientResolver
	templateKey := rule.TemplateKey
	ruleID := rule.ID
	ruleDelayMinutes := rule.DelayMinutes
	ruleTiming := rule.Timing
	maxReminders := rule.MaxReminders
	s.mu.Unlock()

	// Resolve recipients outside email.mu — resolver may call workflow.PlatformSettings().
	if resolver == nil {
		return OutboxItem{}, errValidation
	}
	recipient, ok := resolver(event.CustomerID)
	if !ok || !looksLikeEmail(recipient.Email) {
		return OutboxItem{}, errValidation
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	language := normalizeLanguage(recipient.Language)
	template, ok := s.findTemplateLocked(templateKey, language)
	if !ok || !template.Enabled {
		return OutboxItem{}, errNotFound
	}
	now := time.Now().UTC()
	scheduledAt := now.Add(time.Duration(ruleDelayMinutes) * time.Minute)
	if ruleTiming == "Immediate" {
		scheduledAt = now
	}
	contextValues := s.liveContextLocked(recipient)
	for key, value := range event.Data {
		trimmedKey := strings.TrimSpace(key)
		trimmedValue := strings.TrimSpace(value)
		if trimmedKey == "" || trimmedValue == "" {
			continue
		}
		contextValues[trimmedKey] = trimmedValue
	}
	event.Data = contextValues

	if event.Trigger == "payment.due" {
		orderID := firstNonEmpty(event.Data["order_id"], event.EntityID)
		if maxReminders < 1 {
			maxReminders = 1
		}
		if s.paymentDueSendCountLocked(orderID) >= maxReminders {
			return OutboxItem{}, nil
		}
	}
	if event.Trigger == "payment.confirmed" {
		cancelIDs := splitCSV(event.Data["cancel_reminder_orders"])
		if len(cancelIDs) == 0 {
			cancelIDs = splitCSV(event.Data["order_ids"])
			if orderID := strings.TrimSpace(event.Data["order_id"]); orderID != "" {
				cancelIDs = append(cancelIDs, orderID)
			}
			cancelIDs = splitCSV(strings.Join(cancelIDs, ","))
		}
		for _, orderID := range cancelIDs {
			s.cancelPendingPaymentRemindersLocked(orderID, now, "Balance cleared after payment confirmation.")
		}
	}
	if event.Trigger == "order.cancelled" {
		s.cancelPendingForOrderLocked(firstNonEmpty(event.Data["order_id"], event.EntityID), now, "Order was cancelled.")
	}
	item := s.enqueueLocked(event, recipient, template.Key, scheduledAt)
	for index := range s.rules {
		if s.rules[index].ID == ruleID {
			s.rules[index].LastTriggered = now.Format(time.RFC3339)
		}
	}
	s.persistLocked()
	s.recordSystem("email.event_queued", event.EntityID, "Email event queued: "+event.Trigger+".")
	s.signalWorker()
	return item, nil
}

func (s *Service) CancelOutbox(id string, actor auth.User) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for index := range s.outbox {
		item := &s.outbox[index]
		if item.ID != strings.TrimSpace(id) {
			continue
		}
		if item.Status != StatusQueued && item.Status != StatusScheduled {
			return Snapshot{}, errValidation
		}
		item.Status = StatusCancelled
		item.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		s.updateDeliveryLocked(item.DeliveryID, func(delivery *Delivery) {
			delivery.Status = StatusCancelled
		})
		s.persistLocked()
		s.record(actor, "email.delivery_cancelled", item.ID, "Scheduled email was cancelled.")
		return s.snapshotLocked(), nil
	}
	return Snapshot{}, errNotFound
}

func (s *Service) RetryDelivery(id string, actor auth.User) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for deliveryIndex := range s.deliveries {
		delivery := &s.deliveries[deliveryIndex]
		if delivery.ID != strings.TrimSpace(id) {
			continue
		}
		if delivery.Status != StatusFailed && delivery.Status != StatusSending {
			return Snapshot{}, errValidation
		}
		for outboxIndex := range s.outbox {
			item := &s.outbox[outboxIndex]
			if item.ID != delivery.OutboxID {
				continue
			}
			now := time.Now().UTC().Format(time.RFC3339)
			item.Status = StatusQueued
			item.ScheduledAt = now
			item.UpdatedAt = now
			item.LastError = ""
			delivery.Status = StatusQueued
			delivery.ScheduledAt = now
			delivery.LastError = ""
			s.persistLocked()
			s.record(actor, "email.delivery_retried", delivery.ID, "Failed email was queued for retry.")
			s.signalWorker()
			return s.snapshotLocked(), nil
		}
		return Snapshot{}, errNotFound
	}
	return Snapshot{}, errNotFound
}

func (s *Service) ProcessDue(ctx context.Context, limit int) {
	if limit < 1 {
		limit = 1
	}
	s.mu.Lock()
	reclaimed := s.reclaimStaleSendingLocked(time.Now().UTC())
	if reclaimed {
		s.persistLocked()
	}
	s.mu.Unlock()
	for processed := 0; processed < limit; processed++ {
		id := s.claimDue()
		if id == "" {
			return
		}
		s.processClaimed(ctx, id)
	}
}

func (s *Service) claimDue() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	for index := range s.outbox {
		item := &s.outbox[index]
		if item.Status != StatusQueued && item.Status != StatusScheduled {
			continue
		}
		due, err := time.Parse(time.RFC3339, item.ScheduledAt)
		if err != nil {
			item.Status = StatusFailed
			item.LastError = "Invalid scheduled time on outbox item."
			item.UpdatedAt = now.Format(time.RFC3339)
			s.updateDeliveryLocked(item.DeliveryID, func(delivery *Delivery) {
				delivery.Status = StatusFailed
				delivery.LastError = item.LastError
			})
			s.persistLocked()
			continue
		}
		if due.After(now) {
			continue
		}
		item.Status = StatusSending
		item.Attempts++
		item.UpdatedAt = now.Format(time.RFC3339)
		s.updateDeliveryLocked(item.DeliveryID, func(delivery *Delivery) {
			delivery.Status = StatusSending
			delivery.Attempts = item.Attempts
		})
		s.persistLocked()
		return item.ID
	}
	return ""
}

func (s *Service) processItem(ctx context.Context, id string) {
	s.mu.Lock()
	for index := range s.outbox {
		if s.outbox[index].ID == id && (s.outbox[index].Status == StatusQueued || s.outbox[index].Status == StatusScheduled) {
			s.outbox[index].Status = StatusSending
			s.outbox[index].Attempts++
			s.updateDeliveryLocked(s.outbox[index].DeliveryID, func(delivery *Delivery) {
				delivery.Status = StatusSending
				delivery.Attempts = s.outbox[index].Attempts
			})
			break
		}
	}
	s.persistLocked()
	s.mu.Unlock()
	s.processClaimed(ctx, id)
}

func (s *Service) processClaimed(ctx context.Context, id string) {
	s.mu.RLock()
	var item OutboxItem
	for _, row := range s.outbox {
		if row.ID == id {
			item = copyOutbox(row)
			break
		}
	}
	template, templateFound := s.findTemplateLocked(item.TemplateKey, item.TemplateLanguage)
	settings := s.settings
	mailer := s.mailer
	s.mu.RUnlock()
	if item.ID == "" {
		return
	}

	var sendResult SendResult
	var sendErr error
	if !templateFound || !template.Enabled {
		sendErr = errors.New("template is unavailable")
	} else {
		subject, missingSubject := render(template.Subject, item.Payload)
		body, missingBody := render(template.Body, item.Payload)
		missing := uniqueSorted(append(missingSubject, missingBody...))
		if len(missing) > 0 {
			sendErr = fmt.Errorf("required template data missing: %s", strings.Join(missing, ", "))
		} else {
			sendResult, sendErr = mailer.Send(ctx, Message{
				FromName: settings.FromName, FromEmail: settings.FromEmail, ReplyTo: settings.ReplyTo,
				ToName: item.RecipientName, ToEmail: item.Recipient, Subject: subject, TextBody: body,
				HtmlBody: buildEmailHTML(settings.FromName, subject, body, template.Key, item.Payload),
			})
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	for index := range s.outbox {
		row := &s.outbox[index]
		if row.ID != id {
			continue
		}
		if sendErr == nil {
			row.Status = firstNonEmpty(sendResult.Status, StatusCaptured)
			row.LastError = ""
		} else {
			row.Status = StatusFailed
			row.LastError = safeProviderError(sendErr)
		}
		row.UpdatedAt = now.Format(time.RFC3339)
		s.updateDeliveryLocked(row.DeliveryID, func(delivery *Delivery) {
			delivery.Status = row.Status
			delivery.Attempts = row.Attempts
			delivery.LastError = row.LastError
			if sendErr == nil {
				delivery.ProviderMessageID = sendResult.MessageID
				delivery.SentAt = now.Format(time.RFC3339)
			}
		})
		break
	}
	s.persistLocked()
	if sendErr == nil {
		s.recordSystem("email.delivery_captured", item.ID, "Email captured by "+mailer.Name()+".")
	} else {
		s.recordSystem("email.delivery_failed", item.ID, "Email delivery failed without logging message contents.")
	}
}

func (s *Service) enqueueLocked(event BusinessEvent, recipient Recipient, templateKey string, scheduledAt time.Time) OutboxItem {
	now := time.Now().UTC()
	language := normalizeLanguage(recipient.Language)
	template, _ := s.findTemplateLocked(templateKey, language)
	status := StatusQueued
	if scheduledAt.After(now.Add(time.Second)) {
		status = StatusScheduled
	}
	sequence := strconv.Itoa(len(s.outbox) + len(s.deliveries) + 1)
	stamp := now.Format("20060102T150405.000000000")
	outboxID := "eml_out_" + stamp + "_" + sequence
	deliveryID := "eml_del_" + stamp + "_" + sequence
	item := OutboxItem{
		ID: outboxID, DeliveryID: deliveryID, EventKey: event.EventKey, Trigger: event.Trigger,
		CustomerID: event.CustomerID, EntityID: event.EntityID, Recipient: strings.ToLower(strings.TrimSpace(recipient.Email)),
		RecipientName: firstNonEmpty(recipient.Name, recipient.CompanyName, recipient.Email), TemplateKey: templateKey,
		TemplateLanguage: language, Payload: copyMap(event.Data), Status: status, ScheduledAt: scheduledAt.Format(time.RFC3339),
		CreatedAt: now.Format(time.RFC3339), UpdatedAt: now.Format(time.RFC3339),
	}
	s.outbox = append([]OutboxItem{item}, s.outbox...)
	s.deliveries = append([]Delivery{{
		ID: deliveryID, OutboxID: outboxID, EventKey: event.EventKey, Trigger: event.Trigger,
		CustomerID: event.CustomerID, EntityID: event.EntityID, Recipient: item.Recipient, RecipientName: item.RecipientName,
		TemplateKey: templateKey, TemplateLanguage: language, TemplateVersion: template.Version,
		Subject: renderLoose(template.Subject, item.Payload), Status: status, Provider: s.mailer.Name(),
		CreatedAt: item.CreatedAt, ScheduledAt: item.ScheduledAt,
	}}, s.deliveries...)
	return item
}

func (s *Service) cancelPendingPaymentRemindersLocked(orderID string, now time.Time, reason string) {
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return
	}
	for index := range s.outbox {
		item := &s.outbox[index]
		if item.Trigger != "payment.due" || (item.Status != StatusQueued && item.Status != StatusScheduled && item.Status != StatusSending) {
			continue
		}
		if item.Payload["order_id"] != orderID && item.EntityID != orderID {
			continue
		}
		item.Status = StatusCancelled
		item.LastError = reason
		item.UpdatedAt = now.Format(time.RFC3339)
		s.updateDeliveryLocked(item.DeliveryID, func(delivery *Delivery) {
			delivery.Status = StatusCancelled
			delivery.LastError = reason
		})
	}
}

func (s *Service) cancelPendingForOrderLocked(orderID string, now time.Time, reason string) {
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return
	}
	for index := range s.outbox {
		item := &s.outbox[index]
		if item.Status != StatusQueued && item.Status != StatusScheduled && item.Status != StatusSending {
			continue
		}
		if item.Payload["order_id"] != orderID && item.EntityID != orderID {
			continue
		}
		item.Status = StatusCancelled
		item.LastError = reason
		item.UpdatedAt = now.Format(time.RFC3339)
		s.updateDeliveryLocked(item.DeliveryID, func(delivery *Delivery) {
			delivery.Status = StatusCancelled
			delivery.LastError = reason
		})
	}
}

func (s *Service) reclaimStaleSendingLocked(now time.Time) bool {
	changed := false
	for index := range s.outbox {
		item := &s.outbox[index]
		if item.Status != StatusSending {
			continue
		}
		updated, err := time.Parse(time.RFC3339, item.UpdatedAt)
		if err != nil || now.Sub(updated) < staleSendingAfter {
			continue
		}
		item.Status = StatusQueued
		item.ScheduledAt = now.Format(time.RFC3339)
		item.UpdatedAt = now.Format(time.RFC3339)
		item.LastError = "Reclaimed after interrupted send."
		s.updateDeliveryLocked(item.DeliveryID, func(delivery *Delivery) {
			delivery.Status = StatusQueued
			delivery.ScheduledAt = item.ScheduledAt
			delivery.LastError = item.LastError
		})
		changed = true
	}
	return changed
}

func (s *Service) paymentDueSendCountLocked(orderID string) int {
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return 0
	}
	count := 0
	for _, delivery := range s.deliveries {
		if delivery.Trigger != "payment.due" || delivery.EntityID != orderID {
			continue
		}
		switch delivery.Status {
		case StatusCaptured, StatusFailed, StatusQueued, StatusScheduled, StatusSending:
			count++
		}
	}
	return count
}

func (s *Service) updateDeliveryLocked(id string, update func(*Delivery)) {
	for index := range s.deliveries {
		if s.deliveries[index].ID == id {
			update(&s.deliveries[index])
			return
		}
	}
}

func (s *Service) findTemplateLocked(key string, language string) (Template, bool) {
	key = strings.TrimSpace(key)
	language = normalizeLanguage(language)
	for _, template := range s.templates {
		if template.Key == key && template.Language == language {
			return template, true
		}
	}
	if language != "en" {
		for _, template := range s.templates {
			if template.Key == key && template.Language == "en" {
				return template, true
			}
		}
	}
	return Template{}, false
}

func (s *Service) findRuleLocked(trigger string) (AutomationRule, bool) {
	for _, rule := range s.rules {
		if rule.Trigger == strings.TrimSpace(trigger) {
			return rule, true
		}
	}
	return AutomationRule{}, false
}

func (s *Service) snapshotLocked() Snapshot {
	templates := append([]Template{}, s.templates...)
	rules := append([]AutomationRule{}, s.rules...)
	outbox := make([]OutboxItem, 0, len(s.outbox))
	for _, item := range s.outbox {
		outbox = append(outbox, copyOutbox(item))
	}
	deliveries := append([]Delivery{}, s.deliveries...)
	sort.SliceStable(templates, func(i, j int) bool {
		if templates[i].Category == templates[j].Category {
			if templates[i].Name == templates[j].Name {
				return templates[i].Language < templates[j].Language
			}
			return templates[i].Name < templates[j].Name
		}
		return templates[i].Category < templates[j].Category
	})
	sort.SliceStable(rules, func(i, j int) bool { return rules[i].Label < rules[j].Label })
	sort.SliceStable(outbox, func(i, j int) bool { return outbox[i].CreatedAt > outbox[j].CreatedAt })
	sort.SliceStable(deliveries, func(i, j int) bool { return deliveries[i].CreatedAt > deliveries[j].CreatedAt })
	if len(outbox) > 250 {
		outbox = outbox[:250]
	}
	if len(deliveries) > 500 {
		deliveries = deliveries[:500]
	}
	return Snapshot{
		Settings: s.publicSettingsLocked(), Templates: templates, Rules: rules, Outbox: outbox, Deliveries: deliveries,
		Metrics: metricsFor(deliveries),
	}
}

func metricsFor(deliveries []Delivery) Metrics {
	metrics := Metrics{}
	completed := 0
	for _, delivery := range deliveries {
		switch delivery.Status {
		case StatusQueued:
			metrics.Queued++
		case StatusScheduled:
			metrics.Scheduled++
		case StatusSending:
			metrics.Queued++
		case StatusCaptured:
			metrics.Captured++
			completed++
		case StatusFailed:
			metrics.Failed++
			completed++
		case StatusCancelled:
			metrics.Cancelled++
		}
		if delivery.CreatedAt > metrics.LastActivity {
			metrics.LastActivity = delivery.CreatedAt
		}
	}
	if completed > 0 {
		metrics.SuccessRate = float64(metrics.Captured) / float64(completed) * 100
	}
	return metrics
}

func defaultRules(now string) []AutomationRule {
	return []AutomationRule{
		{ID: "rule_quote_priced", Trigger: "quote.priced", Label: "Quotation ready", Category: "Quotations", TemplateKey: "quote_priced", Enabled: true, Timing: "Immediate", Essential: true, UpdatedAt: now, UpdatedBy: "system"},
		{ID: "rule_quote_accepted", Trigger: "quote.accepted", Label: "Quotation accepted", Category: "Quotations", TemplateKey: "quote_accepted", Enabled: true, Timing: "Immediate", Essential: true, UpdatedAt: now, UpdatedBy: "system"},
		{ID: "rule_order_amended", Trigger: "order.amended", Label: "Order amended", Category: "Orders", TemplateKey: "order_amended", Enabled: true, Timing: "Immediate", Essential: true, UpdatedAt: now, UpdatedBy: "system"},
		{ID: "rule_order_ready", Trigger: "order.ready", Label: "Order ready", Category: "Orders", TemplateKey: "order_ready", Enabled: true, Timing: "Immediate", Essential: true, UpdatedAt: now, UpdatedBy: "system"},
		{ID: "rule_payment_due", Trigger: "payment.due", Label: "Payment reminder", Category: "Payments", TemplateKey: "payment_due", Enabled: true, Timing: "Delayed", DelayMinutes: 1440, MaxReminders: 3, UpdatedAt: now, UpdatedBy: "system"},
		{ID: "rule_payment_confirmed", Trigger: "payment.confirmed", Label: "Payment confirmed", Category: "Payments", TemplateKey: "payment_confirmed", Enabled: true, Timing: "Immediate", Essential: true, UpdatedAt: now, UpdatedBy: "system"},
		{ID: "rule_shipment_dispatched", Trigger: "shipment.dispatched", Label: "Shipment dispatched", Category: "Shipping", TemplateKey: "shipment_dispatched", Enabled: true, Timing: "Immediate", Essential: true, UpdatedAt: now, UpdatedBy: "system"},
		{ID: "rule_order_cancelled", Trigger: "order.cancelled", Label: "Order cancelled", Category: "Orders", TemplateKey: "order_cancelled", Enabled: true, Timing: "Immediate", Essential: true, UpdatedAt: now, UpdatedBy: "system"},
	}
}

func defaultTemplates(now string) []Template {
	definitions := []struct {
		key, name, category, language, subject, body string
	}{
		{"quote_priced", "Quotation ready", "Quotations", "en", "Your quotation {{quote_id}} is ready", "Hi {{customer_name}},\n\nYour formal quotation is ready to review in the portal.\n\nProduct: {{quantity}} × {{product_name}}\nUnit price: {{unit_price}}\nTotal: {{total_amount}}\nExpected ready: {{expected_date}}\n\nOpen {{portal_url}}/quotations\n\nChakuChuri"},
		{"quote_priced", "Quotation ready", "Quotations", "ur", "Aap ki quotation {{quote_id}} tayyar hai", "Assalam-o-Alaikum {{customer_name}},\n\nAap ki formal quotation portal par review ke liye tayyar hai.\n\nProduct: {{quantity}} × {{product_name}}\nFi unit: {{unit_price}}\nKul raqam: {{total_amount}}\nMutawaqqa ready: {{expected_date}}\n\nOpen {{portal_url}}/quotations\n\nChakuChuri"},
		{"quote_accepted", "Quotation accepted", "Quotations", "en", "Order {{order_id}} is confirmed", "Hi {{customer_name}},\n\nWe have confirmed your manufacturing order and production planning can begin.\n\nOrder: {{order_id}}\nProduct: {{quantity}} × {{product_name}}\nTotal: {{total_amount}}\nExpected ready: {{expected_date}}\n\nOpen {{portal_url}}/orders\n\nChakuChuri"},
		{"quote_accepted", "Quotation accepted", "Quotations", "ur", "Order {{order_id}} confirm ho gaya", "Assalam-o-Alaikum {{customer_name}},\n\nAap ka manufacturing order confirm ho gaya hai aur planning shuru ho sakti hai.\n\nOrder: {{order_id}}\nProduct: {{quantity}} × {{product_name}}\nKul raqam: {{total_amount}}\nMutawaqqa ready: {{expected_date}}\n\nOpen {{portal_url}}/orders\n\nChakuChuri"},
		{"order_amended", "Order amended", "Orders", "en", "Order {{order_id}} was updated", "Hi {{customer_name}},\n\nYour order details were updated after verification.\n\nOrder: {{order_id}}\nProduct: {{quantity}} × {{product_name}}\nTotal: {{total_amount}}\nExpected ready: {{expected_date}}\nReason: {{change_reason}}\n\nOpen {{portal_url}}/orders\n\nChakuChuri"},
		{"order_amended", "Order amended", "Orders", "ur", "Order {{order_id}} update ho gaya", "Assalam-o-Alaikum {{customer_name}},\n\nAap ke order details tasdeeq ke baad update kiye gaye hain.\n\nOrder: {{order_id}}\nProduct: {{quantity}} × {{product_name}}\nKul raqam: {{total_amount}}\nMutawaqqa ready: {{expected_date}}\nWajah: {{change_reason}}\n\nOpen {{portal_url}}/orders\n\nChakuChuri"},
		{"order_ready", "Order ready", "Orders", "en", "Order {{order_id}} is ready for delivery", "Hi {{customer_name}},\n\nGood news — your order is ready for delivery.\n\nOrder: {{order_id}}\nProduct: {{product_name}}\nBalance due: {{balance_due}}\n\nOpen {{portal_url}}/orders\n\nChakuChuri"},
		{"order_ready", "Order ready", "Orders", "ur", "Order {{order_id}} delivery ke liye tayyar hai", "Assalam-o-Alaikum {{customer_name}},\n\nAchhi khabar — aap ka order delivery ke liye tayyar hai.\n\nOrder: {{order_id}}\nProduct: {{product_name}}\nBaqaya: {{balance_due}}\n\nOpen {{portal_url}}/orders\n\nChakuChuri"},
		{"payment_due", "Payment reminder", "Payments", "en", "Balance due on order {{order_id}}", "Hi {{customer_name}},\n\nA payment is still outstanding on your account.\n\nOrder: {{order_id}}\nBalance due: {{balance_due}}\n\nPay {{portal_url}}/payments\n\nChakuChuri"},
		{"payment_due", "Payment reminder", "Payments", "ur", "Order {{order_id}} ki payment baqi hai", "Assalam-o-Alaikum {{customer_name}},\n\nAap ke account par payment abhi baqi hai.\n\nOrder: {{order_id}}\nBaqaya: {{balance_due}}\n\nPay {{portal_url}}/payments\n\nChakuChuri"},
		{"payment_confirmed", "Payment confirmed", "Payments", "en", "Payment {{payment_id}} confirmed", "Hi {{customer_name}},\n\nThank you — we have confirmed your payment.\n\nPayment: {{payment_id}}\nType: {{payment_type}}\nAmount: {{payment_amount}}\nStatus: {{payment_status}}\n\nOpen {{portal_url}}/payments\n\nChakuChuri"},
		{"payment_confirmed", "Payment confirmed", "Payments", "ur", "Payment {{payment_id}} confirm ho gayi", "Assalam-o-Alaikum {{customer_name}},\n\nShukriya — aap ki payment confirm ho gayi hai.\n\nPayment: {{payment_id}}\nType: {{payment_type}}\nRaqam: {{payment_amount}}\nStatus: {{payment_status}}\n\nOpen {{portal_url}}/payments\n\nChakuChuri"},
		{"shipment_dispatched", "Shipment dispatched", "Shipping", "en", "Shipment {{shipment_id}} is on the way", "Hi {{customer_name}},\n\nYour shipment has been dispatched.\n\nShipment: {{shipment_id}}\nCourier: {{courier}}\nTracking: {{tracking_number}}\nStatus: {{shipment_status}}\n\nTrack {{portal_url}}/shipping\n\nChakuChuri"},
		{"shipment_dispatched", "Shipment dispatched", "Shipping", "ur", "Shipment {{shipment_id}} rawana ho gayi", "Assalam-o-Alaikum {{customer_name}},\n\nAap ki shipment dispatch ho gayi hai.\n\nShipment: {{shipment_id}}\nCourier: {{courier}}\nTracking: {{tracking_number}}\nStatus: {{shipment_status}}\n\nTrack {{portal_url}}/shipping\n\nChakuChuri"},
		{"order_cancelled", "Order cancelled", "Orders", "en", "Order {{order_id}} was cancelled", "Hi {{customer_name}},\n\nYour order was cancelled after verification and the ledger was updated.\n\nOrder: {{order_id}}\nReason: {{cancellation_reason}}\nReversed charge: {{total_amount}}\nAccount credit: {{customer_credit}}\n\nOpen {{portal_url}}/payments\n\nChakuChuri"},
		{"order_cancelled", "Order cancelled", "Orders", "ur", "Order {{order_id}} cancel ho gaya", "Assalam-o-Alaikum {{customer_name}},\n\nAap ka order tasdeeq ke baad cancel ho gaya hai aur ledger update ho gaya hai.\n\nOrder: {{order_id}}\nWajah: {{cancellation_reason}}\nWapas charge: {{total_amount}}\nAccount credit: {{customer_credit}}\n\nOpen {{portal_url}}/payments\n\nChakuChuri"},
	}
	templates := make([]Template, 0, len(definitions))
	for _, row := range definitions {
		content := row.subject + "\n" + row.body
		templates = append(templates, Template{
			ID: row.key + "." + row.language, Key: row.key, Name: row.name, Category: row.category,
			Language: row.language, Subject: row.subject, Body: row.body, Variables: variablesIn(content),
			Enabled: true, Version: 4, UpdatedAt: now, UpdatedBy: "system",
		})
	}
	return templates
}

var allowedVariables = map[string]bool{
	"customer_name": true, "company_name": true, "portal_url": true, "quote_id": true,
	"order_id": true, "product_name": true, "quantity": true, "unit_price": true,
	"total_amount": true, "expected_date": true, "current_stage": true, "balance_due": true,
	"paid_amount": true, "change_reason": true, "cancellation_reason": true, "customer_credit": true,
	"payment_id": true, "payment_type": true, "payment_amount": true, "payment_status": true,
	"shipment_id": true, "courier": true, "tracking_number": true, "shipment_status": true,
	"support_email": true, "support_phone": true,
}

func sampleContext() map[string]string {
	return map[string]string{
		"customer_name": "Muhammad Zain", "company_name": "ABC Export House", "portal_url": "http://localhost:5170",
		"quote_id": "Q-260903-1114", "order_id": "MFG-260903-1115", "product_name": "Damascus chef knife",
		"quantity": "50", "unit_price": "Rs 8,500", "total_amount": "Rs 425,000", "expected_date": "18 September 2026",
		"current_stage": "Quality check", "balance_due": "Rs 125,000", "paid_amount": "Rs 300,000",
		"change_reason": "Updated after customer confirmation", "cancellation_reason": "Cancelled by mutual agreement",
		"customer_credit": "Rs 50,000", "payment_id": "PAY-260903-1116", "payment_type": "Bank transfer",
		"payment_amount": "Rs 300,000", "payment_status": "Confirmed", "shipment_id": "SHIP-260903-1117",
		"courier": "DHL", "tracking_number": "JD0146000123456789", "shipment_status": "In transit",
		"support_email": "support@chakuchuri.pk", "support_phone": "+92 300 0000000",
	}
}

func (s *Service) liveContextLocked(recipient Recipient) map[string]string {
	return map[string]string{
		"customer_name": firstNonEmpty(recipient.Name, recipient.CompanyName, "Customer"),
		"company_name":  firstNonEmpty(recipient.CompanyName, recipient.Name, "Customer"),
		"portal_url":    firstNonEmpty(s.portalURL, "https://chakuchuri.pk"),
		"support_email": firstNonEmpty(s.supportEmail, s.settings.ReplyTo, "support@chakuchuri.pk"),
		"support_phone": s.supportPhone,
	}
}

func (s *Service) previewContextLocked(recipient Recipient) map[string]string {
	values := sampleContext()
	values["customer_name"] = firstNonEmpty(recipient.Name, recipient.CompanyName, values["customer_name"])
	values["company_name"] = firstNonEmpty(recipient.CompanyName, recipient.Name, values["company_name"])
	values["portal_url"] = firstNonEmpty(s.portalURL, values["portal_url"])
	values["support_email"] = firstNonEmpty(s.supportEmail, s.settings.ReplyTo, values["support_email"])
	if s.supportPhone != "" {
		values["support_phone"] = s.supportPhone
	}
	return values
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || seen[part] {
			continue
		}
		seen[part] = true
		out = append(out, part)
	}
	return out
}

func render(source string, values map[string]string) (string, []string) {
	missing := []string{}
	result := variablePattern.ReplaceAllStringFunc(source, func(match string) string {
		parts := variablePattern.FindStringSubmatch(match)
		key := parts[1]
		value := strings.TrimSpace(values[key])
		if value == "" {
			missing = append(missing, key)
			return match
		}
		return value
	})
	return result, uniqueSorted(missing)
}

func renderLoose(source string, values map[string]string) string {
	result, _ := render(source, values)
	return result
}

func variablesIn(source string) []string {
	values := []string{}
	for _, match := range variablePattern.FindAllStringSubmatch(source, -1) {
		values = append(values, match[1])
	}
	return uniqueSorted(values)
}

func invalidVariables(source string) []string {
	invalid := []string{}
	for _, variable := range variablesIn(source) {
		if !allowedVariables[variable] {
			invalid = append(invalid, variable)
		}
	}
	return invalid
}

func normalizeTiming(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "immediate":
		return "Immediate"
	case "delayed":
		return "Delayed"
	case "manual approval", "manual":
		return "Manual approval"
	default:
		return ""
	}
}

func normalizeLanguage(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), "ur") {
		return "ur"
	}
	return "en"
}

func looksLikeEmail(value string) bool {
	value = strings.TrimSpace(value)
	parts := strings.Split(value, "@")
	return len(parts) == 2 && parts[0] != "" && strings.Contains(parts[1], ".") && !strings.ContainsAny(value, " \t\r\n")
}

func uniqueSorted(values []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func copyMap(values map[string]string) map[string]string {
	copyValues := make(map[string]string, len(values))
	for key, value := range values {
		copyValues[key] = value
	}
	return copyValues
}

func copyOutbox(item OutboxItem) OutboxItem {
	item.Payload = copyMap(item.Payload)
	return item
}

func safeProviderError(err error) string {
	if err == nil {
		return ""
	}
	message := strings.TrimSpace(err.Error())
	if message == "" {
		return "Delivery failed."
	}
	if len(message) > 240 {
		message = message[:240]
	}
	return message
}

func actorName(actor auth.User) string {
	return firstNonEmpty(actor.Name, actor.Email, "system")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (s *Service) signalWorker() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *Service) record(actor auth.User, action string, entity string, detail string) {
	if s.recorder != nil {
		s.recorder.Record(actorName(actor), action, entity, detail)
	}
}

func (s *Service) recordSystem(action string, entity string, detail string) {
	if s.recorder != nil {
		s.recorder.Record("system", action, entity, detail)
	}
}
