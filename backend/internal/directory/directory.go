package directory

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"chakuchuri/backend/internal/audit"
	"chakuchuri/backend/internal/auth"
	"chakuchuri/backend/internal/platform/httpx"
)

var (
	errForbidden       = errors.New("forbidden")
	errNotFound        = errors.New("not found")
	errValidation      = errors.New("validation")
	errInvalidCategory = errors.New("invalid category")
	errInvalidListing  = errors.New("invalid listing")
)

type Category struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	Active    bool   `json:"active"`
	SortOrder int    `json:"sortOrder"`
	CreatedAt string `json:"createdAt"`
}

type Listing struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	CategoryIDs       []string `json:"categoryIds"`
	Phone             string   `json:"phone"`
	WhatsApp          string   `json:"whatsapp"`
	Address           string   `json:"address"`
	City              string   `json:"city"`
	Note              string   `json:"note"`
	PhotoFileID       string   `json:"photoFileId,omitempty"`
	Status            string   `json:"status"`
	SubmittedByUserID string   `json:"submittedByUserId,omitempty"`
	CustomerID        string   `json:"customerId,omitempty"`
	ReviewedByUserID  string   `json:"reviewedByUserId,omitempty"`
	ReviewedAt        string   `json:"reviewedAt,omitempty"`
	ReviewNote        string   `json:"reviewNote,omitempty"`
	CreatedAt         string   `json:"createdAt"`
	UpdatedAt         string   `json:"updatedAt"`
}

type State struct {
	Version    int        `json:"version"`
	Categories []Category `json:"categories"`
	Listings   []Listing  `json:"listings"`
}

type ListingPayload struct {
	Name        string   `json:"name"`
	CategoryIDs []string `json:"categoryIds"`
	Phone       string   `json:"phone"`
	WhatsApp    string   `json:"whatsapp"`
	Address     string   `json:"address"`
	City        string   `json:"city"`
	Note        string   `json:"note"`
	PhotoFileID string   `json:"photoFileId"`
	Status      string   `json:"status"`
}

type CategoryPayload struct {
	Name      string `json:"name"`
	Active    *bool  `json:"active"`
	SortOrder *int   `json:"sortOrder"`
}

type ReviewPayload struct {
	Status string `json:"status"`
	Note   string `json:"note"`
}

type Service struct {
	mu         sync.RWMutex
	auth       *auth.Service
	recorder   *audit.Recorder
	repository Repository
	categories []Category
	listings   []Listing
}

func NewService(authService *auth.Service, recorder *audit.Recorder) *Service {
	categories := seedCategories()
	service := &Service{
		auth:       authService,
		recorder:   recorder,
		categories: categories,
		listings:   seedListings(categories),
	}
	return service
}

func seedCategories() []Category {
	now := time.Now().UTC().Format(time.RFC3339)
	names := []string{"Knife maker", "Box maker", "Steel seller", "Handle maker", "Finisher"}
	rows := make([]Category, 0, len(names))
	for index, name := range names {
		rows = append(rows, Category{
			ID:        "dcat_" + slugify(name),
			Name:      name,
			Slug:      slugify(name),
			Active:    true,
			SortOrder: index + 1,
			CreatedAt: now,
		})
	}
	return rows
}

