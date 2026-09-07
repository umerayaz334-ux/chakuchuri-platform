package workflow

import (
	"errors"
	"testing"

	"chakuchuri/backend/internal/auth"
)

func TestMoveOrderBuildsConsistentTimeline(t *testing.T) {
	service := NewService(nil, nil)
	admin := auth.User{ID: "usr_admin", Name: "Production Admin", Role: "Owner"}

	order, err := service.MoveOrder("MFG-24091", "Quality check", admin)
	if err != nil {
		t.Fatalf("move order: %v", err)
	}
	if order.Status != "Quality check" || order.CurrentStage != "Quality check" || order.Progress != 78 {
		t.Fatalf("unexpected order stage: %#v", order)
	}

	qualityIndex := stageIndex(order.Stages, "Quality check")
	if qualityIndex < 0 || order.Stages[qualityIndex].Status != "active" {
		t.Fatalf("quality check should be active: %#v", order.Stages)
	}
	for index, stage := range order.Stages {
		if index < qualityIndex && stage.Status != "done" {
			t.Fatalf("previous stage %q should be done: %#v", stage.Name, order.Stages)
		}
		if index > qualityIndex && stage.Status != "pending" {
			t.Fatalf("future stage %q should be pending: %#v", stage.Name, order.Stages)
		}
	}

	if _, err := service.MoveOrder(order.ID, "Unknown stage", admin); !errors.Is(err, errValidation) {
		t.Fatalf("invalid stage error = %v, want validation", err)
	}
}

func TestUpdateOrderPublishesCustomerVisibleUpdate(t *testing.T) {
	service := NewService(nil, nil)
	admin := auth.User{ID: "usr_admin", Name: "Production Admin", Role: "Owner"}

	order, err := service.UpdateOrder("MFG-24091", OrderUpdateRequest{
		Detail:       "Handle assembly completed and final inspection is booked.",
		ExpectedDate: "2026-09-18",
	}, admin)
	if err != nil {
		t.Fatalf("update order: %v", err)
	}
	if order.ExpectedDate != "2026-09-18" {
		t.Fatalf("expected date = %q", order.ExpectedDate)
	}
	if len(order.History) == 0 || order.History[0].Label != "Production update" || order.History[0].Actor != admin.Name {
		t.Fatalf("production update missing: %#v", order.History)
	}

	otherCustomer := auth.User{ID: "usr_other", Role: "Customer", CustomerID: "cust_other"}
	if _, err := service.UpdateOrder(order.ID, OrderUpdateRequest{Detail: "Not allowed"}, otherCustomer); !errors.Is(err, errForbidden) {
		t.Fatalf("cross-customer update error = %v, want forbidden", err)
	}
}

func stageIndex(stages []Stage, name string) int {
	for index, stage := range stages {
		if stage.Name == name {
			return index
		}
	}
	return -1
}
