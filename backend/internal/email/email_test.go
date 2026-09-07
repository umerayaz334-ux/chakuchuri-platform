package email

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"chakuchuri/backend/internal/auth"
)

func TestDefaultTemplatesPreviewInEnglishAndUrdu(t *testing.T) {
	service := NewService(nil, nil)
	for _, language := range []string{"en", "ur"} {
		preview, err := service.Preview(PreviewRequest{TemplateKey: "order_ready", Language: language})
		if err != nil {
			t.Fatalf("preview %s template: %v", language, err)
		}
		if preview.Subject == "" || preview.Body == "" || len(preview.MissingVariables) != 0 {
			t.Fatalf("incomplete %s preview: %#v", language, preview)
		}
		if !strings.Contains(preview.HtmlBody, "<!DOCTYPE html>") || !strings.Contains(preview.HtmlBody, "linear-gradient") {
			t.Fatalf("html preview missing rich layout for %s", language)
		}
	}
}

func TestModernEmailLayoutBuildsFactRowsAndCTA(t *testing.T) {
	htmlBody := buildEmailHTML("ChakuChuri", "Order ready", "Hi Zain,\n\nOrder: MFG-1\nBalance due: Rs 10,000\n\nOpen http://localhost:5170/orders\n\nChakuChuri", "order_ready", sampleContext())
	if !strings.Contains(htmlBody, "Balance due") || !strings.Contains(htmlBody, "http://localhost:5170/orders") {
		t.Fatalf("layout missing facts/cta: %s", htmlBody)
	}
	if !strings.Contains(htmlBody, "READY") || !strings.Contains(htmlBody, "📦") || !strings.Contains(htmlBody, "border-radius:16px") {
		t.Fatalf("layout missing rich purpose styling")
	}
}

func TestPaymentDueUsesAttentionTheme(t *testing.T) {
	htmlBody := buildEmailHTML("ChakuChuri", "Balance due", "Hi Zain,\n\nOrder: MFG-1\nBalance due: Rs 10,000\n\nPay http://localhost:5170/payments\n\nChakuChuri", "payment_due", sampleContext())
	if !strings.Contains(htmlBody, "#ea580c") || !strings.Contains(htmlBody, "DUE NOW") || !strings.Contains(htmlBody, "Pay balance") {
		t.Fatalf("payment due theme missing: %s", htmlBody)
	}
}

func TestBusinessEventIsIdempotent(t *testing.T) {
	service := testService()
	event := BusinessEvent{
		EventKey: "quote:Q-1:priced:1", Trigger: "quote.priced",
		CustomerID: "cust_1", EntityID: "Q-1",
		Data: map[string]string{
			"quote_id": "Q-1", "product_name": "Chef knife", "quantity": "20",
			"unit_price": "PKR 8000", "total_amount": "PKR 160000", "expected_date": "2026-09-20",
		},
	}
	if _, err := service.Publish(event); err != nil {
		t.Fatalf("publish first event: %v", err)
	}
	if _, err := service.Publish(event); !errors.Is(err, errDuplicate) {
		t.Fatalf("expected duplicate event error, got %v", err)
	}
	if got := len(service.Snapshot().Outbox); got != 1 {
		t.Fatalf("duplicate event created %d outbox items", got)
	}
}

