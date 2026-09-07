package workflow

import (
	"encoding/json"
	"strings"
	"time"

	"chakuchuri/backend/internal/auth"
)

type FeaturedProduct struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Tag         string `json:"tag"`
	Caption     string `json:"caption"`
	ImageFileID string `json:"imageFileId"`
	ImageName   string `json:"imageName"`
	ProductID   string `json:"productId,omitempty"`
	SortOrder   int    `json:"sortOrder"`
	Active      bool   `json:"active"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
	CreatedBy   string `json:"createdBy"`
}

type FeaturedProductRequest struct {
	Name        string `json:"name"`
	Tag         string `json:"tag"`
	Caption     string `json:"caption"`
	ImageFileID string `json:"imageFileId"`
	ImageName   string `json:"imageName"`
	ProductID   string `json:"productId"`
	SortOrder   *int   `json:"sortOrder"`
	Active      *bool  `json:"active"`
}

func normalizeFeatureTag(value string) string {
	tag := strings.TrimSpace(value)
	if tag == "" {
		return "Featured"
	}
	if len(tag) > 32 {
		return tag[:32]
	}
	return tag
}

func filterFeaturedForUser(items []FeaturedProduct, user auth.User) []FeaturedProduct {
	isStaff := canManageCustomerNotices(user)
	out := make([]FeaturedProduct, 0, len(items))
	for _, item := range items {
		if isStaff || item.Active {
			out = append(out, item)
		}
	}
	sortFeatured(out)
	return out
}

func sortFeatured(items []FeaturedProduct) {
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].SortOrder < items[i].SortOrder ||
				(items[j].SortOrder == items[i].SortOrder && items[j].UpdatedAt > items[i].UpdatedAt) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

func (s *Service) CreateFeaturedProduct(payload FeaturedProductRequest, actor auth.User) (FeaturedProduct, error) {
	if !canManageCustomerNotices(actor) {
		return FeaturedProduct{}, errForbidden
	}
	name := strings.TrimSpace(payload.Name)
	if name == "" {
		return FeaturedProduct{}, errValidation
	}
	imageID := strings.TrimSpace(payload.ImageFileID)
	if imageID == "" {
		return FeaturedProduct{}, errValidation
	}
	now := time.Now().UTC().Format(time.RFC3339)
	active := true
	if payload.Active != nil {
		active = *payload.Active
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	sortOrder := len(s.featured)
	if payload.SortOrder != nil {
		sortOrder = *payload.SortOrder
	}
	item := FeaturedProduct{
		ID:          s.nextID("FEAT"),
		Name:        name,
		Tag:         normalizeFeatureTag(payload.Tag),
		Caption:     strings.TrimSpace(payload.Caption),
		ImageFileID: imageID,
		ImageName:   strings.TrimSpace(payload.ImageName),
		ProductID:   strings.TrimSpace(payload.ProductID),
		SortOrder:   sortOrder,
		Active:      active,
		CreatedAt:   now,
		UpdatedAt:   now,
		CreatedBy:   actor.Name,
	}
	if err := s.commitAppHomeLocked(s.notices, append([]FeaturedProduct{item}, s.featured...)); err != nil {
		return FeaturedProduct{}, err
	}
	s.notifyWorkspaceUpsert("featured", "", item)
	return item, nil
}

func (s *Service) UpdateFeaturedProduct(id string, payload FeaturedProductRequest, actor auth.User) (FeaturedProduct, error) {
	if !canManageCustomerNotices(actor) {
		return FeaturedProduct{}, errForbidden
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return FeaturedProduct{}, errValidation
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for index, item := range s.featured {
		if item.ID != id {
			continue
		}
		name := strings.TrimSpace(payload.Name)
		if name == "" {
			name = item.Name
		}
		imageID := strings.TrimSpace(payload.ImageFileID)
		if imageID == "" {
			imageID = item.ImageFileID
		}
		if name == "" || imageID == "" {
			return FeaturedProduct{}, errValidation
		}
		active := item.Active
		if payload.Active != nil {
			active = *payload.Active
		}
		sortOrder := item.SortOrder
		if payload.SortOrder != nil {
			sortOrder = *payload.SortOrder
		}
		updated := FeaturedProduct{
			ID:          item.ID,
			Name:        name,
			Tag:         normalizeFeatureTag(firstNonEmpty(payload.Tag, item.Tag)),
			Caption:     firstNonEmpty(strings.TrimSpace(payload.Caption), item.Caption),
			ImageFileID: imageID,
			ImageName:   firstNonEmpty(strings.TrimSpace(payload.ImageName), item.ImageName),
			ProductID:   firstNonEmpty(strings.TrimSpace(payload.ProductID), item.ProductID),
			SortOrder:   sortOrder,
			Active:      active,
			CreatedAt:   item.CreatedAt,
			UpdatedAt:   time.Now().UTC().Format(time.RFC3339),
			CreatedBy:   item.CreatedBy,
		}
		next := append([]FeaturedProduct(nil), s.featured...)
		next[index] = updated
		if err := s.commitAppHomeLocked(s.notices, next); err != nil {
			return FeaturedProduct{}, err
		}
		s.notifyWorkspaceUpsert("featured", "", updated)
		return updated, nil
	}
	return FeaturedProduct{}, errNotFound
}

func (s *Service) DeleteFeaturedProduct(id string, actor auth.User) (FeaturedProduct, error) {
	if !canManageCustomerNotices(actor) {
		return FeaturedProduct{}, errForbidden
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return FeaturedProduct{}, errValidation
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for index, item := range s.featured {
		if item.ID != id {
			continue
		}
		raw, _ := json.Marshal(map[string]string{"id": item.ID})
		next := append([]FeaturedProduct(nil), s.featured[:index]...)
		next = append(next, s.featured[index+1:]...)
		if err := s.commitAppHomeLocked(s.notices, next); err != nil {
			return FeaturedProduct{}, err
		}
		s.notifyWorkspaceChanges(WorkspaceChange{
			Operation: "remove",
			Scope:     "featured",
			Data:      raw,
		})
		return item, nil
	}
	return FeaturedProduct{}, errNotFound
}
