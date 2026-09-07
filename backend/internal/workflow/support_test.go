package workflow

import (
	"errors"
	"testing"
	"time"

	"chakuchuri/backend/internal/auth"
)

func TestSupportMessagesCarryUnreadRolesAndAttachments(t *testing.T) {
	service := NewService(nil, nil)
	service.conversations = nil
	service.SetMessageAttachmentResolver(func(id string, actor auth.User) (MessageAttachment, bool) {
		if id != "file_customer_photo" || actor.ID != "usr_customer_support" {
			return MessageAttachment{}, false
		}
		return MessageAttachment{
			FileID: id, Name: "reference.jpg", MimeType: "image/jpeg", ByteSize: 2048,
			URL: "/api/files/" + id, ThumbnailURL: "/api/files/" + id + "/thumbnail",
		}, true
	})

	customer := auth.User{ID: "usr_customer_support", Name: "Customer Buyer", Role: "Customer", CustomerID: "cust_support"}
	admin := auth.User{ID: "usr_support_agent", Name: "Support Agent", Role: "Support Agent"}

	conversation, err := service.SendMessage(MessageRequest{
		CustomerID: "cust_support", Body: "Please check this reference.", AttachmentIDs: []string{"file_customer_photo"},
	}, customer)
	if err != nil {
		t.Fatalf("send customer message: %v", err)
	}
	if conversation.UnreadForAdmin != 1 || conversation.UnreadForCustomer != 0 {
		t.Fatalf("unexpected unread counters after customer message: %#v", conversation)
	}
	message := conversation.Messages[0]
	if message.AuthorRole != "Customer" || len(message.Attachments) != 1 || message.Attachments[0].Name != "reference.jpg" {
		t.Fatalf("expected customer message with attachment, got %#v", message)
	}

	conversation, err = service.SendMessage(MessageRequest{CustomerID: "cust_support", Body: "We are checking it now."}, admin)
	if err != nil {
		t.Fatalf("send admin message: %v", err)
	}
	if conversation.UnreadForAdmin != 0 || conversation.UnreadForCustomer != 1 {
		t.Fatalf("unexpected unread counters after admin reply: %#v", conversation)
	}
	conversation, err = service.MarkConversationRead(conversation.ID, customer)
	if err != nil || conversation.UnreadForCustomer != 0 {
		t.Fatalf("customer should clear their unread counter: %#v, %v", conversation, err)
	}
	if got := conversation.Messages[len(conversation.Messages)-1].AuthorRole; got != "Support Agent" {
		t.Fatalf("expected support author role, got %q", got)
	}

	_, err = service.SendMessage(MessageRequest{CustomerID: "cust_support", AttachmentIDs: []string{"file_customer_photo"}}, admin)
	if !errors.Is(err, errForbidden) {
		t.Fatalf("expected attachment ownership check, got %v", err)
	}
}

func TestSupportMessageAcceptsFiveDistinctAttachments(t *testing.T) {
	service := NewService(nil, nil)
	service.conversations = nil
	service.SetMessageAttachmentResolver(func(id string, actor auth.User) (MessageAttachment, bool) {
		return MessageAttachment{FileID: id, Name: id + ".csv", MimeType: "text/csv", ByteSize: 128, URL: "/api/files/" + id + "/content"}, true
	})

	customer := auth.User{ID: "usr_five_files", Name: "Five Files Buyer", Role: "Customer", CustomerID: "cust_five_files"}
	ids := []string{"file_1", "file_2", "file_3", "file_4", "file_5"}
	conversation, err := service.SendMessage(MessageRequest{CustomerID: customer.CustomerID, AttachmentIDs: ids}, customer)
	if err != nil {
		t.Fatalf("send five attachments: %v", err)
	}
	message := conversation.Messages[len(conversation.Messages)-1]
	if len(message.Attachments) != 5 {
		t.Fatalf("expected five attachments on one message, got %d", len(message.Attachments))
	}
	for index, attachment := range message.Attachments {
		if attachment.FileID != ids[index] {
			t.Fatalf("attachment order mismatch at %d: got %s want %s", index, attachment.FileID, ids[index])
		}
	}
}

