package httpx

import (
	"encoding/json"
	"log"
	"net/http"
	"runtime/debug"
	"strings"
	"time"
)

type APIResponse struct {
	OK        bool        `json:"ok"`
	RequestID string      `json:"requestId"`
	Data      interface{} `json:"data,omitempty"`
	Error     *APIError   `json:"error,omitempty"`
}

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func RequestID(r *http.Request) string {
	if requestID := strings.TrimSpace(r.Header.Get("X-Request-ID")); requestID != "" {
		return requestID
	}
	return time.Now().UTC().Format("20060102150405.000000000")
}

func Write(w http.ResponseWriter, r *http.Request, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(APIResponse{
		OK:        true,
		RequestID: RequestID(r),
		Data:      data,
	})
}

func Error(w http.ResponseWriter, r *http.Request, status int, code string, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(APIResponse{
		OK:        false,
		RequestID: RequestID(r),
		Error: &APIError{
			Code:    code,
			Message: message,
		},
	})
}

func RequireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method == method {
		return true
	}
	Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "This endpoint does not allow "+r.Method+".")
	return false
}

func DecodeJSON(w http.ResponseWriter, r *http.Request, target interface{}) bool {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		Error(w, r, http.StatusBadRequest, "invalid_json", "Request body is not valid JSON.")
		return false
	}
	return true
}

func WithCORS(origin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func WithRequestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s request_id=%s duration=%s", r.Method, r.URL.Path, RequestID(r), time.Since(start).Truncate(time.Millisecond))
	})
}

func WithRecover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Printf("panic request_id=%s error=%v stack=%s", RequestID(r), recovered, string(debug.Stack()))
				Error(w, r, http.StatusInternalServerError, "internal_error", "Something went wrong. The error was logged with this request ID.")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
