package foundation

import (
	"net/http"

	"chakuchuri/backend/internal/platform/httpx"
)

type Module struct {
	Name      string   `json:"name"`
	Owner     string   `json:"owner"`
	Purpose   string   `json:"purpose"`
	Status    string   `json:"status"`
	DataRules []string `json:"dataRules"`
}

type Foundation struct {
	Phase      string   `json:"phase"`
	Principles []string `json:"principles"`
	Modules    []Module `json:"modules"`
}

func Current() Foundation {
	return Foundation{
		Phase: "Phase 4: persistence and database migration safety",
		Principles: []string{
			"Every module owns its data and API routes.",
			"Every important change writes an audit event.",
			"All APIs return one response and error shape with request IDs.",
			"Database changes are only made through migrations.",
			"Customer files are stored by tenant/customer and processed in background jobs.",
		},
		Modules: []Module{
			{Name: "Auth", Owner: "internal/auth", Purpose: "Login, sessions, roles and permissions.", Status: "Started", DataRules: []string{"No plaintext passwords", "Short lived sessions", "Role based permissions"}},
			{Name: "Customers", Owner: "internal/customers", Purpose: "Customer accounts, service access and online presence.", Status: "Started", DataRules: []string{"Tenant scoped", "Soft delete", "Account ledger link"}},
			{Name: "Quotations", Owner: "internal/workflow", Purpose: "Quote requests, pricing and customer approvals.", Status: "Started", DataRules: []string{"Versioned pricing", "Image attachments", "Audit on accept/reject"}},
			{Name: "Manufacturing", Owner: "internal/workflow", Purpose: "Production orders, timeline and expected completion.", Status: "Started", DataRules: []string{"Stage history", "Estimated date", "Payment gate"}},
			{Name: "Shipping", Owner: "internal/workflow", Purpose: "Courier rates, requests, labels and delivery statuses.", Status: "Started", DataRules: []string{"Rate sheet versions", "External product allowed", "Tracking history"}},
			{Name: "Payments", Owner: "internal/workflow", Purpose: "Proof uploads, confirmations and customer balance.", Status: "Started", DataRules: []string{"Double entry ledger", "Admin confirmation", "Immutable posted entries"}},
			{Name: "Database Migrations", Owner: "internal/platform/migrations", Purpose: "Apply, rollback and inspect PostgreSQL schema versions.", Status: "Started", DataRules: []string{"Versioned SQL files", "Reversible changes", "schema_migrations ledger"}},
			{Name: "Chat & Calls", Owner: "internal/realtime", Purpose: "Messages, attachments, online/last online and direct audio calls.", Status: "Designed", DataRules: []string{"Presence events", "Conversation history", "Call history"}},
			{Name: "Files", Owner: "internal/files", Purpose: "Image compression, thumbnails, documents and receipts.", Status: "Designed", DataRules: []string{"Original retained", "Compressed preview", "Malware/type checks"}},
		},
	}
}

func Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/foundation", func(w http.ResponseWriter, r *http.Request) {
		if !httpx.RequireMethod(w, r, http.MethodGet) {
			return
		}
		httpx.Write(w, r, http.StatusOK, Current())
	})
}
