package auth

import (
	"errors"
	"net/http/httptest"
	"testing"
)

func TestTemporarySuspensionKeepsSessionButBlocksProductAccess(t *testing.T) {
	service := NewService(nil)
	owner := User{ID: "owner", Role: "Owner", Email: "owner@example.com", Permissions: []string{"users.manage"}}
	created, err := service.CreateManagedUser(ManagedUserRequest{
		Name:     "Customer Review",
		Email:    "review@example.com",
		Password: "secret123",
		Role:     "Customer",
	}, owner)
	if err != nil {
		t.Fatalf("create managed user: %v", err)
	}
	session, err := service.Login(created.Email, "secret123", nil)
	if err != nil {
		t.Fatalf("login before suspension: %v", err)
	}

	suspended, err := service.UpdateManagedUser(created.ID, ManagedUserRequest{
		Name:             created.Name,
		Email:            created.Email,
		Role:             created.Role,
		Status:           "Temporarily suspended",
		Permissions:      created.Permissions,
		PageAccess:       created.PageAccess,
		SuspensionNotice: "We need to verify your latest payment.",
	}, owner)
	if err != nil {
		t.Fatalf("suspend user: %v", err)
	}
	if suspended.SuspensionNotice != "We need to verify your latest payment." {
		t.Fatalf("custom suspension notice was not retained: %#v", suspended)
	}

	request := httptest.NewRequest("GET", "/api/workspace", nil)
	request.Header.Set("Authorization", "Bearer "+session.Token)
	if _, ok := service.SessionUserFromRequest(request); !ok {
		t.Fatal("suspended user should retain an authenticated session")
	}
	if _, ok := service.UserFromRequest(request); ok {
		t.Fatal("suspended user must not pass normal product authorization")
	}
	if _, err := service.Login(created.Email, "secret123", nil); err != nil {
		t.Fatalf("suspended user should be able to sign in to see the notice: %v", err)
	}

	contacted, err := service.ContactAboutSuspension(suspended, "Please review my account.")
	if err != nil {
		t.Fatalf("contact suspension support: %v", err)
	}
	if contacted.SuspensionContactAt == "" || contacted.SuspensionMessage != "Please review my account." {
		t.Fatalf("suspension contact was not recorded: %#v", contacted)
	}
}

func TestUserCanUpdateOwnLoginAndProfile(t *testing.T) {
	service := NewService(nil)
	created, err := service.CreateUser(User{
		Name:  "Profile User",
		Email: "profile@example.com",
		Role:  "Manager",
	}, "secret123")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	session, err := service.Login(created.Email, "secret123", nil)
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	if _, err := service.UpdateOwnProfile(session.User, ProfileUpdateRequest{
		Name:            "Updated User",
		Email:           "updated@example.com",
		CurrentPassword: "wrong-password",
	}, session.Token); !errors.Is(err, ErrCurrentPassword) {
		t.Fatalf("expected current-password error, got %v", err)
	}

	updated, err := service.UpdateOwnProfile(session.User, ProfileUpdateRequest{
		Name:               "Updated User",
		Email:              "updated@example.com",
		CurrentPassword:    "secret123",
		NewPassword:        "newsecret123",
		ProfileImageFileID: "file_avatar_001",
	}, session.Token)
	if err != nil {
		t.Fatalf("update own profile: %v", err)
	}
	if updated.Email != "updated@example.com" || updated.Name != "Updated User" || updated.ProfileImageFileID != "file_avatar_001" {
		t.Fatalf("profile fields not updated: %#v", updated)
	}

	request := httptest.NewRequest("GET", "/api/auth/me", nil)
	request.Header.Set("Authorization", "Bearer "+session.Token)
	current, ok := service.UserFromRequest(request)
	if !ok || current.Email != updated.Email {
		t.Fatalf("current session did not survive credential update: %#v %v", current, ok)
	}
	if _, err := service.Login("updated@example.com", "secret123", nil); err == nil {
		t.Fatal("old password should no longer work")
	}
	if _, err := service.Login("updated@example.com", "newsecret123", nil); err != nil {
		t.Fatalf("new credentials should work: %v", err)
	}
}