func seedListings(categories []Category) []Listing {
	bySlug := map[string]string{}
	for _, row := range categories {
		bySlug[row.Slug] = row.ID
	}
	cat := func(slugs ...string) []string {
		ids := make([]string, 0, len(slugs))
		for _, slug := range slugs {
			if id := bySlug[slug]; id != "" {
				ids = append(ids, id)
			}
		}
		return ids
	}
	now := time.Now().UTC().Format(time.RFC3339)
	type sample struct {
		id, name, phone, whatsapp, address, note string
		slugs                                    []string
	}
	samples := []sample{
		{
			id: "dlst_sample_malik_cutlery", name: "Malik Cutlery Works",
			phone: "03001234567", whatsapp: "923001234567",
			address: "Near Clock Tower, Main Bazaar", note: "Handmade chef knives, hunting blades, and export finishes.",
			slugs: []string{"knife-maker", "finisher"},
		},
		{
			id: "dlst_sample_box_house", name: "Sialkot Road Box House",
			phone: "03017654321", whatsapp: "923017654321",
			address: "Sialkot Road, Industrial Area", note: "Wooden knife boxes, gift packaging, and foam inserts.",
			slugs: []string{"box-maker"},
		},
		{
			id: "dlst_sample_chenab_steel", name: "Chenab Steel Traders",
			phone: "03219876543", whatsapp: "923219876543",
			address: "Steel Market, Circular Road", note: "Damascus blanks, stainless sheets, and bar stock for makers.",
			slugs: []string{"steel-seller"},
		},
		{
			id: "dlst_sample_bone_handles", name: "Classic Bone Handles",
			phone: "03451112233", whatsapp: "923451112233",
			address: "Mohalla Qasaban", note: "Bone, wood, and micarta handle scales cut to size.",
			slugs: []string{"handle-maker"},
		},
		{
			id: "dlst_sample_mirror_polish", name: "Mirror Polish Finish Co.",
			phone: "03124445566", whatsapp: "923124445566",
			address: "Ghakhar Bypass Link", note: "Mirror polish, satin, and stonewash finishing for blades.",
			slugs: []string{"finisher"},
		},
		{
			id: "dlst_sample_ali_forge", name: "Ali Forge & Tools",
			phone: "03005556677", whatsapp: "923005556677",
			address: "Nawan Pind Road", note: "Custom knife forging and small-batch export runs.",
			slugs: []string{"knife-maker"},
		},
	}
	rows := make([]Listing, 0, len(samples))
	for _, item := range samples {
		ids := cat(item.slugs...)
		if len(ids) == 0 {
			continue
		}
		rows = append(rows, Listing{
			ID:          item.id,
			Name:        item.name,
			CategoryIDs: ids,
			Phone:       item.phone,
			WhatsApp:    item.whatsapp,
			Address:     item.address,
			City:        "Wazirabad",
			Note:        item.note,
			Status:      "approved",
			CreatedAt:   now,
			UpdatedAt:   now,
			ReviewedAt:  now,
		})
	}
	return rows
}

func (s *Service) record(actor auth.User, action, entity, detail string) {
	if s.recorder == nil {
		return
	}
	s.recorder.Record(actor.Email, action, entity, detail)
}

func (s *Service) recordSystem(action, detail string) {
	if s.recorder == nil {
		return
	}
	s.recorder.Record("system", action, "directory", detail)
}

func (s *Service) snapshotLocked() State {
	return State{
		Version:    1,
		Categories: append([]Category(nil), s.categories...),
		Listings:   append([]Listing(nil), s.listings...),
	}
}

func (s *Service) persistLocked() {
	if s.repository == nil {
		return
	}
	if err := s.repository.SaveState(s.snapshotLocked()); err != nil {
		s.recordSystem("directory.persist_error", err.Error())
	}
}

func (s *Service) ListActiveCategories() []Category {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows := make([]Category, 0, len(s.categories))
	for _, row := range s.categories {
		if row.Active {
			rows = append(rows, row)
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].SortOrder == rows[j].SortOrder {
			return strings.ToLower(rows[i].Name) < strings.ToLower(rows[j].Name)
		}
		return rows[i].SortOrder < rows[j].SortOrder
	})
	return rows
}

func (s *Service) ListAllCategories(actor auth.User) ([]Category, error) {
	if !auth.HasPermission(actor, "directory.manage") {
		return nil, errForbidden
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows := append([]Category(nil), s.categories...)
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].SortOrder == rows[j].SortOrder {
			return strings.ToLower(rows[i].Name) < strings.ToLower(rows[j].Name)
		}
		return rows[i].SortOrder < rows[j].SortOrder
	})
	return rows, nil
}

