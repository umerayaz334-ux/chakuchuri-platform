package onboarding

import (
	"errors"
	"net/http"
	"strings"

	"chakuchuri/backend/internal/audit"
	"chakuchuri/backend/internal/auth"
	"chakuchuri/backend/internal/customers"
	"chakuchuri/backend/internal/platform/httpx"
	"chakuchuri/backend/internal/platform/idgen"
)

type SignupRequest struct {
	CompanyName string   `json:"companyName"`
	ContactName string   `json:"contactName"`
	Email       string   `json:"email"`
	Phone       string   `json:"phone"`
	Country     string   `json:"country"`
	Password    string   `json:"password"`
	Services    []string `json:"services"`
}

func Register(mux *http.ServeMux, authService *auth.Service, customerService *customers.Service, recorder *audit.Recorder) {
	mux.HandleFunc("/api/auth/signup", func(w http.ResponseWriter, r *http.Request) {
		if !httpx.RequireMethod(w, r, http.MethodPost) {
			return
		}

		var payload SignupRequest
		if !httpx.DecodeJSON(w, r, &payload) {
			return
		}
		payload.CompanyName = strings.TrimSpace(payload.CompanyName)
		payload.ContactName = strings.TrimSpace(payload.ContactName)
		payload.Email = strings.ToLower(strings.TrimSpace(payload.Email))
		payload.Password = strings.TrimSpace(payload.Password)
		if payload.CompanyName == "" || payload.Email == "" || len(payload.Password) < 6 {
			httpx.Error(w, r, http.StatusBadRequest, "invalid_signup", "Company name, email and a 6 character password are required.")
			return
		}

		customerID := idgen.New("cust_signup")
		customer := customerService.Create(customers.CreateRequest{
			ID:          customerID,
			TenantID:    "tenant_chakuchuri",
			CompanyName: payload.CompanyName,
			ContactName: payload.ContactName,
			Email:       payload.Email,
			Phone:       payload.Phone,
			Country:     payload.Country,
			Services:    payload.Services,
		})

		_, err := authService.CreateUser(auth.User{
			ID:         idgen.New("usr_signup"),
			TenantID:   "tenant_chakuchuri",
			CustomerID: customer.ID,
			Name:       payload.CompanyName,
			Email:      payload.Email,
			Role:       "Customer",
			Permissions: []string{
				"quotes.create",
				"orders.read",
				"shipping.create",
				"payments.create",
				"chat.use",
				"calls.start",
			},
		}, payload.Password)
		if errors.Is(err, auth.ErrEmailExists) {
			httpx.Error(w, r, http.StatusConflict, "email_exists", "An account with this email already exists.")
			return
		}
		if err != nil {
			httpx.Error(w, r, http.StatusBadRequest, "signup_failed", "Customer account could not be created.")
			return
		}

		session, err := authService.StartSession(payload.Email)
		if err != nil {
			httpx.Error(w, r, http.StatusUnauthorized, "signup_login_failed", "Account was created but sign in failed.")
			return
		}
		recorder.Record(payload.Email, "onboarding.signup", "customer", "Customer portal signup completed.")
		httpx.Write(w, r, http.StatusCreated, map[string]interface{}{
			"customer": customer,
			"session":  session,
		})
	})
}