func TestResolvedPaymentCancelsScheduledReminder(t *testing.T) {
	service := testService()
	if _, err := service.Publish(BusinessEvent{
		EventKey: "order:MFG-1:payment-due:1", Trigger: "payment.due",
		CustomerID: "cust_1", EntityID: "MFG-1",
		Data: map[string]string{"order_id": "MFG-1", "balance_due": "PKR 50000"},
	}); err != nil {
		t.Fatalf("queue payment reminder: %v", err)
	}
	if _, err := service.Publish(BusinessEvent{
		EventKey: "payment:PAY-1:confirmed", Trigger: "payment.confirmed",
		CustomerID: "cust_1", EntityID: "PAY-1",
		Data: map[string]string{
			"order_id": "MFG-1", "payment_id": "PAY-1", "payment_type": "Bank transfer",
			"payment_amount": "PKR 50000", "payment_status": "Confirmed",
			"cancel_reminder_orders": "MFG-1",
		},
	}); err != nil {
		t.Fatalf("queue payment confirmation: %v", err)
	}
	snapshot := service.Snapshot()
	if snapshot.Outbox[1].Status != StatusCancelled {
		t.Fatalf("payment reminder was not cancelled: %#v", snapshot.Outbox)
	}
	service.ProcessDue(context.Background(), 10)
	snapshot = service.Snapshot()
	if snapshot.Deliveries[0].Status != StatusCaptured {
		t.Fatalf("payment confirmation was not captured: %#v", snapshot.Deliveries[0])
	}
}

func TestLivePublishDoesNotSeedSamplePayload(t *testing.T) {
	service := testService()
	service.SetDeliveryDefaults("https://portal.example", "help@example.com", "")
	item, err := service.Publish(BusinessEvent{
		EventKey: "order:MFG-9:ready", Trigger: "order.ready",
		CustomerID: "cust_1", EntityID: "MFG-9",
		Data: map[string]string{"order_id": "MFG-9", "product_name": "Knife", "balance_due": "PKR 1"},
	})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if item.Payload["portal_url"] != "https://portal.example" {
		t.Fatalf("portal url not applied: %#v", item.Payload)
	}
	if item.Payload["quote_id"] != "" || item.Payload["courier"] != "" || strings.Contains(item.Payload["portal_url"], "localhost") {
		t.Fatalf("sample fields leaked into live payload: %#v", item.Payload)
	}
}

func TestStaleSendingIsReclaimed(t *testing.T) {
	service := testService()
	item, err := service.Publish(BusinessEvent{
		EventKey: "order:MFG-2:ready", Trigger: "order.ready",
		CustomerID: "cust_1", EntityID: "MFG-2",
		Data: map[string]string{"order_id": "MFG-2", "product_name": "Knife", "balance_due": "PKR 1"},
	})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	service.mu.Lock()
	for index := range service.outbox {
		if service.outbox[index].ID == item.ID {
			service.outbox[index].Status = StatusSending
			service.outbox[index].UpdatedAt = time.Now().UTC().Add(-10 * time.Minute).Format(time.RFC3339)
			service.updateDeliveryLocked(service.outbox[index].DeliveryID, func(delivery *Delivery) {
				delivery.Status = StatusSending
			})
		}
	}
	service.mu.Unlock()
	service.ProcessDue(context.Background(), 5)
	snapshot := service.Snapshot()
	found := false
	for _, delivery := range snapshot.Deliveries {
		if delivery.OutboxID == item.ID {
			found = true
			if delivery.Status != StatusCaptured {
				t.Fatalf("expected reclaimed send to capture, got %#v", delivery)
			}
		}
	}
	if !found {
		t.Fatal("delivery missing after reclaim")
	}
}

