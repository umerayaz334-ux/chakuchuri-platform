package workflow

import (
	"errors"
	"testing"

	"chakuchuri/backend/internal/auth"
)

func TestCustomerCannotCreateQuoteForAnotherCustomer(t *testing.T) {
	service := NewService(nil, nil)
	customer := auth.User{
		ID:         "usr_customer",
		Role:       "Customer",
		CustomerID: "cust_abc_export",
	}

	_, err := service.CreateQuote(QuoteRequest{
		CustomerID:  "cust_northern_trading",
		ProductName: "Outside account request",
		Quantity:    10,
	}, customer)
	if !errors.Is(err, errForbidden) {
		t.Fatalf("expected forbidden error, got %v", err)
	}
}

func TestInternalUserCanSeeWorkspaceWithoutCustomerAssignment(t *testing.T) {
	service := NewService(nil, nil)
	manager := auth.User{ID: "usr_manager", Role: "Manager"}

	dashboard := service.DashboardForUser(manager)
	if len(dashboard.Products) < 2 || len(dashboard.Quotations) == 0 {
		t.Fatalf("expected internal user to see company workspace, got %#v", dashboard)
	}
}
func TestPlatformSettingsRequireAdminAndNormalizeValues(t *testing.T) {
	service := NewService(nil, nil)
	customer := auth.User{ID: "usr_customer", Role: "Customer"}
	if _, err := service.UpdatePlatformSettings(PlatformSettings{ThemePreset: "Cobalt"}, customer); !errors.Is(err, errForbidden) {
		t.Fatalf("expected customer settings update to be forbidden, got %v", err)
	}

	owner := auth.User{ID: "usr_owner", Name: "Owner", Role: "Owner"}
	settings, err := service.UpdatePlatformSettings(PlatformSettings{
		ThemePreset:             "cobalt",
		PrimaryColor:            "#155EAA",
		AccentColor:             "#C75B4C",
		SurfaceColor:            "#F5F7FA",
		DefaultLanguage:         "UR",
		AllowUserLanguageChoice: true,
	}, owner)
	if err != nil {
		t.Fatalf("update settings: %v", err)
	}
	if settings.ThemePreset != "Cobalt" || settings.PrimaryColor != "#155eaa" || settings.DefaultLanguage != "ur" {
		t.Fatalf("settings were not normalized: %#v", settings)
	}
	if len(settings.EnabledLanguages) != 2 || settings.EnabledLanguages[0] != "en" || settings.EnabledLanguages[1] != "ur" {
		t.Fatalf("expected English and Urdu languages, got %#v", settings.EnabledLanguages)
	}
}

func TestPlatformSettingsRejectInvalidColor(t *testing.T) {
	service := NewService(nil, nil)
	owner := auth.User{ID: "usr_owner", Role: "Owner"}
	_, err := service.UpdatePlatformSettings(PlatformSettings{PrimaryColor: "green"}, owner)
	if !errors.Is(err, errValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}
