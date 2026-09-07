package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"chakuchuri/backend/internal/audit"
	"chakuchuri/backend/internal/platform/httpx"
)

type User struct {
	ID                  string          `json:"id"`
	TenantID            string          `json:"tenantId"`
	CustomerID          string          `json:"customerId,omitempty"`
	AssignedCustomerIDs []string        `json:"assignedCustomerIds,omitempty"`
	Name                string          `json:"name"`
	Email               string          `json:"email"`
	Role                string          `json:"role"`
	Status              string          `json:"status"`
	Permissions         []string        `json:"permissions"`
	PageAccess          []string        `json:"pageAccess"`
	LastLogin           string          `json:"lastLogin"`
	LastOnline          string          `json:"lastOnline"`
	FailedLoginCount    int             `json:"failedLoginCount"`
	LoginActivity       []LoginActivity `json:"loginActivity,omitempty"`
	KnownIPs            []KnownIP       `json:"knownIps,omitempty"`
	ProfileImageFileID  string          `json:"profileImageFileId,omitempty"`
	SuspensionNotice    string          `json:"suspensionNotice,omitempty"`
	SuspensionContactAt string          `json:"suspensionContactAt,omitempty"`
	SuspensionMessage   string          `json:"suspensionContactMessage,omitempty"`
	NotificationReadIDs []string        `json:"notificationReadIds,omitempty"`
}

type Session struct {
	Token     string `json:"token"`
	User      User   `json:"user"`
	ExpiresAt string `json:"expiresAt"`
}

type Service struct {
	mu                  sync.RWMutex
	recorder            *audit.Recorder
	repository          Repository
	customerProvisioner func(User)
	changeNotifier      func(string)
	presenceNotifier    func(UserPresence)
	users               map[string]storedUser
	sessions            map[string]sessionRecord
	lastPresencePublish map[string]time.Time
	forcedOffline       map[string]time.Time
}

type storedUser struct {
	User
	salt         string
	passwordHash string
}

type sessionRecord struct {
	UserEmail string
	ExpiresAt time.Time
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

var ErrInvalidCredentials = errors.New("invalid credentials")
var ErrEmailExists = errors.New("email already exists")

func (s *Service) SetCustomerProvisioner(provisioner func(User)) {
	s.customerProvisioner = provisioner
}

func (s *Service) SetChangeNotifier(notifier func(string)) {
	s.changeNotifier = notifier
}

func (s *Service) SetPresenceNotifier(notifier func(UserPresence)) {
	s.presenceNotifier = notifier
}

func (s *Service) notifyChange(scope string) {
	if s.changeNotifier != nil {
		s.changeNotifier(scope)
	}
}
func NewService(recorder *audit.Recorder) *Service {
	service := &Service{
		recorder:            recorder,
		users:               map[string]storedUser{},
		sessions:            map[string]sessionRecord{},
		lastPresencePublish: map[string]time.Time{},
		forcedOffline:       map[string]time.Time{},
	}
	service.seedUser(User{
		ID:       "usr_admin_001",
		TenantID: "tenant_chakuchuri",
		Name:     "ChakuChuri Admin",
		Email:    "admin@chakuchuri.pk",
		Role:     "Owner",
		Permissions: []string{
			"customers.manage",
			"quotes.manage",
			"manufacturing.manage",
			"shipping.manage",
			"payments.manage",
			"chat.manage",
			"calls.manage",
			"audit.read",
		},
	}, "admin123")
	service.seedUser(User{
		ID:         "usr_customer_001",
		TenantID:   "tenant_chakuchuri",
		CustomerID: "cust_abc_export",
		Name:       "ABC Export House",
		Email:      "customer@chakuchuri.pk",
		Role:       "Customer",
		Permissions: []string{
			"quotes.create",
			"orders.read",
			"shipping.create",
			"payments.create",
			"chat.use",
			"calls.start",
		},
	}, "customer123")
	return service
}

func (s *Service) Login(email string, password string, r *http.Request) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := strings.ToLower(strings.TrimSpace(email))
	user, ok := s.users[key]
	if !ok {
		s.recordSystem("auth.login_failed", "Unknown email attempted sign in.")
		return Session{}, ErrInvalidCredentials
	}

	ip := requestIP(r)
	userAgent := requestUserAgent(r)
	if !canStartSession(user.User) {
		user.User = appendLoginActivity(user.User, "Blocked", ip, userAgent, "User status is "+normalizeStatus(user.Status)+".")
		s.users[key] = user
		s.persistLocked()
		return Session{}, ErrInvalidCredentials
	}
	if user.passwordHash != hashPassword(user.salt, password) {
		user.FailedLoginCount++
		user.User = appendLoginActivity(user.User, "Failed", ip, userAgent, "Invalid password.")
		s.users[key] = user
		s.persistLocked()
		if s.recorder != nil {
			s.recorder.Record(user.Email, "auth.login_failed", "session", "Invalid password.")
		}
		return Session{}, ErrInvalidCredentials
	}

	user.FailedLoginCount = 0
	return s.createSessionLocked(key, user, "auth.login", "User signed in.", r), nil
}

