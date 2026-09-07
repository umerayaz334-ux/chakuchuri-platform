package workflow

import (
	"fmt"
	"testing"
)

func TestWorkspacePaginationDefaultsToTwentyAndKeepsTotals(t *testing.T) {
	rows := make([]Quotation, 45)
	for index := range rows {
		rows[index].ID = string(rune('A' + index))
	}
	dashboard := paginateDashboard(Dashboard{Quotations: rows}, 20)
	if len(dashboard.Quotations) != 20 {
		t.Fatalf("loaded %d quotations", len(dashboard.Quotations))
	}
	info := dashboard.Pagination["quotations"]
	if info.Loaded != 20 || info.Total != 45 || !info.HasMore {
		t.Fatalf("unexpected page info: %+v", info)
	}
	page, ok := workspacePage(Dashboard{Quotations: rows}, "quotations", 20, 20)
	if !ok || len(page.Items.([]Quotation)) != 20 || page.Pagination.Loaded != 40 || !page.Pagination.HasMore {
		t.Fatalf("unexpected second page: %+v", page)
	}
	last, ok := workspacePage(Dashboard{Quotations: rows}, "quotations", 40, 20)
	if !ok || len(last.Items.([]Quotation)) != 5 || last.Pagination.Loaded != 45 || last.Pagination.HasMore {
		t.Fatalf("unexpected last page: %+v", last)
	}
}

func TestWorkspacePaginationRejectsUnknownScopeAndBoundsLimit(t *testing.T) {
	rows := make([]CallRequest, 150)
	page, ok := workspacePage(Dashboard{Calls: rows}, "calls", -10, 1000)
	if !ok || len(page.Items.([]CallRequest)) != maxWorkspacePageSize {
		t.Fatalf("limit not bounded: %+v", page)
	}
	if _, ok := workspacePage(Dashboard{}, "secrets", 0, 20); ok {
		t.Fatal("unknown scope accepted")
	}
}

func TestConversationMessagesKeepLatestTwentyAndLoadEarlier(t *testing.T) {
	messages := make([]Message, 45)
	for index := range messages {
		messages[index].ID = fmt.Sprintf("message-%02d", index)
	}
	conversation := Conversation{ID: "conversation-1", Messages: messages}

	latest := compactConversation(conversation, 0, 20)
	if len(latest.Messages) != 20 || latest.Messages[0].ID != "message-25" || latest.MessagePagination.Total != 45 || !latest.MessagePagination.HasMore {
		t.Fatalf("unexpected latest page: first=%q len=%d pagination=%+v", latest.Messages[0].ID, len(latest.Messages), latest.MessagePagination)
	}

	earlier := compactConversation(conversation, 20, 20)
	if len(earlier.Messages) != 20 || earlier.Messages[0].ID != "message-05" || earlier.MessagePagination.Loaded != 40 || !earlier.MessagePagination.HasMore {
		t.Fatalf("unexpected earlier page: first=%q len=%d pagination=%+v", earlier.Messages[0].ID, len(earlier.Messages), earlier.MessagePagination)
	}
}