func TestDirectCallLifecycle(t *testing.T) {
	service := NewService(nil, nil)
	service.calls = nil

	customer := auth.User{
		ID: "usr_customer_support", Name: "Customer Buyer", Role: "Customer", CustomerID: "cust_support",
		Permissions: []string{"calls.start"},
	}
	admin := auth.User{ID: "usr_support_agent", Name: "Support Agent", Role: "Admin"}

	call, err := service.StartCall(CallRequestPayload{CustomerID: "cust_support", Subject: "Shipping question"}, customer)
	if err != nil {
		t.Fatalf("start call: %v", err)
	}
	if call.Status != "Ringing" || call.InitiatorUserID != customer.ID || call.RingExpiresAt == "" {
		t.Fatalf("unexpected ringing call: %#v", call)
	}
	if _, err := service.UpdateCallStatus(call.ID, CallActionRequest{Status: "In call"}, customer); !errors.Is(err, errInvalidCallTransition) {
		t.Fatalf("caller must not answer their own call, got %v", err)
	}

	answered, err := service.UpdateCallStatus(call.ID, CallActionRequest{Status: "In call"}, admin)
	if err != nil {
		t.Fatalf("answer call: %v", err)
	}
	if answered.Status != "In call" || answered.RoomID == "" || answered.AnsweredByUserID != admin.ID || answered.StartedAt == "" {
		t.Fatalf("unexpected answered call: %#v", answered)
	}

	service.calls[0].StartedAt = time.Now().UTC().Add(-3 * time.Second).Format(time.RFC3339)
	completed, err := service.EndCall(call.ID, admin)
	if err != nil {
		t.Fatalf("end call: %v", err)
	}
	if completed.Status != "Completed" || completed.EndedAt == "" || completed.DurationSeconds < 2 {
		t.Fatalf("unexpected completed call: %#v", completed)
	}

	declinedCall, err := service.StartCall(CallRequestPayload{CustomerID: "cust_support", Subject: "Follow-up"}, customer)
	if err != nil {
		t.Fatalf("start second call: %v", err)
	}
	declined, err := service.UpdateCallStatus(declinedCall.ID, CallActionRequest{Status: "Declined"}, admin)
	if err != nil || declined.Status != "Declined" {
		t.Fatalf("decline call: %#v, %v", declined, err)
	}

	missedCall, err := service.StartCall(CallRequestPayload{CustomerID: "cust_support", Subject: "No answer"}, customer)
	if err != nil {
		t.Fatalf("start expiring call: %v", err)
	}
	for index := range service.calls {
		if service.calls[index].ID == missedCall.ID {
			service.calls[index].RingExpiresAt = time.Now().UTC().Add(-time.Second).Format(time.RFC3339)
		}
	}
	dashboard := service.DashboardForUser(admin)
	if got := dashboard.Metrics["ringingCalls"].(int); got != 0 {
		t.Fatalf("expected no calls still ringing, got %d", got)
	}
	if got := dashboard.Metrics["missedCalls"].(int); got != 1 {
		t.Fatalf("expected one missed call, got %d", got)
	}
}

