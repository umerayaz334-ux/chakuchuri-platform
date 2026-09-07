package email

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"chakuchuri/backend/internal/auth"
	"chakuchuri/backend/internal/platform/httpx"
)

func Register(mux *http.ServeMux, service *Service, authService *auth.Service) {
	mux.HandleFunc("/api/email", func(w http.ResponseWriter, r *http.Request) {
		if !httpx.RequireMethod(w, r, http.MethodGet) {
			return
		}
		user, ok := requireUser(w, r, authService, "email.read", "email.manage")
		if !ok {
			return
		}
		_ = user
		httpx.Write(w, r, http.StatusOK, paginateEmailSnapshot(r, service.Snapshot()))
	})

	mux.HandleFunc("/api/email/preview", func(w http.ResponseWriter, r *http.Request) {
		if !httpx.RequireMethod(w, r, http.MethodPost) {
			return
		}
		if _, ok := requireUser(w, r, authService, "email.read", "email.manage"); !ok {
			return
		}
		var payload PreviewRequest
		if !httpx.DecodeJSON(w, r, &payload) {
			return
		}
		result, err := service.Preview(payload)
		writeResult(w, r, result, err)
	})

	mux.HandleFunc("/api/email/test", func(w http.ResponseWriter, r *http.Request) {
		if !httpx.RequireMethod(w, r, http.MethodPost) {
			return
		}
		user, ok := requireUser(w, r, authService, "email.manage")
		if !ok {
			return
		}
		var payload TestRequest
		if !httpx.DecodeJSON(w, r, &payload) {
			return
		}
		result, err := service.SendTest(r.Context(), payload, user)
		writeResult(w, r, result, err)
	})

	mux.HandleFunc("/api/email/settings", func(w http.ResponseWriter, r *http.Request) {
		if !httpx.RequireMethod(w, r, http.MethodPost) {
			return
		}
		user, ok := requireUser(w, r, authService, "email.manage")
		if !ok {
			return
		}
		var payload SettingsUpdate
		if !httpx.DecodeJSON(w, r, &payload) {
			return
		}
		result, err := service.UpdateSettings(payload, user)
		writeResult(w, r, result, err)
	})

	mux.HandleFunc("/api/email/settings/verify", func(w http.ResponseWriter, r *http.Request) {
		if !httpx.RequireMethod(w, r, http.MethodPost) {
			return
		}
		user, ok := requireUser(w, r, authService, "email.manage")
		if !ok {
			return
		}
		result, err := service.VerifyConnection(r.Context(), user)
		writeResult(w, r, result, err)
	})

	mux.HandleFunc("/api/email/templates/", func(w http.ResponseWriter, r *http.Request) {
		if !httpx.RequireMethod(w, r, http.MethodPost) {
			return
		}
		user, ok := requireUser(w, r, authService, "email.manage")
		if !ok {
			return
		}
		id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/email/templates/"), "/")
		var payload TemplateUpdate
		if id == "" || !httpx.DecodeJSON(w, r, &payload) {
			return
		}
		result, err := service.UpdateTemplate(id, payload, user)
		writeResult(w, r, result, err)
	})

	mux.HandleFunc("/api/email/automations/", func(w http.ResponseWriter, r *http.Request) {
		if !httpx.RequireMethod(w, r, http.MethodPost) {
			return
		}
		user, ok := requireUser(w, r, authService, "email.manage")
		if !ok {
			return
		}
		id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/email/automations/"), "/")
		var payload RuleUpdate
		if id == "" || !httpx.DecodeJSON(w, r, &payload) {
			return
		}
		result, err := service.UpdateRule(id, payload, user)
		writeResult(w, r, result, err)
	})

	mux.HandleFunc("/api/email/outbox/", func(w http.ResponseWriter, r *http.Request) {
		if !httpx.RequireMethod(w, r, http.MethodPost) {
			return
		}
		user, ok := requireUser(w, r, authService, "email.manage")
		if !ok {
			return
		}
		path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/email/outbox/"), "/")
		if !strings.HasSuffix(path, "/cancel") {
			httpx.Error(w, r, http.StatusNotFound, "not_found", "Email action was not found.")
			return
		}
		id := strings.TrimSuffix(path, "/cancel")
		result, err := service.CancelOutbox(id, user)
		writeResult(w, r, result, err)
	})

	mux.HandleFunc("/api/email/deliveries/", func(w http.ResponseWriter, r *http.Request) {
		if !httpx.RequireMethod(w, r, http.MethodPost) {
			return
		}
		user, ok := requireUser(w, r, authService, "email.manage")
		if !ok {
			return
		}
		path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/email/deliveries/"), "/")
		if !strings.HasSuffix(path, "/retry") {
			httpx.Error(w, r, http.StatusNotFound, "not_found", "Email action was not found.")
			return
		}
		id := strings.TrimSuffix(path, "/retry")
		result, err := service.RetryDelivery(id, user)
		writeResult(w, r, result, err)
	})
}

