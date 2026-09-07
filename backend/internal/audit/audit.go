package audit

import (
	"net/http"
	"sync"
	"time"

	"chakuchuri/backend/internal/platform/httpx"
)

type Event struct {
	ID        string `json:"id"`
	Actor     string `json:"actor"`
	Action    string `json:"action"`
	Entity    string `json:"entity"`
	Detail    string `json:"detail"`
	CreatedAt string `json:"createdAt"`
}

type Recorder struct {
	mu     sync.RWMutex
	events []Event
}

type AccessCheck func(*http.Request) (int, string, string, bool)

func NewRecorder() *Recorder {
	recorder := &Recorder{}
	recorder.Record("system", "foundation.boot", "platform", "Audit recorder initialized.")
	return recorder
}

func (r *Recorder) Record(actor string, action string, entity string, detail string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append([]Event{{
		ID:        time.Now().UTC().Format("20060102150405.000000000"),
		Actor:     actor,
		Action:    action,
		Entity:    entity,
		Detail:    detail,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}}, r.events...)
	if len(r.events) > 200 {
		r.events = r.events[:200]
	}
}

func (r *Recorder) Recent(limit int) []Event {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if limit <= 0 || limit > len(r.events) {
		limit = len(r.events)
	}
	copyEvents := make([]Event, limit)
	copy(copyEvents, r.events[:limit])
	return copyEvents
}

func Register(mux *http.ServeMux, recorder *Recorder, access AccessCheck) {
	mux.HandleFunc("/api/audit/recent", func(w http.ResponseWriter, r *http.Request) {
		if !httpx.RequireMethod(w, r, http.MethodGet) {
			return
		}
		if access != nil {
			status, code, message, ok := access(r)
			if !ok {
				httpx.Error(w, r, status, code, message)
				return
			}
		}
		httpx.Write(w, r, http.StatusOK, map[string]interface{}{
			"events": recorder.Recent(25),
		})
	})
}