func TestSMTPSettingsRoundTripHidesPassword(t *testing.T) {
	service := NewService(nil, nil)
	actor := auth.User{ID: "owner_1", Name: "Owner", Email: "owner@example.com", Role: "Owner"}
	snapshot, err := service.UpdateSettings(SettingsUpdate{
		FromName: "ChakuChuri", FromEmail: "notifications@chakuchuri.pk", ReplyTo: "support@chakuchuri.pk",
		Provider: "smtp", SMTPHost: "smtp.example.com", SMTPPort: 587, SMTPUsername: "mailer@example.com",
		SMTPPassword: "super-secret",
	}, actor)
	if err != nil {
		t.Fatalf("update settings: %v", err)
	}
	if snapshot.Settings.Provider != "smtp" || snapshot.Settings.SMTPHost != "smtp.example.com" {
		t.Fatalf("smtp settings missing: %#v", snapshot.Settings)
	}
	if snapshot.Settings.SMTPPassword != "" {
		t.Fatal("password leaked in snapshot")
	}
	if !snapshot.Settings.PasswordConfigured {
		t.Fatal("expected passwordConfigured flag")
	}
	if _, ok := service.mailer.(SMTPMailer); !ok {
		t.Fatalf("expected SMTP mailer, got %T", service.mailer)
	}

	kept, err := service.UpdateSettings(SettingsUpdate{
		FromName: "ChakuChuri", FromEmail: "notifications@chakuchuri.pk", ReplyTo: "support@chakuchuri.pk",
		Provider: "smtp", SMTPHost: "smtp.example.com", SMTPPort: 587, SMTPUsername: "mailer@example.com",
	}, actor)
	if err != nil {
		t.Fatalf("keep password: %v", err)
	}
	if !kept.Settings.PasswordConfigured {
		t.Fatal("password should remain configured when blank update")
	}
	service.mu.Lock()
	password := service.settings.SMTPPassword
	service.mu.Unlock()
	if password != "super-secret" {
		t.Fatalf("password was cleared unexpectedly: %q", password)
	}
}

func TestSMTPMailerRejectsEmptyHost(t *testing.T) {
	mailer := SMTPMailer{Host: "", Port: 587}
	if err := mailer.Verify(context.Background()); err == nil {
		t.Fatal("expected verify failure for empty host")
	}
}

func TestPreviewUsesDraftSubjectAndBody(t *testing.T) {
	service := NewService(nil, nil)
	preview, err := service.Preview(PreviewRequest{
		TemplateKey: "order_ready", Language: "en",
		Subject: "Draft subject {{order_id}}",
		Body:    "Draft body for {{customer_name}}",
	})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if !strings.Contains(preview.Subject, "Draft subject") || !strings.Contains(preview.Body, "Draft body") {
		t.Fatalf("draft override ignored: %#v", preview)
	}
}

func TestSendTestCapturesDeliveryAndPersistsTemplateVersion(t *testing.T) {
	dataDir := t.TempDir()
	service := NewService(nil, nil)
	service.EnablePersistence(dataDir)
	actor := auth.User{ID: "owner_1", Name: "Owner", Email: "owner@example.com", Role: "Owner"}

	snapshot, err := service.SendTest(context.Background(), TestRequest{
		Recipient: "owner@example.com", TemplateKey: "quote_priced", Language: "en",
	}, actor)
	if err != nil {
		t.Fatalf("send test: %v", err)
	}
	if len(snapshot.Deliveries) != 1 || snapshot.Deliveries[0].Status != StatusCaptured {
		t.Fatalf("test delivery was not captured: %#v", snapshot.Deliveries)
	}

	template := findTemplate(snapshot.Templates, "quote_priced.en")
	updated, err := service.UpdateTemplate(template.ID, TemplateUpdate{
		Subject: template.Subject + " - updated", Body: template.Body, Enabled: true,
	}, actor)
	if err != nil {
		t.Fatalf("update template: %v", err)
	}
	if findTemplate(updated.Templates, template.ID).Version != template.Version+1 {
		t.Fatal("template version did not increment")
	}

	reloaded := NewService(nil, nil)
	reloaded.EnablePersistence(dataDir)
	if findTemplate(reloaded.Snapshot().Templates, template.ID).Version != template.Version+1 {
		t.Fatal("template version did not survive snapshot reload")
	}
}

func testService() *Service {
	service := NewService(nil, nil)
	service.SetRecipientResolver(func(customerID string) (Recipient, bool) {
		return Recipient{
			CustomerID: customerID, Name: "Muhammad Zain", CompanyName: "ABC Export House",
			Email: "buyer@example.com", Language: "en",
		}, true
	})
	return service
}

func findTemplate(templates []Template, id string) Template {
	for _, template := range templates {
		if template.ID == id {
			return template
		}
	}
	return Template{}
}
