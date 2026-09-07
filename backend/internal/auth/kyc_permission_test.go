package auth

import "testing"

func TestKYCReviewPermissionIsInternalAndExplicit(t *testing.T) {
	ownerPermissions := defaultPermissionsForRole("Owner")
	if !contains(ownerPermissions, "kyc.review") {
		t.Fatalf("owner defaults are missing kyc.review: %#v", ownerPermissions)
	}
	customerPermissions := permissionCatalogForRole("Customer")
	if contains(customerPermissions, "kyc.review") {
		t.Fatalf("customer role can be assigned kyc.review: %#v", customerPermissions)
	}
	internalPermissions := permissionCatalogForRole("Manager")
	if !contains(internalPermissions, "kyc.review") {
		t.Fatalf("internal permission catalog is missing kyc.review: %#v", internalPermissions)
	}
}
