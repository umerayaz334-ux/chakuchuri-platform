package auth

import (
	"net/http"
	"testing"
)

func TestRequestIPPrefersForwardedClientOverProxy(t *testing.T) {
	req := &http.Request{RemoteAddr: "127.0.0.1:5170", Header: http.Header{}}
	req.Header.Set("X-Forwarded-For", "203.0.113.50, 127.0.0.1")
	if got := requestIP(req); got != "203.0.113.50" {
		t.Fatalf("expected public client ip, got %q", got)
	}

	mapped := &http.Request{RemoteAddr: "127.0.0.1:5170", Header: http.Header{}}
	mapped.Header.Set("X-Real-IP", "::ffff:198.51.100.9")
	if got := requestIP(mapped); got != "198.51.100.9" {
		t.Fatalf("expected normalized ipv4, got %q", got)
	}

	direct := &http.Request{RemoteAddr: "192.168.10.44:55123", Header: http.Header{}}
	if got := requestIP(direct); got != "192.168.10.44" {
		t.Fatalf("expected direct remote ip, got %q", got)
	}
}

func TestKnownIPsTrackDistinctAddresses(t *testing.T) {
	service := NewService(nil)
	created, err := service.CreateUser(User{
		Name: "Ops", Email: "ops-ip@example.com", Role: "Admin", Status: "Active",
	}, "secret123")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	reqA := &http.Request{RemoteAddr: "203.0.113.10:443", Header: http.Header{}}
	if _, err := service.Login(created.Email, "secret123", reqA); err != nil {
		t.Fatalf("login a: %v", err)
	}
	reqB := &http.Request{RemoteAddr: "198.51.100.20:443", Header: http.Header{}}
	if _, err := service.Login(created.Email, "secret123", reqB); err != nil {
		t.Fatalf("login b: %v", err)
	}
	reqAAgain := &http.Request{RemoteAddr: "203.0.113.10:443", Header: http.Header{}}
	if _, err := service.Login(created.Email, "secret123", reqAAgain); err != nil {
		t.Fatalf("login a again: %v", err)
	}

	owner := User{Role: "Owner", Status: "Active", Permissions: []string{"users.manage"}}
	users, err := service.ListUsers(owner)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var found User
	for _, user := range users {
		if user.Email == created.Email {
			found = user
			break
		}
	}
	if len(found.KnownIPs) != 2 {
		t.Fatalf("expected 2 known ips, got %#v", found.KnownIPs)
	}
	if found.KnownIPs[0].IP != "203.0.113.10" {
		t.Fatalf("expected newest ip first, got %#v", found.KnownIPs)
	}
	if found.KnownIPs[0].SuccessCount != 2 || found.KnownIPs[1].SuccessCount != 1 {
		t.Fatalf("unexpected success counts: %#v", found.KnownIPs)
	}
}

func TestManagedUserDefaultsAndLoginActivity(t *testing.T) {
	service := NewService(nil)
	owner := User{ID: "owner", Role: "Owner", Email: "owner@example.com", Permissions: []string{"users.manage"}}

	created, err := service.CreateManagedUser(ManagedUserRequest{
		Name:     "Shipping Desk",
		Email:    "ship@example.com",
		Password: "secret123",
		Role:     "Shipping Staff",
	}, owner)
	if err != nil {
		t.Fatalf("create managed user: %v", err)
	}
	if created.Status != "Active" {
		t.Fatalf("expected active status, got %s", created.Status)
	}
	if !contains(created.Permissions, "shipping.manage") {
		t.Fatalf("shipping permission missing: %#v", created.Permissions)
	}
	if !contains(created.PageAccess, "Shipping") || contains(created.PageAccess, "Payments") {
		t.Fatalf("unexpected shipping page access: %#v", created.PageAccess)
	}

	if _, err := service.Login("ship@example.com", "wrong", nil); err == nil {
		t.Fatal("wrong password should fail")
	}
	users, err := service.ListUsers(owner)
	if err != nil {
		t.Fatalf("list users: %v", err)
	}
	var found User
	for _, user := range users {
		if user.Email == "ship@example.com" {
			found = user
		}
	}
	if found.FailedLoginCount != 1 || len(found.LoginActivity) == 0 || found.LoginActivity[0].Result != "Failed" {
		t.Fatalf("failed login was not recorded: %#v", found)
	}
}

