package workflow

import (
	"errors"
	"testing"

	"chakuchuri/backend/internal/auth"
)

// These probes run only through review-overlay.json, using synthetic data.
func TestReviewSelectedNoticeIsolation(t *testing.T) {
	notice := CustomerNotice{
		ID: "review-notice", Title: "Private update", Body: "For customer A",
		Active: true, Audience: "selected", CustomerIDs: []string{"review-a"},
	}
	customer := auth.User{ID: "review-user-b", Role: "Customer", Status: "Active", CustomerID: "review-b"}
	if got := filterNoticesForUser([]CustomerNotice{notice}, customer); len(got) != 0 {
		t.Fatal("snapshot exposed selected notice to customer B")
	}
	change := workspaceUpsert("notices", "", notice)
	for _, got := range filterWorkspaceChanges(customer, []WorkspaceChange{change}) {
		if got.Operation != "remove" {
			t.Fatal("WebSocket exposes customer A's selected notice to customer B despite snapshot filtering")
		}
	}
}

func TestReviewInactiveNoticeIsolation(t *testing.T) {
	customer := auth.User{ID: "review-user", Role: "Customer", Status: "Active", CustomerID: "review-a"}
	change := workspaceUpsert("notices", "", CustomerNotice{ID: "review-draft", Active: false, Audience: "all"})
	for _, got := range filterWorkspaceChanges(customer, []WorkspaceChange{change}) {
		if got.Operation == "upsert" {
			t.Fatal("WebSocket sends inactive notice content to a customer")
		}
	}
}

type reviewFailRepository struct{ saves int }

func (r *reviewFailRepository) LoadState() (State, string, error) {
	return State{}, "", errors.New("review: no stored state")
}

func (r *reviewFailRepository) SaveState(State) error {
	r.saves++
	return errors.New("review: simulated storage failure")
}

func TestReviewNoticePersistenceFailure(t *testing.T) {
	repository := &reviewFailRepository{}
	hub := newWorkspaceEventHub()
	observer := &workspaceClient{
		user: auth.User{Role: "Owner", Status: "Active"},
		send: make(chan workspaceSocketEvent, 4), done: make(chan struct{}),
	}
	hub.join(observer)
	service := &Service{repository: repository, workspaceEvents: hub}
	_, err := service.CreateCustomerNotice(CustomerNoticeRequest{
		Title: "Synthetic review notice", Body: "Not saved to any live storage",
	}, auth.User{ID: "review-owner", Role: "Owner", Status: "Active"})
	if repository.saves != 1 {
		t.Fatalf("expected one storage attempt, got %d", repository.saves)
	}
	if err == nil {
		t.Error("create reports success after SaveState failed")
	}
	if len(service.notices) != 0 {
		t.Error("failed save leaves an uncommitted notice in memory")
	}
	select {
	case <-observer.send:
		t.Error("failed save already published a customer-visible event")
	default:
	}
}

func TestReviewUnrelatedEventFanout(t *testing.T) {
	hub := newWorkspaceEventHub()
	observer := &workspaceClient{
		user: auth.User{Role: "Customer", Status: "Active", CustomerID: "review-b"},
		send: make(chan workspaceSocketEvent, 4), done: make(chan struct{}),
	}
	hub.join(observer)
	hub.publish([]WorkspaceChange{workspaceUpsert("quotations", "review-a", map[string]string{"id": "review-quote"})})
	select {
	case event := <-observer.send:
		t.Fatalf("unrelated customer received a frame with %d visible changes; fanout is global", len(event.Changes))
	default:
	}
}
