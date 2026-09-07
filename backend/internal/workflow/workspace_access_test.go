package workflow

import (
	"testing"

	"chakuchuri/backend/internal/auth"
)

func TestWorkspaceAccessChanges(t *testing.T) {
	user := auth.User{ID: "u1", TenantID: "t1", CustomerID: "c1", Role: "Staff", Status: "Active", Permissions: []string{"read"}, PageAccess: []string{"Messages"}, AssignedCustomerIDs: []string{"c1"}}
	key := workspaceAccessKey(user)
	for name, mutate := range map[string]func(*auth.User){
		"account":     func(u *auth.User) { u.ID = "u2" },
		"tenant":      func(u *auth.User) { u.TenantID = "t2" },
		"customer":    func(u *auth.User) { u.CustomerID = "c2" },
		"role":        func(u *auth.User) { u.Role = "Customer" },
		"suspension":  func(u *auth.User) { u.Status = "Temporarily suspended" },
		"permissions": func(u *auth.User) { u.Permissions = nil },
		"pages":       func(u *auth.User) { u.PageAccess = nil },
		"assignments": func(u *auth.User) { u.AssignedCustomerIDs = nil },
	} {
		t.Run(name, func(t *testing.T) {
			changed := user
			mutate(&changed)
			if workspaceAccessKey(changed) == key {
				t.Fatal("access change not detected")
			}
		})
	}
	user.LastOnline = "2026-09-05T12:00:00Z"
	user.Name = "Updated name"
	if workspaceAccessKey(user) != key {
		t.Fatal("profile/presence should not disconnect")
	}
}

func TestRevokedWorkspaceEventContainsNoQueuedData(t *testing.T) {
	valid := true
	client := &workspaceClient{authorize: func() bool { return valid }}
	queued := workspaceSocketEvent{Type: "workspace.changed", Revision: 42, Changes: []WorkspaceChange{
		workspaceUpsert("quotations", "private-customer", map[string]string{"id": "private-quote"}),
	}}
	if event, allowed := client.authorizedEvent(queued); !allowed || event.Changes[0].CustomerID != "private-customer" {
		t.Fatal("valid session lost its event")
	}
	valid = false
	for _, candidate := range []workspaceSocketEvent{queued, {}} {
		event, allowed := client.authorizedEvent(candidate)
		if allowed || len(event.Changes) != 1 || event.Changes[0].Scope != "users" ||
			event.Changes[0].Operation != "invalidate" || event.Changes[0].Data != nil || event.Changes[0].CustomerID != "" {
			t.Fatalf("revoked session retained data: %+v", event)
		}
	}
}