func (s *Service) CreateCategory(payload CategoryPayload, actor auth.User) (Category, error) {
	if !auth.HasPermission(actor, "directory.manage") {
		return Category{}, errForbidden
	}
	name := strings.TrimSpace(payload.Name)
	if name == "" {
		return Category{}, errInvalidCategory
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.persistLocked()

	slug := slugify(name)
	for _, row := range s.categories {
		if row.Slug == slug || strings.EqualFold(row.Name, name) {
			return Category{}, errInvalidCategory
		}
	}
	maxSort := 0
	for _, row := range s.categories {
		if row.SortOrder > maxSort {
			maxSort = row.SortOrder
		}
	}
	row := Category{
		ID:        "dcat_" + randomID(8),
		Name:      name,
		Slug:      slug,
		Active:    true,
		SortOrder: maxSort + 1,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	s.categories = append(s.categories, row)
	s.record(actor, "directory.category_created", row.ID, "Category "+row.Name+" created.")
	return row, nil
}

func (s *Service) UpdateCategory(id string, payload CategoryPayload, actor auth.User) (Category, error) {
	if !auth.HasPermission(actor, "directory.manage") {
		return Category{}, errForbidden
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.persistLocked()

	for index := range s.categories {
		if s.categories[index].ID != id {
			continue
		}
		if name := strings.TrimSpace(payload.Name); name != "" {
			slug := slugify(name)
			for _, other := range s.categories {
				if other.ID != id && (other.Slug == slug || strings.EqualFold(other.Name, name)) {
					return Category{}, errInvalidCategory
				}
			}
			s.categories[index].Name = name
			s.categories[index].Slug = slug
		}
		if payload.Active != nil {
			s.categories[index].Active = *payload.Active
		}
		if payload.SortOrder != nil {
			s.categories[index].SortOrder = *payload.SortOrder
		}
		s.record(actor, "directory.category_updated", id, "Category updated.")
		return s.categories[index], nil
	}
	return Category{}, errNotFound
}

func (s *Service) ListApprovedListings(query, categoryID string) []Listing {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.filterListingsLocked("approved", "", query, categoryID)
}

func (s *Service) ListMine(actor auth.User) ([]Listing, error) {
	if !auth.IsCustomerRole(actor) {
		return nil, errForbidden
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows := make([]Listing, 0)
	for _, row := range s.listings {
		if row.SubmittedByUserID == actor.ID || (actor.CustomerID != "" && row.CustomerID == actor.CustomerID) {
			rows = append(rows, row)
		}
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].CreatedAt > rows[j].CreatedAt })
	return rows, nil
}

func (s *Service) ListAdminListings(actor auth.User, status, query, categoryID string) ([]Listing, error) {
	if !auth.HasPermission(actor, "directory.manage") {
		return nil, errForbidden
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.filterListingsLocked(status, "", query, categoryID), nil
}

func (s *Service) filterListingsLocked(status, submitterID, query, categoryID string) []Listing {
	query = strings.ToLower(strings.TrimSpace(query))
	categoryID = strings.TrimSpace(categoryID)
	status = strings.ToLower(strings.TrimSpace(status))
	rows := make([]Listing, 0)
	for _, row := range s.listings {
		if status != "" && status != "all" && !strings.EqualFold(row.Status, status) {
			continue
		}
		if submitterID != "" && row.SubmittedByUserID != submitterID {
			continue
		}
		if categoryID != "" && !containsString(row.CategoryIDs, categoryID) {
			continue
		}
		if query != "" {
			hay := strings.ToLower(strings.Join([]string{row.Name, row.Phone, row.WhatsApp, row.Address, row.City, row.Note}, " "))
			if !strings.Contains(hay, query) {
				continue
			}
		}
		rows = append(rows, row)
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].CreatedAt > rows[j].CreatedAt })
	return rows
}

func paginateListings(r *http.Request, rows []Listing) ([]Listing, map[string]interface{}) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if offset < 0 {
		offset = 0
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset > len(rows) {
		offset = len(rows)
	}
	end := offset + limit
	if end > len(rows) {
		end = len(rows)
	}
	return rows[offset:end], map[string]interface{}{"loaded": end, "total": len(rows), "hasMore": end < len(rows)}
}

func (s *Service) validCategoryIDsLocked(ids []string, requireActive bool) ([]string, bool) {
	clean := make([]string, 0, len(ids))
	seen := map[string]struct{}{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		found := false
		for _, cat := range s.categories {
			if cat.ID != id {
				continue
			}
			if requireActive && !cat.Active {
				return nil, false
			}
			found = true
			break
		}
		if !found {
			return nil, false
		}
		seen[id] = struct{}{}
		clean = append(clean, id)
	}
	if len(clean) == 0 {
		return nil, false
	}
	return clean, true
}

func (s *Service) SubmitListing(payload ListingPayload, actor auth.User) (Listing, error) {
	if !auth.IsCustomerRole(actor) {
		return Listing{}, errForbidden
	}
	name := strings.TrimSpace(payload.Name)
	phone := strings.TrimSpace(payload.Phone)
	if name == "" || phone == "" {
		return Listing{}, errValidation
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.persistLocked()

	categoryIDs, ok := s.validCategoryIDsLocked(payload.CategoryIDs, true)
	if !ok {
		return Listing{}, errInvalidCategory
	}
	now := time.Now().UTC().Format(time.RFC3339)
	row := Listing{
		ID:                "dlst_" + randomID(10),
		Name:              name,
		CategoryIDs:       categoryIDs,
		Phone:             phone,
		WhatsApp:          firstNonEmpty(strings.TrimSpace(payload.WhatsApp), phone),
		Address:           strings.TrimSpace(payload.Address),
		City:              firstNonEmpty(strings.TrimSpace(payload.City), "Wazirabad"),
		Note:              strings.TrimSpace(payload.Note),
		PhotoFileID:       strings.TrimSpace(payload.PhotoFileID),
		Status:            "pending",
		SubmittedByUserID: actor.ID,
		CustomerID:        actor.CustomerID,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	s.listings = append([]Listing{row}, s.listings...)
	s.record(actor, "directory.listing_submitted", row.ID, "Directory listing submitted.")
	return row, nil
}

func (s *Service) AdminCreateListing(payload ListingPayload, actor auth.User) (Listing, error) {
	if !auth.HasPermission(actor, "directory.manage") {
		return Listing{}, errForbidden
	}
	name := strings.TrimSpace(payload.Name)
	phone := strings.TrimSpace(payload.Phone)
	if name == "" || phone == "" {
		return Listing{}, errValidation
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.persistLocked()

	categoryIDs, ok := s.validCategoryIDsLocked(payload.CategoryIDs, false)
	if !ok {
		return Listing{}, errInvalidCategory
	}
	now := time.Now().UTC().Format(time.RFC3339)
	status := strings.ToLower(strings.TrimSpace(payload.Status))
	if status != "pending" && status != "rejected" {
		status = "approved"
	}
	row := Listing{
		ID:               "dlst_" + randomID(10),
		Name:             name,
		CategoryIDs:      categoryIDs,
		Phone:            phone,
		WhatsApp:         firstNonEmpty(strings.TrimSpace(payload.WhatsApp), phone),
		Address:          strings.TrimSpace(payload.Address),
		City:             firstNonEmpty(strings.TrimSpace(payload.City), "Wazirabad"),
		Note:             strings.TrimSpace(payload.Note),
		PhotoFileID:      strings.TrimSpace(payload.PhotoFileID),
		Status:           status,
		ReviewedByUserID: actor.ID,
		ReviewedAt:       now,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	s.listings = append([]Listing{row}, s.listings...)
	s.record(actor, "directory.listing_created", row.ID, "Directory listing created by admin.")
	return row, nil
}

func (s *Service) ReviewListing(id string, payload ReviewPayload, actor auth.User) (Listing, error) {
	if !auth.HasPermission(actor, "directory.manage") {
		return Listing{}, errForbidden
	}
	status := strings.ToLower(strings.TrimSpace(payload.Status))
	if status != "approved" && status != "rejected" {
		return Listing{}, errInvalidListing
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.persistLocked()

	for index := range s.listings {
		if s.listings[index].ID != id {
			continue
		}
		now := time.Now().UTC().Format(time.RFC3339)
		s.listings[index].Status = status
		s.listings[index].ReviewedByUserID = actor.ID
		s.listings[index].ReviewedAt = now
		s.listings[index].ReviewNote = strings.TrimSpace(payload.Note)
		s.listings[index].UpdatedAt = now
		s.record(actor, "directory.listing_reviewed", id, "Listing marked "+status+".")
		return s.listings[index], nil
	}
	return Listing{}, errNotFound
}

func Register(mux *http.ServeMux, service *Service, authService *auth.Service) {
	mux.HandleFunc("/api/directory/categories", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			httpx.Write(w, r, http.StatusOK, map[string]any{"categories": service.ListActiveCategories()})
			return
		}
		httpx.Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
	})

	mux.HandleFunc("/api/directory/listings", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			query := r.URL.Query().Get("q")
			categoryID := r.URL.Query().Get("category")
			rows, page := paginateListings(r, service.ListApprovedListings(query, categoryID))
			httpx.Write(w, r, http.StatusOK, map[string]any{
				"listings":   rows,
				"categories": service.ListActiveCategories(),
				"pagination": page,
			})
		case http.MethodPost:
			user, ok := authService.UserFromRequest(r)
			if !ok {
				httpx.Error(w, r, http.StatusUnauthorized, "not_authenticated", "Sign in is required.")
				return
			}
			var payload ListingPayload
			if !httpx.DecodeJSON(w, r, &payload) {
				return
			}
			listing, err := service.SubmitListing(payload, user)
			writeErr(w, r, listing, err)
		default:
			httpx.Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
		}
	})

	mux.HandleFunc("/api/directory/listings/mine", func(w http.ResponseWriter, r *http.Request) {
		if !httpx.RequireMethod(w, r, http.MethodGet) {
			return
		}
		user, ok := authService.UserFromRequest(r)
		if !ok {
			httpx.Error(w, r, http.StatusUnauthorized, "not_authenticated", "Sign in is required.")
			return
		}
		rows, err := service.ListMine(user)
		paged, page := paginateListings(r, rows)
		writeErr(w, r, map[string]any{"listings": paged, "pagination": page}, err)
	})

	mux.HandleFunc("/api/public/directory/categories", func(w http.ResponseWriter, r *http.Request) {
		if !httpx.RequireMethod(w, r, http.MethodGet) {
			return
		}
		httpx.Write(w, r, http.StatusOK, map[string]any{"categories": service.ListActiveCategories()})
	})

	mux.HandleFunc("/api/public/directory/listings", func(w http.ResponseWriter, r *http.Request) {
		if !httpx.RequireMethod(w, r, http.MethodGet) {
			return
		}
		rows, page := paginateListings(r, service.ListApprovedListings(r.URL.Query().Get("q"), r.URL.Query().Get("category")))
		httpx.Write(w, r, http.StatusOK, map[string]any{
			"listings":   rows,
			"categories": service.ListActiveCategories(),
			"pagination": page,
		})
	})

	mux.HandleFunc("/api/admin/directory/categories", func(w http.ResponseWriter, r *http.Request) {
		user, ok := authService.UserFromRequest(r)
		if !ok {
			httpx.Error(w, r, http.StatusUnauthorized, "not_authenticated", "Sign in is required.")
			return
		}
		switch r.Method {
		case http.MethodGet:
			rows, err := service.ListAllCategories(user)
			writeErr(w, r, map[string]any{"categories": rows}, err)
		case http.MethodPost:
			var payload CategoryPayload
			if !httpx.DecodeJSON(w, r, &payload) {
				return
			}
			row, err := service.CreateCategory(payload, user)
			writeErr(w, r, map[string]any{"category": row}, err)
		default:
			httpx.Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
		}
	})

	mux.HandleFunc("/api/admin/directory/categories/", func(w http.ResponseWriter, r *http.Request) {
		if !httpx.RequireMethod(w, r, http.MethodPost) {
			return
		}
		user, ok := authService.UserFromRequest(r)
		if !ok {
			httpx.Error(w, r, http.StatusUnauthorized, "not_authenticated", "Sign in is required.")
			return
		}
		id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/admin/directory/categories/"), "/")
		if id == "" || strings.Contains(id, "/") {
			httpx.Error(w, r, http.StatusNotFound, "not_found", "Category was not found.")
			return
		}
		var payload CategoryPayload
		if !httpx.DecodeJSON(w, r, &payload) {
			return
		}
		row, err := service.UpdateCategory(id, payload, user)
		writeErr(w, r, map[string]any{"category": row}, err)
	})

	mux.HandleFunc("/api/admin/directory/listings", func(w http.ResponseWriter, r *http.Request) {
		user, ok := authService.UserFromRequest(r)
		if !ok {
			httpx.Error(w, r, http.StatusUnauthorized, "not_authenticated", "Sign in is required.")
			return
		}
		switch r.Method {
		case http.MethodGet:
			rows, err := service.ListAdminListings(user, r.URL.Query().Get("status"), r.URL.Query().Get("q"), r.URL.Query().Get("category"))
			cats, _ := service.ListAllCategories(user)
			paged, page := paginateListings(r, rows)
			writeErr(w, r, map[string]any{"listings": paged, "categories": cats, "pagination": page}, err)
		case http.MethodPost:
			var payload ListingPayload
			if !httpx.DecodeJSON(w, r, &payload) {
				return
			}
			row, err := service.AdminCreateListing(payload, user)
			writeErr(w, r, map[string]any{"listing": row}, err)
		default:
			httpx.Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
		}
	})

	mux.HandleFunc("/api/admin/directory/listings/", func(w http.ResponseWriter, r *http.Request) {
		if !httpx.RequireMethod(w, r, http.MethodPost) {
			return
		}
		user, ok := authService.UserFromRequest(r)
		if !ok {
			httpx.Error(w, r, http.StatusUnauthorized, "not_authenticated", "Sign in is required.")
			return
		}
		path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/admin/directory/listings/"), "/")
		parts := strings.Split(path, "/")
		if len(parts) != 2 || parts[1] != "review" {
			httpx.Error(w, r, http.StatusNotFound, "not_found", "Listing route was not found.")
			return
		}
		var payload ReviewPayload
		if !httpx.DecodeJSON(w, r, &payload) {
			return
		}
		row, err := service.ReviewListing(parts[0], payload, user)
		writeErr(w, r, map[string]any{"listing": row}, err)
	})
}

func writeErr(w http.ResponseWriter, r *http.Request, data any, err error) {
	if err == nil {
		httpx.Write(w, r, http.StatusOK, data)
		return
	}
	switch {
	case errors.Is(err, errForbidden):
		httpx.Error(w, r, http.StatusForbidden, "forbidden", "You do not have access.")
	case errors.Is(err, errNotFound):
		httpx.Error(w, r, http.StatusNotFound, "not_found", "Item was not found.")
	case errors.Is(err, errValidation), errors.Is(err, errInvalidCategory), errors.Is(err, errInvalidListing):
		httpx.Error(w, r, http.StatusBadRequest, "invalid_request", "Check the directory details and try again.")
	default:
		httpx.Error(w, r, http.StatusBadRequest, "invalid_request", err.Error())
	}
}

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "category"
	}
	return out
}

func randomID(n int) string {
	raw := make([]byte, n)
	_, _ = rand.Read(raw)
	return hex.EncodeToString(raw)[:n]
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
