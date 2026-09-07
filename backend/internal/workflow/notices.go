package workflow

import (
	"encoding/json"
	"strings"
	"time"

	"chakuchuri/backend/internal/auth"
)

type CustomerNotice struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Body        string   `json:"body"`
	Tone        string   `json:"tone"`
	Audience    string   `json:"audience"`
	CustomerIDs []string `json:"customerIds"`
	Active      bool     `json:"active"`
	CreatedAt   string   `json:"createdAt"`
	UpdatedAt   string   `json:"updatedAt"`
	CreatedBy   string   `json:"createdBy"`
}

type CustomerNoticeRequest struct {
	Title       string   `json:"title"`
	Body        string   `json:"body"`
	Tone        string   `json:"tone"`
	Audience    string   `json:"audience"`
	CustomerIDs []string `json:"customerIds"`
	Active      *bool    `json:"active"`
}

func normalizeNoticeTone(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "success":
		return "success"
	case "warning":
		return "warning"
	default:
		return "info"
	}
}

func normalizeNoticeAudience(value string, customerIDs []string) (string, []string, error) {
	ids := uniqueNonEmpty(customerIDs)
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "selected":
		if len(ids) == 0 {
			return "", nil, errValidation
		}
		return "selected", ids, nil
	default:
		return "all", nil, nil
	}
}

func uniqueNonEmpty(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		id := strings.TrimSpace(value)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func noticeVisibleToCustomer(notice CustomerNotice, customerID string) bool {
	if !notice.Active {
		return false
	}
	if notice.Audience != "selected" {
		return true
	}
	customerID = strings.TrimSpace(customerID)
	if customerID == "" {
		return false
	}
	for _, id := range notice.CustomerIDs {
		if id == customerID {
			return true
		}
	}
	return false
}

func filterNoticesForUser(notices []CustomerNotice, user auth.User) []CustomerNotice {
	out := make([]CustomerNotice, 0, len(notices))
	for _, notice := range notices {
		if noticeVisibleToUser(notice, user) {
			if !canManageCustomerNotices(user) {
				notice.CustomerIDs = nil
			}
			out = append(out, notice)
		}
	}
	return out
}

func noticeVisibleToUser(notice CustomerNotice, user auth.User) bool {
	if canManageCustomerNotices(user) {
		return true
	}
	if auth.IsCustomerRole(user) {
		return noticeVisibleToCustomer(notice, user.CustomerID)
	}
	if !notice.Active {
		return false
	}
	if notice.Audience != "selected" {
		return true
	}
	for _, id := range notice.CustomerIDs {
		if auth.CanAccessCustomer(user, id) {
			return true
		}
	}
	return false
}

func (s *Service) CreateCustomerNotice(payload CustomerNoticeRequest, actor auth.User) (CustomerNotice, error) {
	if !canManageCustomerNotices(actor) {
		return CustomerNotice{}, errForbidden
	}
	title := strings.TrimSpace(payload.Title)
	body := strings.TrimSpace(payload.Body)
	if title == "" || body == "" {
		return CustomerNotice{}, errValidation
	}
	audience, customerIDs, err := normalizeNoticeAudience(payload.Audience, payload.CustomerIDs)
	if err != nil {
		return CustomerNotice{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	active := true
	if payload.Active != nil {
		active = *payload.Active
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	notice := CustomerNotice{
		ID:          s.nextID("NTC"),
		Title:       title,
		Body:        body,
		Tone:        normalizeNoticeTone(payload.Tone),
		Audience:    audience,
		CustomerIDs: customerIDs,
		Active:      active,
		CreatedAt:   now,
		UpdatedAt:   now,
		CreatedBy:   actor.Name,
	}
	if err := s.commitAppHomeLocked(append([]CustomerNotice{notice}, s.notices...), s.featured); err != nil {
		return CustomerNotice{}, err
	}
	s.notifyWorkspaceUpsert("notices", "", notice)
	return notice, nil
}

func (s *Service) UpdateCustomerNotice(id string, payload CustomerNoticeRequest, actor auth.User) (CustomerNotice, error) {
	if !canManageCustomerNotices(actor) {
		return CustomerNotice{}, errForbidden
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return CustomerNotice{}, errValidation
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for index, notice := range s.notices {
		if notice.ID != id {
			continue
		}
		title := strings.TrimSpace(payload.Title)
		body := strings.TrimSpace(payload.Body)
		if title == "" {
			title = notice.Title
		}
		if body == "" {
			body = notice.Body
		}
		if title == "" || body == "" {
			return CustomerNotice{}, errValidation
		}
		audience := notice.Audience
		customerIDs := append([]string(nil), notice.CustomerIDs...)
		if strings.TrimSpace(payload.Audience) != "" || payload.CustomerIDs != nil {
			nextAudience, nextIDs, err := normalizeNoticeAudience(payload.Audience, payload.CustomerIDs)
			if err != nil {
				return CustomerNotice{}, err
			}
			audience = nextAudience
			customerIDs = nextIDs
		}
		active := notice.Active
		if payload.Active != nil {
			active = *payload.Active
		}
		updated := CustomerNotice{
			ID:          notice.ID,
			Title:       title,
			Body:        body,
			Tone:        normalizeNoticeTone(firstNonEmpty(payload.Tone, notice.Tone)),
			Audience:    audience,
			CustomerIDs: customerIDs,
			Active:      active,
			CreatedAt:   notice.CreatedAt,
			UpdatedAt:   time.Now().UTC().Format(time.RFC3339),
			CreatedBy:   notice.CreatedBy,
		}
		next := append([]CustomerNotice(nil), s.notices...)
		next[index] = updated
		if err := s.commitAppHomeLocked(next, s.featured); err != nil {
			return CustomerNotice{}, err
		}
		s.notifyWorkspaceUpsert("notices", "", updated)
		return updated, nil
	}
	return CustomerNotice{}, errNotFound
}

func (s *Service) DeleteCustomerNotice(id string, actor auth.User) (CustomerNotice, error) {
	if !canManageCustomerNotices(actor) {
		return CustomerNotice{}, errForbidden
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return CustomerNotice{}, errValidation
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for index, notice := range s.notices {
		if notice.ID != id {
			continue
		}
		raw, _ := json.Marshal(map[string]string{"id": notice.ID})
		next := append([]CustomerNotice(nil), s.notices[:index]...)
		next = append(next, s.notices[index+1:]...)
		if err := s.commitAppHomeLocked(next, s.featured); err != nil {
			return CustomerNotice{}, err
		}
		s.notifyWorkspaceChanges(WorkspaceChange{
			Operation: "remove",
			Scope:     "notices",
			Data:      raw,
		})
		return notice, nil
	}
	return CustomerNotice{}, errNotFound
}

func canManageCustomerNotices(actor auth.User) bool {
	role := strings.ToLower(strings.TrimSpace(actor.Role))
	if role == "owner" || role == "admin" || role == "manager" {
		return true
	}
	for _, permission := range actor.Permissions {
		if permission == "customers.manage" || permission == "platform.manage" {
			return true
		}
	}
	return false
}