func (s *Service) CreateUser(user User, password string) (User, error) {
	s.mu.Lock()
	key := strings.ToLower(strings.TrimSpace(user.Email))
	if key == "" || strings.TrimSpace(password) == "" {
		s.mu.Unlock()
		return User{}, ErrInvalidCredentials
	}
	if _, exists := s.users[key]; exists {
		s.mu.Unlock()
		return User{}, ErrEmailExists
	}
	user.Email = key
	user = normalizeNewUser(user)
	user.LastOnline = firstNonEmpty(user.LastOnline, "Never")
	user.LastLogin = firstNonEmpty(user.LastLogin, "Never")
	serviceUser := storedUser{
		User:         user,
		salt:         randomToken(),
		passwordHash: "",
	}
	serviceUser.passwordHash = hashPassword(serviceUser.salt, password)
	s.users[key] = serviceUser
	if s.recorder != nil {
		s.recorder.Record(user.Email, "auth.user_created", "user", "Portal user created.")
	}
	s.persistLocked()
	s.mu.Unlock()
	s.notifyChange("users")
	return user, nil
}

func (s *Service) StartSession(email string) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := strings.ToLower(strings.TrimSpace(email))
	user, ok := s.users[key]
	if !ok {
		return Session{}, ErrInvalidCredentials
	}
	return s.createSessionLocked(key, user, "auth.signup_login", "New customer signed in after signup.", nil), nil
}

func (s *Service) UserFromRequest(r *http.Request) (User, bool) {
	user, ok := s.SessionUserFromRequest(r)
	if !ok || !isActiveUser(user) {
		return User{}, false
	}
	return user, true
}

