package workflow

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"chakuchuri/backend/internal/auth"
)

type appHomeTestRepository struct {
	fail       bool
	saves      int
	state      State
	beforeSave func()
}

func onlyRemovalID(data json.RawMessage, id string) bool {
	var fields map[string]string
	return json.Unmarshal(data, &fields) == nil && len(fields) == 1 && fields["id"] == id
}

func (r *appHomeTestRepository) LoadState() (State, string, error) {
	return r.state, "test", nil
}
func (r *appHomeTestRepository) SaveState(state State) error {
	r.saves++
	if r.beforeSave != nil {
		r.beforeSave()
	}
	if r.fail {
		return errors.New("synthetic storage failure")
	}
	r.state = state
	return nil
}

func TestAppHomeSaveFailuresRollbackBeforePublish(t *testing.T) {
	owner := auth.User{Role: "Owner", Status: "Active"}
	for _, operation := range []string{"notice-create", "notice-update", "notice-delete", "featured-create", "featured-update", "featured-delete"} {
		t.Run(operation, func(t *testing.T) {
			repo := &appHomeTestRepository{fail: true}
			hub := newWorkspaceEventHub()
			client := &workspaceClient{user: owner, send: make(chan workspaceSocketEvent, 8), done: make(chan struct{})}
			hub.join(client)
			<-client.send // Initial subscription acknowledgement.
			notices := []CustomerNotice{{ID: "n1", Title: "Original", Body: "Body", Active: true, Audience: "all"}, {ID: "n2"}}
			featured := []FeaturedProduct{{ID: "f1", Name: "Original", ImageFileID: "image", Active: true}, {ID: "f2"}}
			s := &Service{repository: repo, workspaceEvents: hub, notices: append([]CustomerNotice(nil), notices...), featured: append([]FeaturedProduct(nil), featured...)}
			repo.beforeSave = func() {
				if len(client.send) != 0 {
					t.Error("event published before persistence")
				}
			}
			run := func() error {
				switch operation {
				case "notice-create":
					_, err := s.CreateCustomerNotice(CustomerNoticeRequest{Title: "New", Body: "Body"}, owner)
					return err
				case "notice-update":
					_, err := s.UpdateCustomerNotice("n1", CustomerNoticeRequest{Title: "Changed"}, owner)
					return err
				case "notice-delete":
					_, err := s.DeleteCustomerNotice("n1", owner)
					return err
				case "featured-create":
					_, err := s.CreateFeaturedProduct(FeaturedProductRequest{Name: "New", ImageFileID: "image"}, owner)
					return err
				case "featured-update":
					_, err := s.UpdateFeaturedProduct("f1", FeaturedProductRequest{Name: "Changed"}, owner)
					return err
				default:
					_, err := s.DeleteFeaturedProduct("f1", owner)
					return err
				}
			}
			if err := run(); !errors.Is(err, errPersistence) {
				t.Fatalf("expected storage error, got %v", err)
			}
			if !reflect.DeepEqual(s.notices, notices) || !reflect.DeepEqual(s.featured, featured) {
				t.Fatal("failed mutation changed state")
			}
			if len(client.send) != 0 {
				t.Fatal("failed mutation published event")
			}
			repo.fail = false
			if err := run(); err != nil {
				t.Fatal(err)
			}
			if repo.saves != 2 || len(client.send) != 1 {
				t.Fatal("successful retry must save and publish once")
			}
			if !reflect.DeepEqual(s.notices, repo.state.Notices) || !reflect.DeepEqual(s.featured, repo.state.Featured) {
				t.Fatal("memory does not match persisted data")
			}
		})
	}
}

func TestNoticeAudienceAndWithdrawal(t *testing.T) {
	notice := CustomerNotice{ID: "n1", Title: "Private", Body: "Sensitive", Audience: "selected", CustomerIDs: []string{"a", "c"}, Active: true}
	for _, customerID := range []string{"a", "b"} {
		user := auth.User{Role: "Customer", Status: "Active", CustomerID: customerID}
		snapshot := filterNoticesForUser([]CustomerNotice{notice}, user)
		events := filterWorkspaceChanges(user, []WorkspaceChange{workspaceUpsert("notices", "", notice)})
		if len(events) != 1 {
			t.Fatal("expected upsert or withdrawal")
		}
		if customerID == "a" {
			if len(snapshot) != 1 || len(snapshot[0].CustomerIDs) != 0 || events[0].Operation != "upsert" {
				t.Fatal("intended recipient missing notice or received audience IDs")
			}
			var received CustomerNotice
			if err := json.Unmarshal(events[0].Data, &received); err != nil {
				t.Fatal(err)
			}
			if len(received.CustomerIDs) != 0 || received.Body != notice.Body {
				t.Fatal("incorrect recipient payload")
			}
		} else {
			if len(snapshot) != 0 || events[0].Operation != "remove" || !onlyRemovalID(events[0].Data, "n1") {
				t.Fatal("unauthorized notice content exposed")
			}
		}
	}
	notice.Active = false
	customer := auth.User{Role: "Customer", Status: "Active", CustomerID: "a"}
	events := filterWorkspaceChanges(customer, []WorkspaceChange{workspaceUpsert("notices", "", notice)})
	if len(events) != 1 || events[0].Operation != "remove" {
		t.Fatal("inactive notice must be withdrawn")
	}
	owner := auth.User{Role: "Owner", Status: "Active"}
	if got := filterWorkspaceChanges(owner, []WorkspaceChange{workspaceUpsert("notices", "", notice)}); len(got) != 1 || got[0].Operation != "upsert" {
		t.Fatal("owner cannot manage inactive notice")
	}
	if len(notice.CustomerIDs) != 2 {
		t.Fatal("filter modified stored audience")
	}
}

func TestFeaturedWithdrawalAndStorageHTTPError(t *testing.T) {
	customer := auth.User{Role: "Customer", Status: "Active", CustomerID: "a"}
	events := filterWorkspaceChanges(customer, []WorkspaceChange{workspaceUpsert("featured", "", FeaturedProduct{ID: "f1", Active: false})})
	if len(events) != 1 || events[0].Operation != "remove" || !onlyRemovalID(events[0].Data, "f1") {
		t.Fatal("inactive featured item leaked")
	}
	w := httptest.NewRecorder()
	writeResult(w, httptest.NewRequest(http.MethodPost, "/", nil), nil, errPersistence)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}