func TestBlockedManagedUserCannotLogin(t *testing.T) {
	service := NewService(nil)
	owner := User{ID: "owner", Role: "Owner", Email: "owner@example.com", Permissions: []string{"users.manage"}}

	created, err := service.CreateManagedUser(ManagedUserRequest{
		Name:     "Accountant",
		Email:    "accounts@example.com",
		Password: "secret123",
		Role:     "Accountant",
	}, owner)
	if err != nil {
		t.Fatalf("create managed user: %v", err)
	}
	if _, err := service.UpdateManagedUser(created.ID, ManagedUserRequest{
		Name:        created.Name,
		Email:       created.Email,
		Role:        created.Role,
		Status:      "Blocked",
		Permissions: created.Permissions,
		PageAccess:  created.PageAccess,
	}, owner); err != nil {
		t.Fatalf("block user: %v", err)
	}

	if _, err := service.Login("accounts@example.com", "secret123", nil); err == nil {
		t.Fatal("blocked user should not login")
	}
}

func TestCustomerRoleGetsOwnPortalAccount(t *testing.T) {
	service := NewService(nil)
	owner := User{ID: "owner", Role: "Owner", Email: "owner@example.com", Permissions: []string{"users.manage"}}

	created, err := service.CreateManagedUser(ManagedUserRequest{
		Name:                "Blade Export Buyer",
		Email:               "buyer@example.com",
		Password:            "secret123",
		Role:                "Customer",
		AssignedCustomerIDs: []string{"cust_other"},
		Permissions:         []string{"quotes.create", "customers.manage"},
		PageAccess:          []string{"Home", "Users & Access"},
	}, owner)
	if err != nil {
		t.Fatalf("create customer portal user: %v", err)
	}
	if created.CustomerID != "cust_blade_export_buyer" {
		t.Fatalf("expected generated customer account, got %s", created.CustomerID)
	}
	if len(created.AssignedCustomerIDs) != 0 {
		t.Fatalf("customer role should not keep assigned customers: %#v", created.AssignedCustomerIDs)
	}
	if contains(created.Permissions, "customers.manage") {
		t.Fatalf("customer role kept internal permission: %#v", created.Permissions)
	}
	if contains(created.PageAccess, "Users & Access") || !contains(created.PageAccess, "Home") {
		t.Fatalf("customer role kept wrong pages: %#v", created.PageAccess)
	}
}

func TestInternalRoleDoesNotKeepCustomerAssignment(t *testing.T) {
	service := NewService(nil)
	owner := User{ID: "owner", Role: "Owner", Email: "owner@example.com", Permissions: []string{"users.manage"}}

	created, err := service.CreateManagedUser(ManagedUserRequest{
		Name:                "Operations Manager",
		Email:               "manager@example.com",
		Password:            "secret123",
		Role:                "Manager",
		CustomerID:          "cust_abc_export",
		AssignedCustomerIDs: []string{"cust_abc_export"},
		Permissions:         []string{"quotes.manage", "quotes.create"},
		PageAccess:          []string{"Quotations", "Home"},
	}, owner)
	if err != nil {
		t.Fatalf("create internal user: %v", err)
	}
	if created.CustomerID != "" || len(created.AssignedCustomerIDs) != 0 {
		t.Fatalf("internal user should not keep customer assignment: %#v", created)
	}
	if contains(created.Permissions, "quotes.create") || !contains(created.Permissions, "quotes.manage") {
		t.Fatalf("internal user permissions were not filtered correctly: %#v", created.Permissions)
	}
	if contains(created.PageAccess, "Home") || !contains(created.PageAccess, "Quotations") {
		t.Fatalf("internal user pages were not filtered correctly: %#v", created.PageAccess)
	}
}

func TestMarkNotificationsReadPersistsForAccount(t *testing.T) {
	service := NewService(nil)
	owner := User{ID: "owner", Role: "Owner", Email: "owner@example.com", Permissions: []string{"users.manage"}}
	created, err := service.CreateManagedUser(ManagedUserRequest{
		Name:     "Desk User",
		Email:    "desk@example.com",
		Password: "secret123",
		Role:     "Owner",
	}, owner)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	updated, err := service.MarkNotificationsRead(created, []string{"quote-priced-Q1", "quote-priced-Q1", "payment-review-P1"})
	if err != nil {
		t.Fatalf("mark read: %v", err)
	}
	if len(updated.NotificationReadIDs) != 2 {
		t.Fatalf("expected 2 unique reads, got %#v", updated.NotificationReadIDs)
	}
	again, err := service.MarkNotificationsRead(created, []string{"order-update-O1"})
	if err != nil {
		t.Fatalf("mark more: %v", err)
	}
	if len(again.NotificationReadIDs) != 3 {
		t.Fatalf("expected merged reads, got %#v", again.NotificationReadIDs)
	}
	listed, err := service.ListUsers(created)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, user := range listed {
		if len(user.NotificationReadIDs) != 0 {
			t.Fatalf("list users should not expose notification reads: %#v", user.NotificationReadIDs)
		}
	}
}
func contains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