// SessionUserFromRequest accepts an active or temporarily suspended session.
// Normal product APIs use UserFromRequest, so suspended users remain locked out.
func (s *Service) SessionUserFromRequest(r *http.Request) (User, bool) {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	token := strings.TrimPrefix(header, "Bearer ")
	if token == header {
		return User{}, false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[token]
	if !ok || time.Now().After(session.ExpiresAt) {
		return User{}, false
	}
	user, ok := s.users[session.UserEmail]
	if !ok || !canStartSession(user.User) {
		return User{}, false
	}
	return normalizeLoadedUser(user.User), true
}

func (s *Service) seedUser(user User, password string) {
	_, _ = s.CreateUser(user, password)
}

func (s *Service) createSessionLocked(key string, user storedUser, action string, detail string, r *http.Request) Session {
	token := randomToken()
	expiresAt := time.Now().Add(12 * time.Hour)
	now := time.Now().UTC().Format(time.RFC3339)
	user.LastOnline = now
	user.LastLogin = now
	user.User = appendLoginActivity(user.User, "Success", requestIP(r), requestUserAgent(r), detail)
	s.users[key] = user
	s.sessions[token] = sessionRecord{UserEmail: key, ExpiresAt: expiresAt}
	if s.recorder != nil {
		s.recorder.Record(user.Email, action, "session", detail)
	}
	s.persistLocked()

	return Session{
		Token:     token,
		User:      normalizeLoadedUser(user.User),
		ExpiresAt: expiresAt.UTC().Format(time.RFC3339),
	}
}

func hashPassword(salt string, password string) string {
	sum := sha256.Sum256([]byte(salt + ":" + password))
	return hex.EncodeToString(sum[:])
}

func randomToken() string {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return base64.RawURLEncoding.EncodeToString(buffer)
}

func Register(mux *http.ServeMux, service *Service) {
	mux.HandleFunc("/api/auth/login", func(w http.ResponseWriter, r *http.Request) {
		if !httpx.RequireMethod(w, r, http.MethodPost) {
			return
		}
		var payload LoginRequest
		if !httpx.DecodeJSON(w, r, &payload) {
			return
		}
		session, err := service.Login(payload.Email, payload.Password, r)
		if err != nil {
			httpx.Error(w, r, http.StatusUnauthorized, "invalid_credentials", "Email or password is incorrect, or the user is not active.")
			return
		}
		httpx.Write(w, r, http.StatusOK, session)
	})

	mux.HandleFunc("/api/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		if !httpx.RequireMethod(w, r, http.MethodPost) {
			return
		}
		token := bearerToken(r)
		if token == "" {
			httpx.Error(w, r, http.StatusUnauthorized, "not_authenticated", "Sign in is required.")
			return
		}
		service.Logout(token)
		httpx.Write(w, r, http.StatusOK, map[string]string{"status": "signed_out"})
	})

	mux.HandleFunc("/api/auth/presence/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		if !httpx.RequireMethod(w, r, http.MethodPost) {
			return
		}
		user, ok := service.UserFromRequest(r)
		if !ok {
			httpx.Error(w, r, http.StatusUnauthorized, "not_authenticated", "Sign in is required.")
			return
		}
		service.Heartbeat(user.ID)
		httpx.Write(w, r, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/api/auth/me", func(w http.ResponseWriter, r *http.Request) {
		if !httpx.RequireMethod(w, r, http.MethodGet) {
			return
		}
		user, ok := service.SessionUserFromRequest(r)
		if !ok {
			httpx.Error(w, r, http.StatusUnauthorized, "not_authenticated", "Sign in is required.")
			return
		}
		httpx.Write(w, r, http.StatusOK, map[string]User{"user": user})
	})

	mux.HandleFunc("/api/auth/notifications/read", func(w http.ResponseWriter, r *http.Request) {
		if !httpx.RequireMethod(w, r, http.MethodPost) {
			return
		}
		actor, ok := service.UserFromRequest(r)
		if !ok {
			httpx.Error(w, r, http.StatusUnauthorized, "not_authenticated", "Sign in is required.")
			return
		}
		var payload NotificationReadRequest
		if !httpx.DecodeJSON(w, r, &payload) {
			return
		}
		user, err := service.MarkNotificationsRead(actor, payload.IDs)
		writeAuthResult(w, r, map[string]User{"user": user}, err)
	})

	mux.HandleFunc("/api/auth/profile", func(w http.ResponseWriter, r *http.Request) {
		if !httpx.RequireMethod(w, r, http.MethodPost) {
			return
		}
		actor, ok := service.UserFromRequest(r)
		if !ok {
			httpx.Error(w, r, http.StatusUnauthorized, "not_authenticated", "Sign in is required.")
			return
		}
		var payload ProfileUpdateRequest
		if !httpx.DecodeJSON(w, r, &payload) {
			return
		}
		user, err := service.UpdateOwnProfile(actor, payload, bearerToken(r))
		writeAuthResult(w, r, user, err)
	})

	mux.HandleFunc("/api/auth/suspension/contact", func(w http.ResponseWriter, r *http.Request) {
		if !httpx.RequireMethod(w, r, http.MethodPost) {
			return
		}
		actor, ok := service.SessionUserFromRequest(r)
		if !ok {
			httpx.Error(w, r, http.StatusUnauthorized, "not_authenticated", "Sign in is required.")
			return
		}
		var payload SuspensionContactRequest
		if !httpx.DecodeJSON(w, r, &payload) {
			return
		}
		user, err := service.ContactAboutSuspension(actor, payload.Message)
		writeAuthResult(w, r, user, err)
	})
	mux.HandleFunc("/api/auth/users/options", func(w http.ResponseWriter, r *http.Request) {
		if !httpx.RequireMethod(w, r, http.MethodGet) {
			return
		}
		actor, ok := service.UserFromRequest(r)
		if !ok {
			httpx.Error(w, r, http.StatusUnauthorized, "not_authenticated", "Sign in is required.")
			return
		}
		options, err := service.AccessOptions(actor)
		writeAuthResult(w, r, options, err)
	})

	mux.HandleFunc("/api/auth/users", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := service.UserFromRequest(r)
		if !ok {
			httpx.Error(w, r, http.StatusUnauthorized, "not_authenticated", "Sign in is required.")
			return
		}
		switch r.Method {
		case http.MethodGet:
			users, err := service.ListUsers(actor)
			if err != nil {
				writeAuthResult[interface{}](w, r, nil, err)
				return
			}
			query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
			role := strings.TrimSpace(r.URL.Query().Get("role"))
			status := strings.TrimSpace(r.URL.Query().Get("status"))
			filtered := make([]User, 0, len(users))
			for _, user := range users {
				matchesQuery := query == "" || strings.Contains(strings.ToLower(user.Name+" "+user.Email+" "+user.Role+" "+user.Status), query)
				if matchesQuery && (role == "" || role == "All roles" || user.Role == role) && (status == "" || status == "All statuses" || user.Status == status) {
					filtered = append(filtered, user)
				}
			}
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
			if offset > len(filtered) {
				offset = len(filtered)
			}
			end := offset + limit
			if end > len(filtered) {
				end = len(filtered)
			}
			writeAuthResult(w, r, map[string]interface{}{
				"users":      filtered[offset:end],
				"pagination": map[string]interface{}{"loaded": end, "total": len(filtered), "hasMore": end < len(filtered)},
			}, nil)
		case http.MethodPost:
			var payload ManagedUserRequest
			if !httpx.DecodeJSON(w, r, &payload) {
				return
			}
			user, err := service.CreateManagedUser(payload, actor)
			writeAuthResult(w, r, user, err)
		default:
			httpx.Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method is not allowed.")
		}
	})

	mux.HandleFunc("/api/auth/users/", func(w http.ResponseWriter, r *http.Request) {
		if !httpx.RequireMethod(w, r, http.MethodPost) {
			return
		}
		actor, ok := service.UserFromRequest(r)
		if !ok {
			httpx.Error(w, r, http.StatusUnauthorized, "not_authenticated", "Sign in is required.")
			return
		}
		path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/auth/users/"), "/")
		if path == "" {
			httpx.Error(w, r, http.StatusNotFound, "user_not_found", "User was not found.")
			return
		}
		if strings.HasSuffix(path, "/delete") {
			id := strings.TrimSuffix(path, "/delete")
			writeAuthResult(w, r, map[string]string{"deleted": id}, service.DeleteManagedUser(id, actor))
			return
		}
		var payload ManagedUserRequest
		if !httpx.DecodeJSON(w, r, &payload) {
			return
		}
		user, err := service.UpdateManagedUser(path, payload, actor)
		writeAuthResult(w, r, user, err)
	})
}