func requireUser(w http.ResponseWriter, r *http.Request, authService *auth.Service, permissions ...string) (auth.User, bool) {
	user, ok := authService.UserFromRequest(r)
	if !ok {
		httpx.Error(w, r, http.StatusUnauthorized, "not_authenticated", "Sign in is required.")
		return auth.User{}, false
	}
	if !auth.HasAnyPermission(user, permissions...) {
		httpx.Error(w, r, http.StatusForbidden, "forbidden", "You do not have access to email operations.")
		return auth.User{}, false
	}
	return user, true
}

func writeResult(w http.ResponseWriter, r *http.Request, result interface{}, err error) {
	if err == nil {
		if snapshot, ok := result.(Snapshot); ok {
			result = paginateEmailSnapshot(r, snapshot)
		}
		httpx.Write(w, r, http.StatusOK, result)
		return
	}
	if errors.Is(err, errNotFound) {
		httpx.Error(w, r, http.StatusNotFound, "not_found", "Email record was not found.")
		return
	}
	if errors.Is(err, errDuplicate) {
		httpx.Error(w, r, http.StatusConflict, "duplicate_event", "This email event is already queued.")
		return
	}
	message := "Please check the email details and try again."
	if errors.Is(err, errValidation) {
		raw := strings.TrimSpace(err.Error())
		prefix := errValidation.Error() + ": "
		if strings.HasPrefix(raw, prefix) {
			message = strings.TrimSpace(strings.TrimPrefix(raw, prefix))
		}
	} else if text := strings.TrimSpace(err.Error()); text != "" {
		message = text
		if len(message) > 240 {
			message = message[:240]
		}
	}
	httpx.Error(w, r, http.StatusBadRequest, "validation_failed", message)
}

func paginateEmailSnapshot(r *http.Request, snapshot Snapshot) Snapshot {
	scope := strings.TrimSpace(r.URL.Query().Get("scope"))
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

	snapshot.Pagination = map[string]PageInfo{}
	if scope == "deliveries" {
		snapshot.Deliveries, snapshot.Pagination["deliveries"] = emailPage(snapshot.Deliveries, offset, limit)
		snapshot.Pagination["outbox"] = PageInfo{Total: len(snapshot.Outbox), HasMore: len(snapshot.Outbox) > 0}
		snapshot.Outbox = []OutboxItem{}
		return snapshot
	}
	if scope == "outbox" {
		snapshot.Outbox, snapshot.Pagination["outbox"] = emailPage(snapshot.Outbox, offset, limit)
		snapshot.Pagination["deliveries"] = PageInfo{Total: len(snapshot.Deliveries), HasMore: len(snapshot.Deliveries) > 0}
		snapshot.Deliveries = []Delivery{}
		return snapshot
	}

	snapshot.Outbox, snapshot.Pagination["outbox"] = emailPage(snapshot.Outbox, 0, limit)
	snapshot.Deliveries, snapshot.Pagination["deliveries"] = emailPage(snapshot.Deliveries, 0, limit)
	return snapshot
}

func emailPage[T any](rows []T, offset, limit int) ([]T, PageInfo) {
	if offset > len(rows) {
		offset = len(rows)
	}
	end := offset + limit
	if end > len(rows) {
		end = len(rows)
	}
	return append([]T(nil), rows[offset:end]...), PageInfo{Loaded: end, Total: len(rows), HasMore: end < len(rows)}
}