func TestEndCallBroadcastsToCallRoom(t *testing.T) {
	service := NewService(nil, nil)
	service.calls = nil

	customer := auth.User{
		ID: "usr_customer_end", Name: "Customer Buyer", Role: "Customer", CustomerID: "cust_end",
		Permissions: []string{"calls.start"},
	}
	admin := auth.User{ID: "usr_admin_end", Name: "Support Agent", Role: "Admin"}

	call, err := service.StartCall(CallRequestPayload{CustomerID: "cust_end", Subject: "Hang up test"}, customer)
	if err != nil {
		t.Fatalf("start call: %v", err)
	}
	if _, err := service.UpdateCallStatus(call.ID, CallActionRequest{Status: "In call"}, admin); err != nil {
		t.Fatalf("answer call: %v", err)
	}

	listener := &callClient{
		callID: call.ID,
		send:   make(chan callSocketEvent, 4),
		done:   make(chan struct{}),
	}
	realtimeCallHub.join(listener)
	defer realtimeCallHub.leave(listener)

	if _, err := service.EndCall(call.ID, customer); err != nil {
		t.Fatalf("end call over HTTP path: %v", err)
	}

	select {
	case event := <-listener.send:
		if event.Type != "call-ended" || event.Call == nil || event.Call.Status != "Completed" {
			t.Fatalf("expected call-ended Completed, got %#v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("call room did not receive call-ended after HTTP EndCall")
	}
}

func TestStartCallRejectsBusyAndRequiresCustomerID(t *testing.T) {
	service := NewService(nil, nil)
	service.calls = nil

	customer := auth.User{
		ID: "usr_customer_busy", Name: "Customer Buyer", Role: "Customer", CustomerID: "cust_busy",
		Permissions: []string{"calls.start"},
	}
	admin := auth.User{ID: "usr_admin_busy", Name: "Support Agent", Role: "Admin"}

	if _, err := service.StartCall(CallRequestPayload{Subject: "Missing customer"}, admin); !errors.Is(err, errValidation) {
		t.Fatalf("expected validation without customer id, got %v", err)
	}

	first, err := service.StartCall(CallRequestPayload{CustomerID: "cust_busy", Subject: "First"}, customer)
	if err != nil {
		t.Fatalf("start first call: %v", err)
	}
	if _, err := service.StartCall(CallRequestPayload{CustomerID: "cust_busy", Subject: "Second"}, admin); !errors.Is(err, errCallBusy) {
		t.Fatalf("expected call busy while first is ringing, got %v", err)
	}
	if _, err := service.UpdateCallStatus(first.ID, CallActionRequest{Status: "Cancelled"}, customer); err != nil {
		t.Fatalf("cancel first call: %v", err)
	}
	if _, err := service.StartCall(CallRequestPayload{CustomerID: "cust_busy", Subject: "After cancel"}, admin); err != nil {
		t.Fatalf("start after cancel should work: %v", err)
	}
}

func TestExpireRingingCallsFansOutMissedStatus(t *testing.T) {
	service := NewService(nil, nil)
	service.calls = nil

	customer := auth.User{
		ID: "usr_customer_expire", Name: "Customer Buyer", Role: "Customer", CustomerID: "cust_expire",
		Permissions: []string{"calls.start"},
	}
	call, err := service.StartCall(CallRequestPayload{CustomerID: "cust_expire", Subject: "Expire me"}, customer)
	if err != nil {
		t.Fatalf("start call: %v", err)
	}
	for index := range service.calls {
		if service.calls[index].ID == call.ID {
			service.calls[index].RingExpiresAt = time.Now().UTC().Add(-time.Second).Format(time.RFC3339)
		}
	}
	if n := service.ExpireRingingCalls(); n != 1 {
		t.Fatalf("expected 1 expired call, got %d", n)
	}
	for _, row := range service.calls {
		if row.ID == call.ID && row.Status != "Missed" {
			t.Fatalf("expected missed call after expire, got %#v", row)
		}
	}
	if n := service.ExpireRingingCalls(); n != 0 {
		t.Fatalf("second expire pass should be idle, got %d", n)
	}
}

func TestCallSignalsAreScopedAndSequenced(t *testing.T) {
	service := NewService(nil, nil)
	service.calls = nil
	service.callSignals = nil

	customer := auth.User{
		ID: "usr_customer_support", Name: "Customer Buyer", Role: "Customer", CustomerID: "cust_support",
		Permissions: []string{"calls.start"},
	}
	admin := auth.User{ID: "usr_support_agent", Name: "Support Agent", Role: "Admin"}

	call, err := service.StartCall(CallRequestPayload{CustomerID: "cust_support", Subject: "In-app audio"}, customer)
	if err != nil {
		t.Fatalf("start call: %v", err)
	}
	started, err := service.UpdateCallStatus(call.ID, CallActionRequest{Status: "In call"}, admin)
	if err != nil {
		t.Fatalf("answer call: %v", err)
	}
	if started.RoomID == "" || started.CallType != "audio" {
		t.Fatalf("expected active audio room, got %#v", started)
	}

	offer, err := service.SendCallSignal(started.ID, CallSignalRequest{SignalType: "offer", Payload: map[string]interface{}{"type": "offer", "sdp": "fake"}}, admin)
	if err != nil {
		t.Fatalf("send offer: %v", err)
	}
	if offer.SignalNo != 1 {
		t.Fatalf("expected first signal number 1, got %d", offer.SignalNo)
	}
	adminSignals, err := service.ListCallSignals(started.ID, 0, admin)
	if err != nil {
		t.Fatalf("list admin signals: %v", err)
	}
	if len(adminSignals) != 0 {
		t.Fatalf("admin should not receive own offer, got %#v", adminSignals)
	}
	customerSignals, err := service.ListCallSignals(started.ID, 0, customer)
	if err != nil {
		t.Fatalf("list customer signals: %v", err)
	}
	if len(customerSignals) != 1 || customerSignals[0].SignalType != "offer" {
		t.Fatalf("customer should receive admin offer, got %#v", customerSignals)
	}

	answer, err := service.SendCallSignal(started.ID, CallSignalRequest{SignalType: "answer", Payload: map[string]interface{}{"type": "answer", "sdp": "fake"}}, customer)
	if err != nil {
		t.Fatalf("send answer: %v", err)
	}
	if answer.SignalNo != 2 {
		t.Fatalf("expected answer signal number 2, got %d", answer.SignalNo)
	}
	adminSignals, err = service.ListCallSignals(started.ID, 1, admin)
	if err != nil {
		t.Fatalf("list admin answer: %v", err)
	}
	if len(adminSignals) != 1 || adminSignals[0].SignalType != "answer" {
		t.Fatalf("admin should receive customer answer, got %#v", adminSignals)
	}
}