func writeAuthResult[T any](w http.ResponseWriter, r *http.Request, payload T, err error) {
	if err != nil {
		writeAuthError(w, r, err)
		return
	}
	httpx.Write(w, r, http.StatusOK, payload)
}

func writeAuthError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrForbidden):
		httpx.Error(w, r, http.StatusForbidden, "forbidden", "You do not have access to manage users.")
	case errors.Is(err, ErrUserNotFound):
		httpx.Error(w, r, http.StatusNotFound, "user_not_found", "User was not found.")
	case errors.Is(err, ErrCannotDeleteSelf):
		httpx.Error(w, r, http.StatusBadRequest, "cannot_delete_self", "You cannot delete your own active login.")
	case errors.Is(err, ErrEmailExists):
		httpx.Error(w, r, http.StatusConflict, "email_exists", "A user with this email already exists.")
	case errors.Is(err, ErrInvalidCredentials):
		httpx.Error(w, r, http.StatusBadRequest, "invalid_user", "Name, email and password are required for a new user.")
	case errors.Is(err, ErrCurrentPassword):
		httpx.Error(w, r, http.StatusBadRequest, "current_password_invalid", "Current password is incorrect.")
	case errors.Is(err, ErrWeakPassword):
		httpx.Error(w, r, http.StatusBadRequest, "weak_password", "New password must be at least 8 characters.")
	case errors.Is(err, ErrNotSuspended):
		httpx.Error(w, r, http.StatusForbidden, "not_suspended", "This account is not temporarily suspended.")
	default:
		httpx.Error(w, r, http.StatusBadRequest, "auth_error", "User access change could not be saved.")
	}
}

func bearerToken(r *http.Request) string {
	if r == nil {
		return ""
	}
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	return strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
}
