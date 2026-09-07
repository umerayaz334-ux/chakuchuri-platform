package workflow

import (
	"encoding/csv"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"chakuchuri/backend/internal/auth"
	"chakuchuri/backend/internal/platform/httpx"
)

type StatementEntry struct {
	LedgerEntry
	Balance int `json:"balance"`
}

type AccountingPage struct {
	Summary    AccountingSummary `json:"summary"`
	Ledger     []StatementEntry  `json:"ledger"`
	Payments   []Payment         `json:"payments"`
	Pagination PageInfo          `json:"pagination"`
	Issues     []AccountingIssue `json:"issues"`
}

func (s *Service) accountingRows(user auth.User, customerID, from, query string) (AccountingPage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	all, allowed := auth.CustomerScope(user)
	if customerID != "" && customerID != "all" {
		if !auth.CanAccessCustomer(user, customerID) {
			return AccountingPage{}, errForbidden
		}
		all, allowed = false, map[string]bool{customerID: true}
	}
	if from != "" {
		if _, err := time.Parse("2006-01-02", from); err != nil {
			return AccountingPage{}, errValidation
		}
	}
	ledger := filterLedger(s.ledger, all, allowed)
	payments := filterPayments(s.payments, all, allowed)
	result := AccountingPage{Summary: accountingSummary(ledger, payments), Ledger: []StatementEntry{}, Payments: []Payment{}, Issues: []AccountingIssue{}}
	query = strings.ToLower(strings.TrimSpace(query))
	match := func(text string) bool { return query == "" || strings.Contains(strings.ToLower(text), query) }
	sort.Slice(ledger, func(i, j int) bool {
		if ledger[i].PostedAt == ledger[j].PostedAt {
			return ledger[i].ID < ledger[j].ID
		}
		return ledger[i].PostedAt < ledger[j].PostedAt
	})
	balances := map[string]int{}
	customers := map[string]bool{}
	for _, l := range ledger {
		customers[l.CustomerID] = true
		balances[l.CustomerID] += l.Debit - l.Credit
		if (from == "" || l.PostedAt >= from) && match(l.EntryNo+" "+l.SourceID+" "+l.Note+" "+l.CustomerID) {
			result.Ledger = append(result.Ledger, StatementEntry{l, balances[l.CustomerID]})
		}
	}
	for i, j := 0, len(result.Ledger)-1; i < j; i, j = i+1, j-1 {
		result.Ledger[i], result.Ledger[j] = result.Ledger[j], result.Ledger[i]
	}
	for _, p := range payments {
		customers[p.CustomerID] = true
		if (from == "" || p.CreatedAt >= from) && match(p.ID+" "+p.Type+" "+p.Note+" "+p.Status+" "+p.ProofName+" "+p.CustomerID) {
			result.Payments = append(result.Payments, p)
		}
	}
	sort.Slice(result.Payments, func(i, j int) bool {
		a, b := result.Payments[i], result.Payments[j]
		if a.CreatedAt == b.CreatedAt {
			return a.ID > b.ID
		}
		return a.CreatedAt > b.CreatedAt
	})
	for customer := range customers {
		result.Issues = append(result.Issues, s.accountingIssuesLocked(customer)...)
	}
	return result, nil
}

func (s *Service) registerAccounting(mux *http.ServeMux) {
	mux.HandleFunc("/api/accounting/", func(w http.ResponseWriter, r *http.Request) {
		user, ok := s.authService.UserFromRequest(r)
		if !ok {
			httpx.Error(w, r, http.StatusUnauthorized, "not_authenticated", "Sign in is required.")
			return
		}
		if !auth.IsCustomerRole(user) && !auth.HasAnyPermission(user, "payments.manage", "payments.read", "customers.manage") {
			httpx.Error(w, r, http.StatusForbidden, "forbidden", "Payment access is required.")
			return
		}
		w.Header().Set("Cache-Control", "private, no-store")
		if r.URL.Path == "/api/accounting/reconcile" {
			if !httpx.RequireMethod(w, r, http.MethodPost) {
				return
			}
			var payload struct {
				CustomerID string `json:"customerId"`
				Digest     string `json:"digest"`
				Reason     string `json:"reason"`
			}
			if !httpx.DecodeJSON(w, r, &payload) {
				return
			}
			plan, err := s.ReconcileAccounting(payload.CustomerID, payload.Digest, payload.Reason, user)
			writeResult(w, r, plan, err)
			return
		}
		if !httpx.RequireMethod(w, r, http.MethodGet) {
			return
		}
		q := r.URL.Query()
		page, err := s.accountingRows(user, q.Get("customerId"), q.Get("from"), q.Get("search"))
		if err != nil {
			writeResult(w, r, nil, err)
			return
		}
		switch r.URL.Path {
		case "/api/accounting/metrics":
			a := page.Summary
			httpx.Write(w, r, http.StatusOK, map[string]interface{}{"balanceDue": a.Due, "ledgerBalance": a.Balance, "accountCredit": a.Credit, "totalCharges": a.Charges, "totalAdjustments": a.Adjustments, "totalReceived": a.Received, "pendingPaymentAmount": a.Pending})
		case "/api/accounting/statement.csv":
			w.Header().Set("Content-Type", "text/csv; charset=utf-8")
			w.Header().Set("Content-Disposition", `attachment; filename="statement.csv"`)
			writer := csv.NewWriter(w)
			_ = writer.Write([]string{"Date", "Customer ID", "Entry", "Source", "Description", "Debit", "Credit", "Balance", "Currency"})
			for _, l := range page.Ledger {
				_ = writer.Write([]string{csvSafe(l.PostedAt), csvSafe(l.CustomerID), csvSafe(l.EntryNo), csvSafe(l.SourceID), csvSafe(l.Note), strconv.Itoa(l.Debit), strconv.Itoa(l.Credit), strconv.Itoa(l.Balance), csvSafe(l.Currency)})
			}
			writer.Flush()
		case "/api/accounting/page":
			offset, _ := strconv.Atoi(q.Get("offset"))
			limit, _ := strconv.Atoi(q.Get("limit"))
			if q.Get("scope") == "payments" {
				page.Payments, page.Pagination = pagedSlice(page.Payments, offset, limit)
				page.Ledger = []StatementEntry{}
			} else {
				page.Ledger, page.Pagination = pagedSlice(page.Ledger, offset, limit)
				page.Payments = []Payment{}
			}
			httpx.Write(w, r, http.StatusOK, page)
		default:
			httpx.Error(w, r, http.StatusNotFound, "not_found", "Accounting endpoint not found.")
		}
	})
}

func csvSafe(value string) string {
	if strings.ContainsAny(value[:min(len(value), 1)], "=+-@\t\r") {
		return "'" + value
	}
	return value
}
