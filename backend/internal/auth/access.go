package auth

import (
	"errors"
	"net"
	"net/http"
	"sort"
	"strings"
	"time"

	"chakuchuri/backend/internal/platform/idgen"
)

type LoginActivity struct {
	At        string `json:"at"`
	Result    string `json:"result"`
	IP        string `json:"ip"`
	UserAgent string `json:"userAgent"`
	Detail    string `json:"detail"`
}

type KnownIP struct {
	IP           string `json:"ip"`
	FirstSeenAt  string `json:"firstSeenAt"`
	LastSeenAt   string `json:"lastSeenAt"`
	SuccessCount int    `json:"successCount"`
	FailCount    int    `json:"failCount,omitempty"`
}

type AccessOptions struct {
	Roles              []string            `json:"roles"`
	Pages              []string            `json:"pages"`
	Permissions        []string            `json:"permissions"`
	RoleDefaults       map[string][]string `json:"roleDefaults"`
	PageAccessDefaults map[string][]string `json:"pageAccessDefaults"`
	Statuses           []string            `json:"statuses"`
}

type UserPresence struct {
	UserID             string `json:"userId"`
	CustomerID         string `json:"customerId,omitempty"`
	ProfileImageFileID string `json:"profileImageFileId,omitempty"`
	Name               string `json:"name"`
	Email              string `json:"email,omitempty"`
	Role               string `json:"role"`
	Status             string `json:"status"`
	Online             bool   `json:"online"`
	LastOnline         string `json:"lastOnline"`
	LastLogin          string `json:"lastLogin"`
	ActiveSessions     int    `json:"activeSessions"`
}

type ManagedUserRequest struct {
	Name                string   `json:"name"`
	Email               string   `json:"email"`
	Password            string   `json:"password"`
	Role                string   `json:"role"`
	Status              string   `json:"status"`
	CustomerID          string   `json:"customerId"`
	AssignedCustomerIDs []string `json:"assignedCustomerIds"`
	Permissions         []string `json:"permissions"`
	PageAccess          []string `json:"pageAccess"`
	SuspensionNotice    string   `json:"suspensionNotice"`
}

type ProfileUpdateRequest struct {
	Name               string `json:"name"`
	Email              string `json:"email"`
	CurrentPassword    string `json:"currentPassword"`
	NewPassword        string `json:"newPassword"`
	ProfileImageFileID string `json:"profileImageFileId"`
	RemoveProfileImage bool   `json:"removeProfileImage"`
}

type NotificationReadRequest struct {
	IDs []string `json:"ids"`
}

type SuspensionContactRequest struct {
	Message string `json:"message"`
}

var ErrForbidden = errors.New("forbidden")
var ErrUserNotFound = errors.New("user not found")
var ErrCannotDeleteSelf = errors.New("cannot delete own user")
var ErrCurrentPassword = errors.New("current password is invalid")
var ErrWeakPassword = errors.New("password is too short")
var ErrNotSuspended = errors.New("user is not suspended")

const defaultSuspensionNotice = "Your access is temporarily paused while we review this account. Contact the ChakuChuri team below and we will help resolve it."

var roleCatalog = []string{
	"Owner",
	"Admin",
	"Manager",
	"Accountant",
	"Shipping Staff",
	"Production Staff",
	"Support Agent",
	"Customer",
}

var adminPageCatalog = []string{
	"Command",
	"Customers",
	"Users & Access",
	"Quotations",
	"Orders",
	"Shipping",
	"Payments",
	"Directory",
	"Messages",
	"Calls",
	"Email",
	"Settings",
}

var customerPageCatalog = []string{
	"Home",
	"Get Quote",
	"Products",
	"Orders",
	"Shipping",
	"Payments",
	"Directory",
	"Messages",
	"Settings",
}

var permissionCatalog = []string{
	"audit.read",
	"backups.manage",
	"calls.manage",
	"calls.start",
	"chat.manage",
	"chat.use",
	"customers.manage",
	"directory.manage",
	"email.manage",
	"email.read",
	"kyc.review",
	"manufacturing.manage",
	"orders.read",
	"payments.create",
	"payments.manage",
	"platform.settings.manage",
	"products.manage",
	"quotes.create",
	"quotes.manage",
	"shipping.create",
	"shipping.manage",
	"users.manage",
}

var statusCatalog = []string{"Active", "Temporarily suspended", "Paused", "Blocked"}

func (s *Service) AccessOptions(actor User) (AccessOptions, error) {
	if !canManageUsers(actor) {
		return AccessOptions{}, ErrForbidden
	}

	roleDefaults := map[string][]string{}
	pageDefaults := map[string][]string{}
	for _, role := range roleCatalog {
		roleDefaults[role] = defaultPermissionsForRole(role)
		pageDefaults[role] = defaultPageAccessForRole(role)
	}
	return AccessOptions{
		Roles:              append([]string(nil), roleCatalog...),
		Pages:              append([]string(nil), append(adminPageCatalog, customerPageCatalog...)...),
		Permissions:        append([]string(nil), permissionCatalog...),
		RoleDefaults:       roleDefaults,
		PageAccessDefaults: pageDefaults,
		Statuses:           append([]string(nil), statusCatalog...),
	}, nil
}

func (s *Service) ListUsers(actor User) ([]User, error) {
	if !canManageUsers(actor) {
		return nil, ErrForbidden
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := make([]string, 0, len(s.users))
	for key := range s.users {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	users := make([]User, 0, len(keys))
	for _, key := range keys {
		user := normalizeLoadedUser(s.users[key].User)
		if strings.EqualFold(user.Status, "Deleted") {
			continue
		}
		user.NotificationReadIDs = nil
		users = append(users, user)
	}
	return users, nil
}

const (
	presenceOnlineWindow = 90 * time.Second
	presencePersistEvery = time.Minute
	presencePublishEvery = 12 * time.Second
)

func isRecentlyActive(lastOnline string, now time.Time) bool {
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(lastOnline))
	if err != nil {
		return false
	}
	delta := now.Sub(parsed)
	// Allow a few seconds of clock skew between devices.
	if delta < -5*time.Second {
		return false
	}
	if delta < 0 {
		delta = 0
	}
	return delta <= presenceOnlineWindow
}

func (s *Service) TouchOnline(userID string) {
	s.touchOnline(userID, false)
}

func (s *Service) Heartbeat(userID string) {
	s.touchOnline(userID, true)
}

func (s *Service) touchOnline(userID string, publish bool) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return
	}

	s.mu.Lock()
	key, user, ok := s.findUserByIDLocked(userID)
	if !ok {
		s.mu.Unlock()
		return
	}
	now := time.Now().UTC()
	// A heartbeat means the client is alive again.
	delete(s.forcedOffline, user.ID)

	wasOnline := s.isUserOnlineLocked(key, user, now)
	previous := user.LastOnline
	user.LastOnline = now.Format(time.RFC3339)
	stored := s.users[key]
	stored.User = user
	s.users[key] = stored

	shouldPersist := true
	if parsed, err := time.Parse(time.RFC3339, previous); err == nil && now.Sub(parsed) < presencePersistEvery {
		shouldPersist = false
	}
	if shouldPersist {
		s.persistLocked()
	}

	becameOnline := !wasOnline
	shouldPublish := publish || becameOnline
	if publish && !becameOnline {
		if last, ok := s.lastPresencePublish[user.ID]; ok && now.Sub(last) < presencePublishEvery {
			shouldPublish = false
		}
	}
	var presence UserPresence
	if shouldPublish {
		if s.lastPresencePublish == nil {
			s.lastPresencePublish = map[string]time.Time{}
		}
		s.lastPresencePublish[user.ID] = now
		presence = s.presenceRecordLocked(key, s.users[key], now, s.activeSessionCountsLocked(now))
	}
	notifier := s.presenceNotifier
	s.mu.Unlock()

	if shouldPublish && notifier != nil {
		notifier(presence)
	}
}

func (s *Service) Logout(token string) {
	token = strings.TrimSpace(token)
	if token == "" {
		return
	}

	s.mu.Lock()
	session, ok := s.sessions[token]
	if !ok {
		s.mu.Unlock()
		return
	}
	delete(s.sessions, token)
	now := time.Now().UTC()
	stored, exists := s.users[session.UserEmail]
	var presence UserPresence
	var notify bool
	if exists {
		// Keep a real last-seen timestamp, but force offline until the next heartbeat.
		stored.User.LastOnline = now.Format(time.RFC3339)
		s.users[session.UserEmail] = stored
		if s.forcedOffline == nil {
			s.forcedOffline = map[string]time.Time{}
		}
		s.forcedOffline[stored.User.ID] = now.Add(presenceOnlineWindow)
		s.persistLocked()
		presence = s.presenceRecordLocked(session.UserEmail, stored, now, s.activeSessionCountsLocked(now))
		notify = true
		if s.lastPresencePublish == nil {
			s.lastPresencePublish = map[string]time.Time{}
		}
		s.lastPresencePublish[stored.User.ID] = now
	}
	notifier := s.presenceNotifier
	s.mu.Unlock()

	if notify && notifier != nil {
		notifier(presence)
	}
}

func (s *Service) activeSessionCountsLocked(now time.Time) map[string]int {
	activeSessions := map[string]int{}
	for _, session := range s.sessions {
		if now.Before(session.ExpiresAt) {
			activeSessions[session.UserEmail]++
		}
	}
	return activeSessions
}

func (s *Service) isForcedOfflineLocked(userID string, now time.Time) bool {
	until, ok := s.forcedOffline[userID]
	if !ok {
		return false
	}
	if now.After(until) {
		delete(s.forcedOffline, userID)
		return false
	}
	return true
}

func (s *Service) isUserOnlineLocked(key string, user User, now time.Time) bool {
	if s.isForcedOfflineLocked(user.ID, now) {
		return false
	}
	return s.activeSessionCountsLocked(now)[key] > 0 && isRecentlyActive(user.LastOnline, now)
}

func (s *Service) presenceRecordLocked(key string, stored storedUser, now time.Time, activeSessions map[string]int) UserPresence {
	user := normalizeLoadedUser(stored.User)
	email := user.Email
	if IsCustomerRole(user) {
		email = ""
	}
	online := !s.isForcedOfflineLocked(user.ID, now) && activeSessions[key] > 0 && isRecentlyActive(user.LastOnline, now)
	return UserPresence{
		UserID:             user.ID,
		CustomerID:         user.CustomerID,
		ProfileImageFileID: user.ProfileImageFileID,
		Name:               user.Name,
		Email:              email,
		Role:               user.Role,
		Status:             user.Status,
		Online:             online,
		LastOnline:         firstNonEmpty(user.LastOnline, "Never"),
		LastLogin:          firstNonEmpty(user.LastLogin, "Never"),
		ActiveSessions:     activeSessions[key],
	}
}

func (s *Service) PresenceFor(actor User) []UserPresence {
	actor = normalizeLoadedUser(actor)

	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now().UTC()
	activeSessions := s.activeSessionCountsLocked(now)

	presence := []UserPresence{}
	for key, stored := range s.users {
		user := normalizeLoadedUser(stored.User)
		if strings.EqualFold(user.Status, "Deleted") || !canSeePresence(actor, user) {
			continue
		}
		record := s.presenceRecordLocked(key, stored, now, activeSessions)
		if IsCustomerRole(actor) {
			record.Email = ""
		}
		presence = append(presence, record)
	}

	sort.Slice(presence, func(i, j int) bool {
		if presence[i].Online != presence[j].Online {
			return presence[i].Online
		}
		return strings.ToLower(presence[i].Name) < strings.ToLower(presence[j].Name)
	})
	return presence
}

func (s *Service) PresenceByCustomerID(customerID string) (online bool, lastOnline string) {
	customerID = strings.TrimSpace(customerID)
	if customerID == "" {
		return false, ""
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := time.Now().UTC()
	activeSessions := s.activeSessionCountsLocked(now)
	for key, stored := range s.users {
		user := normalizeLoadedUser(stored.User)
		if !IsCustomerRole(user) || user.CustomerID != customerID {
			continue
		}
		if strings.EqualFold(user.Status, "Deleted") {
			continue
		}
		record := s.presenceRecordLocked(key, stored, now, activeSessions)
		if record.Online || lastOnline == "" {
			online = record.Online
			lastOnline = record.LastOnline
		}
		if record.Online {
			return online, lastOnline
		}
	}
	return online, lastOnline
}

// IsCustomerOnline reports whether any portal user for the customer is currently online.
func (s *Service) IsCustomerOnline(customerID string) bool {
	online, _ := s.PresenceByCustomerID(customerID)
	return online
}

// IsSupportOnline reports whether any staff user who can take calls is online.
func (s *Service) IsSupportOnline() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := time.Now().UTC()
	activeSessions := s.activeSessionCountsLocked(now)
	for key, stored := range s.users {
		user := normalizeLoadedUser(stored.User)
		if IsCustomerRole(user) || strings.EqualFold(user.Status, "Deleted") || !isActiveUser(user) {
			continue
		}
		if !HasAnyPermission(user, "calls.manage", "chat.manage") {
			continue
		}
		if s.presenceRecordLocked(key, stored, now, activeSessions).Online {
			return true
		}
	}
	return false
}

func (s *Service) CreateManagedUser(payload ManagedUserRequest, actor User) (User, error) {
	if !canManageUsers(actor) {
		return User{}, ErrForbidden
	}
	user := User{
		Name:                payload.Name,
		Email:               payload.Email,
		Role:                payload.Role,
		Status:              payload.Status,
		CustomerID:          payload.CustomerID,
		AssignedCustomerIDs: payload.AssignedCustomerIDs,
		Permissions:         payload.Permissions,
		PageAccess:          payload.PageAccess,
		SuspensionNotice:    payload.SuspensionNotice,
	}
	user = normalizeManagedUser(user, "")
	created, err := s.CreateUser(user, payload.Password)
	if err != nil {
		return User{}, err
	}
	s.provisionCustomer(created)
	if s.recorder != nil {
		s.recorder.Record(actor.Email, "auth.user_created", created.ID, "Managed user created.")
	}
	return created, nil
}

func (s *Service) UpdateManagedUser(id string, payload ManagedUserRequest, actor User) (User, error) {
	if !canManageUsers(actor) {
		return User{}, ErrForbidden
	}

	s.mu.Lock()

	key, current, ok := s.findUserByIDLocked(id)
	if !ok {
		s.mu.Unlock()
		return User{}, ErrUserNotFound
	}

	nextEmail := strings.ToLower(strings.TrimSpace(firstNonEmpty(payload.Email, current.Email)))
	if nextEmail == "" {
		s.mu.Unlock()
		return User{}, ErrInvalidCredentials
	}
	if nextEmail != key {
		if _, exists := s.users[nextEmail]; exists {
			s.mu.Unlock()
			return User{}, ErrEmailExists
		}
	}

	previousCustomerID := current.CustomerID
	previousStatus := current.Status
	current.Name = firstNonEmpty(payload.Name, current.Name)
	current.Email = nextEmail
	current.Role = firstNonEmpty(payload.Role, current.Role)
	current.Status = normalizeStatus(firstNonEmpty(payload.Status, current.Status))
	current.CustomerID = strings.TrimSpace(payload.CustomerID)
	current.AssignedCustomerIDs = payload.AssignedCustomerIDs
	current.Permissions = payload.Permissions
	current.PageAccess = payload.PageAccess
	current.SuspensionNotice = strings.TrimSpace(payload.SuspensionNotice)
	current = normalizeManagedUser(current, previousCustomerID)
	if isTemporarilySuspended(current) {
		current.SuspensionNotice = firstNonEmpty(current.SuspensionNotice, defaultSuspensionNotice)
	} else if isTemporarilySuspended(User{Status: previousStatus}) {
		current.SuspensionNotice = ""
		current.SuspensionContactAt = ""
		current.SuspensionMessage = ""
	}
	current = appendLoginActivity(current, "Access updated", "", "", "User access profile changed.")

	stored := s.users[key]
	stored.User = current
	passwordChanged := strings.TrimSpace(payload.Password) != ""
	if passwordChanged {
		stored.salt = randomToken()
		stored.passwordHash = hashPassword(stored.salt, payload.Password)
		stored.User = appendLoginActivity(stored.User, "Password reset", "", "", "Password changed by admin.")
	}
	if nextEmail != key {
		delete(s.users, key)
	}
	s.users[nextEmail] = stored
	if nextEmail != key || passwordChanged || !canStartSession(stored.User) {
		s.invalidateSessionsForEmailLocked(key)
		if nextEmail != key {
			s.invalidateSessionsForEmailLocked(nextEmail)
		}
	}
	result := stored.User
	s.persistLocked()
	if s.recorder != nil {
		s.recorder.Record(actor.Email, "auth.user_updated", current.ID, "Managed user access updated.")
	}
	s.mu.Unlock()
	s.notifyChange("users")

	// Outside the lock so customer provisioning can rebind without deadlocking.
	s.provisionCustomer(result)
	s.mu.RLock()
	_, rebound, found := s.findUserByIDLocked(result.ID)
	s.mu.RUnlock()
	if found {
		return rebound, nil
	}
	return result, nil
}

func (s *Service) UpdateOwnProfile(actor User, payload ProfileUpdateRequest, currentToken string) (User, error) {
	if !isActiveUser(actor) {
		return User{}, ErrForbidden
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.persistLocked()

	key, current, ok := s.findUserByIDLocked(actor.ID)
	if !ok {
		return User{}, ErrUserNotFound
	}
	nextName := strings.TrimSpace(payload.Name)
	nextEmail := strings.ToLower(strings.TrimSpace(payload.Email))
	if nextName == "" || nextEmail == "" {
		return User{}, ErrInvalidCredentials
	}
	if nextEmail != key {
		if _, exists := s.users[nextEmail]; exists {
			return User{}, ErrEmailExists
		}
	}

	stored := s.users[key]
	sensitiveChange := nextEmail != key || strings.TrimSpace(payload.NewPassword) != ""
	if sensitiveChange && stored.passwordHash != hashPassword(stored.salt, payload.CurrentPassword) {
		return User{}, ErrCurrentPassword
	}
	if payload.NewPassword != "" && len(payload.NewPassword) < 8 {
		return User{}, ErrWeakPassword
	}

	current.Name = nextName
	current.Email = nextEmail
	if payload.RemoveProfileImage {
		current.ProfileImageFileID = ""
	} else if strings.TrimSpace(payload.ProfileImageFileID) != "" {
		current.ProfileImageFileID = strings.TrimSpace(payload.ProfileImageFileID)
	}
	current = appendLoginActivity(current, "Profile updated", "", "", "Personal account settings changed.")
	stored.User = current
	if payload.NewPassword != "" {
		stored.salt = randomToken()
		stored.passwordHash = hashPassword(stored.salt, payload.NewPassword)
		stored.User = appendLoginActivity(stored.User, "Password changed", "", "", "Password changed by user.")
	}
	if nextEmail != key {
		delete(s.users, key)
	}
	s.users[nextEmail] = stored
	if sensitiveChange {
		for token, session := range s.sessions {
			if session.UserEmail != key && session.UserEmail != nextEmail {
				continue
			}
			if token == currentToken {
				session.UserEmail = nextEmail
				s.sessions[token] = session
				continue
			}
			delete(s.sessions, token)
		}
	}
	if s.recorder != nil {
		s.recorder.Record(nextEmail, "auth.profile_updated", current.ID, "User updated personal account settings.")
	}
	s.notifyChange("users")
	return normalizeLoadedUser(stored.User), nil
}

func (s *Service) ContactAboutSuspension(actor User, message string) (User, error) {
	if !isTemporarilySuspended(actor) {
		return User{}, ErrNotSuspended
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.persistLocked()
	key, current, ok := s.findUserByIDLocked(actor.ID)
	if !ok {
		return User{}, ErrUserNotFound
	}
	if !isTemporarilySuspended(current) {
		return User{}, ErrNotSuspended
	}
	message = strings.TrimSpace(message)
	if message == "" {
		message = "Please review my account suspension and contact me with the next steps."
	}
	if len(message) > 1000 {
		message = message[:1000]
	}
	current.SuspensionContactAt = time.Now().UTC().Format(time.RFC3339)
	current.SuspensionMessage = message
	current = appendLoginActivity(current, "Support contacted", "", "", "Suspension review requested.")
	stored := s.users[key]
	stored.User = current
	s.users[key] = stored
	if s.recorder != nil {
		s.recorder.Record(current.Email, "auth.suspension_contacted", current.ID, "Suspended user requested account review.")
	}
	s.notifyChange("users")
	return normalizeLoadedUser(current), nil
}
func (s *Service) DeleteManagedUser(id string, actor User) error {
	if !canManageUsers(actor) {
		return ErrForbidden
	}
	if strings.TrimSpace(id) == "" {
		return ErrUserNotFound
	}
	if actor.ID == id {
		return ErrCannotDeleteSelf
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	key, user, ok := s.findUserByIDLocked(id)
	if !ok {
		return ErrUserNotFound
	}
	if s.repository != nil {
		if err := s.repository.DeleteUser(user); err != nil {
			return err
		}
	}
	delete(s.users, key)
	s.invalidateSessionsForEmailLocked(key)
	s.persistLocked()
	if s.recorder != nil {
		s.recorder.Record(actor.Email, "auth.user_deleted", user.ID, "Managed user permanently deleted.")
	}
	s.notifyChange("users")
	return nil
}

func (s *Service) provisionCustomer(user User) {
	if s.customerProvisioner == nil || !IsCustomerRole(user) {
		return
	}
	s.customerProvisioner(user)
}

// RebindCustomerID points a customer user at the canonical customer account id.
// Returns the previous customer id when a change was made.
func (s *Service) RebindCustomerID(userID, customerID string) (previous string, changed bool) {
	userID = strings.TrimSpace(userID)
	customerID = strings.TrimSpace(customerID)
	if userID == "" || customerID == "" {
		return "", false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	key, user, ok := s.findUserByIDLocked(userID)
	if !ok || !IsCustomerRole(user) {
		return "", false
	}
	if user.CustomerID == customerID {
		return "", false
	}
	previous = user.CustomerID
	user.CustomerID = customerID
	stored := s.users[key]
	stored.User = user
	s.users[key] = stored
	s.persistLocked()
	s.notifyChange("users")
	return previous, true
}

// HealCustomerBindings re-runs portal provisioning so drifted customerIds snap back.
// Returns from→to pairs that changed so callers can remap related workflow rows.
func (s *Service) HealCustomerBindings() [][2]string {
	s.mu.RLock()
	users := make([]User, 0, len(s.users))
	for _, stored := range s.users {
		user := normalizeLoadedUser(stored.User)
		if IsCustomerRole(user) && !strings.EqualFold(user.Status, "Deleted") {
			users = append(users, user)
		}
	}
	s.mu.RUnlock()

	changes := make([][2]string, 0)
	seen := map[string]struct{}{}
	for _, user := range users {
		before := user.CustomerID
		s.provisionCustomer(user)
		// provisioner may call RebindCustomerID; re-read current binding
		s.mu.RLock()
		_, current, ok := s.findUserByIDLocked(user.ID)
		s.mu.RUnlock()
		if !ok {
			continue
		}
		after := current.CustomerID
		if before == "" || after == "" || before == after {
			continue
		}
		key := before + "→" + after
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		changes = append(changes, [2]string{before, after})
	}
	return changes
}

func (s *Service) findUserByIDLocked(id string) (string, User, bool) {
	id = strings.TrimSpace(id)
	for key, row := range s.users {
		if row.User.ID == id {
			return key, row.User, true
		}
	}
	return "", User{}, false
}

func (s *Service) invalidateSessionsForEmailLocked(email string) {
	email = strings.ToLower(strings.TrimSpace(email))
	for token, session := range s.sessions {
		if session.UserEmail == email {
			delete(s.sessions, token)
		}
	}
}

func canManageUsers(user User) bool {
	return HasPermission(user, "users.manage")
}

func canSeePresence(actor User, target User) bool {
	if actor.ID == target.ID {
		return true
	}
	if IsCustomerRole(actor) {
		return !IsCustomerRole(target) && isActiveUser(target) && HasAnyPermission(target, "chat.manage", "calls.manage")
	}
	return HasAnyPermission(actor, "chat.manage", "calls.manage", "users.manage")
}

func hasPermission(user User, permission string) bool {
	for _, value := range user.Permissions {
		if value == permission {
			return true
		}
	}
	return false
}

func HasPermission(user User, permission string) bool {
	role := strings.ToLower(strings.TrimSpace(user.Role))
	if role == "owner" || role == "admin" {
		return true
	}
	return hasPermission(user, permission)
}

func HasAnyPermission(user User, permissions ...string) bool {
	for _, permission := range permissions {
		if HasPermission(user, permission) {
			return true
		}
	}
	return false
}

func IsCustomerRole(user User) bool {
	return strings.EqualFold(strings.TrimSpace(user.Role), "Customer")
}

func normalizeManagedUser(user User, previousCustomerID string) User {
	user.Role = normalizeRole(user.Role)
	user.Status = normalizeStatus(user.Status)
	user.Email = strings.ToLower(strings.TrimSpace(user.Email))
	if IsCustomerRole(user) {
		user.AssignedCustomerIDs = nil
		user.CustomerID = firstNonEmpty(user.CustomerID, previousCustomerID, customerIDFromUser(user))
	} else {
		user.CustomerID = ""
		user.AssignedCustomerIDs = nil
	}
	if isTemporarilySuspended(user) {
		user.SuspensionNotice = firstNonEmpty(user.SuspensionNotice, defaultSuspensionNotice)
	} else {
		user.SuspensionNotice = ""
	}
	user.Permissions = cleanPermissionsForRole(user.Permissions, user.Role)
	if len(user.Permissions) == 0 {
		user.Permissions = defaultPermissionsForRole(user.Role)
	}
	user.Permissions = ensureDefaultCatalogItems(user.Permissions, defaultPermissionsForRole(user.Role), "directory.manage")
	user.PageAccess = cleanPageAccess(user.PageAccess, user.Role)
	return user
}

func CustomerScope(user User) (bool, map[string]bool) {
	if IsCustomerRole(user) {
		allowed := map[string]bool{}
		if customerID := strings.TrimSpace(user.CustomerID); customerID != "" {
			allowed[customerID] = true
		}
		return false, allowed
	}
	return true, nil
}

func CanAccessCustomer(user User, customerID string) bool {
	customerID = strings.TrimSpace(customerID)
	if customerID == "" {
		return !IsCustomerRole(user)
	}
	all, allowed := CustomerScope(user)
	if all {
		return true
	}
	return allowed[customerID]
}

func normalizeLoadedUser(user User) User {
	user.Status = normalizeStatus(user.Status)
	user.Role = normalizeRole(user.Role)
	user.Email = strings.ToLower(strings.TrimSpace(user.Email))
	user = normalizeManagedUser(user, user.CustomerID)
	if isTemporarilySuspended(user) {
		user.SuspensionNotice = firstNonEmpty(user.SuspensionNotice, defaultSuspensionNotice)
	}
	if user.LastOnline == "" {
		user.LastOnline = "Never"
	}
	if user.LastLogin == "" {
		user.LastLogin = user.LastOnline
	}
	user.LoginActivity = trimLoginActivity(user.LoginActivity)
	if len(user.KnownIPs) == 0 && len(user.LoginActivity) > 0 {
		user.KnownIPs = rebuildKnownIPsFromActivity(user.LoginActivity, nil)
	} else {
		user.KnownIPs = trimKnownIPs(user.KnownIPs)
	}
	user.NotificationReadIDs = sanitizeNotificationReads(user.NotificationReadIDs)
	return user
}

func (s *Service) MarkNotificationsRead(actor User, ids []string) (User, error) {
	if !isActiveUser(actor) {
		return User{}, ErrForbidden
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.persistLocked()

	key, current, ok := s.findUserByIDLocked(actor.ID)
	if !ok {
		return User{}, ErrUserNotFound
	}
	current.NotificationReadIDs = sanitizeNotificationReads(append(append([]string{}, current.NotificationReadIDs...), ids...))
	stored := s.users[key]
	stored.User = current
	s.users[key] = stored
	return normalizeLoadedUser(current), nil
}

func sanitizeNotificationReads(ids []string) []string {
	seen := map[string]bool{}
	clean := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || len(id) > 180 || seen[id] {
			continue
		}
		seen[id] = true
		clean = append(clean, id)
	}
	if len(clean) > 400 {
		return clean[len(clean)-400:]
	}
	return clean
}

func normalizeNewUser(user User) User {
	user = normalizeLoadedUser(user)
	if user.ID == "" {
		user.ID = idgen.New("usr")
	}
	if user.TenantID == "" {
		user.TenantID = "tenant_chakuchuri"
	}
	return user
}

func normalizeRole(role string) string {
	role = strings.TrimSpace(role)
	for _, value := range roleCatalog {
		if strings.EqualFold(value, role) {
			return value
		}
	}
	if role == "" {
		return "Customer"
	}
	return role
}

func normalizeStatus(status string) string {
	status = strings.TrimSpace(status)
	for _, value := range statusCatalog {
		if strings.EqualFold(value, status) {
			return value
		}
	}
	if strings.EqualFold(status, "suspended") || strings.EqualFold(status, "temporarily suspended") {
		return "Temporarily suspended"
	}
	if strings.EqualFold(status, "inactive") {
		return "Paused"
	}
	if strings.EqualFold(status, "deleted") {
		return "Deleted"
	}
	return "Active"
}

func isActiveUser(user User) bool {
	return strings.EqualFold(normalizeStatus(user.Status), "Active")
}

func isTemporarilySuspended(user User) bool {
	return strings.EqualFold(normalizeStatus(user.Status), "Temporarily suspended")
}

func canStartSession(user User) bool {
	return isActiveUser(user) || isTemporarilySuspended(user)
}

func appendLoginActivity(user User, result string, ip string, userAgent string, detail string) User {
	now := time.Now().UTC().Format(time.RFC3339)
	ip = strings.TrimSpace(ip)
	user.LoginActivity = append([]LoginActivity{{
		At:        now,
		Result:    result,
		IP:        ip,
		UserAgent: strings.TrimSpace(userAgent),
		Detail:    strings.TrimSpace(detail),
	}}, trimLoginActivity(user.LoginActivity)...)
	user.LoginActivity = trimLoginActivity(user.LoginActivity)
	user.KnownIPs = touchKnownIP(user.KnownIPs, ip, result, now)
	return user
}

func touchKnownIP(values []KnownIP, ip string, result string, at string) []KnownIP {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return trimKnownIPs(values)
	}
	at = firstNonEmpty(strings.TrimSpace(at), time.Now().UTC().Format(time.RFC3339))
	success := strings.EqualFold(strings.TrimSpace(result), "Success")
	failed := strings.Contains(strings.ToLower(result), "fail") || strings.Contains(strings.ToLower(result), "block")

	for index := range values {
		if values[index].IP != ip {
			continue
		}
		values[index].LastSeenAt = at
		if values[index].FirstSeenAt == "" {
			values[index].FirstSeenAt = at
		}
		if success {
			values[index].SuccessCount++
		}
		if failed {
			values[index].FailCount++
		}
		updated := values[index]
		rest := append(append([]KnownIP{}, values[:index]...), values[index+1:]...)
		return trimKnownIPs(append([]KnownIP{updated}, rest...))
	}

	entry := KnownIP{IP: ip, FirstSeenAt: at, LastSeenAt: at}
	if success {
		entry.SuccessCount = 1
	}
	if failed {
		entry.FailCount = 1
	}
	return trimKnownIPs(append([]KnownIP{entry}, values...))
}

func trimLoginActivity(values []LoginActivity) []LoginActivity {
	if len(values) > 25 {
		values = values[:25]
	}
	return values
}

func trimKnownIPs(values []KnownIP) []KnownIP {
	if values == nil {
		return []KnownIP{}
	}
	sort.SliceStable(values, func(i, j int) bool {
		return values[i].LastSeenAt > values[j].LastSeenAt
	})
	if len(values) > 50 {
		values = values[:50]
	}
	return values
}

func rebuildKnownIPsFromActivity(activity []LoginActivity, existing []KnownIP) []KnownIP {
	byIP := make(map[string]KnownIP, len(existing)+len(activity))
	for _, row := range existing {
		ip := strings.TrimSpace(row.IP)
		if ip == "" {
			continue
		}
		byIP[ip] = row
	}
	// Walk oldest → newest so first/last seen settle correctly.
	for index := len(activity) - 1; index >= 0; index-- {
		row := activity[index]
		ip := strings.TrimSpace(row.IP)
		if ip == "" {
			continue
		}
		current := byIP[ip]
		if current.IP == "" {
			current.IP = ip
			current.FirstSeenAt = row.At
		}
		if current.FirstSeenAt == "" || (row.At != "" && row.At < current.FirstSeenAt) {
			current.FirstSeenAt = row.At
		}
		if current.LastSeenAt == "" || row.At > current.LastSeenAt {
			current.LastSeenAt = row.At
		}
		lower := strings.ToLower(row.Result)
		if strings.EqualFold(row.Result, "Success") {
			current.SuccessCount++
		} else if strings.Contains(lower, "fail") || strings.Contains(lower, "block") {
			current.FailCount++
		}
		byIP[ip] = current
	}
	out := make([]KnownIP, 0, len(byIP))
	for _, row := range byIP {
		out = append(out, row)
	}
	return trimKnownIPs(out)
}

func requestIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	remote := normalizeIP(remoteAddrIP(r.RemoteAddr))
	if isTrustedProxy(remote) {
		if ip := clientIPFromHeaders(r); ip != "" {
			return ip
		}
	}
	return remote
}

func clientIPFromHeaders(r *http.Request) string {
	candidates := make([]string, 0, 8)
	for _, header := range []string{"CF-Connecting-IP", "True-Client-IP", "X-Real-IP"} {
		if value := strings.TrimSpace(r.Header.Get(header)); value != "" {
			candidates = append(candidates, value)
		}
	}
	if value := strings.TrimSpace(r.Header.Get("Forwarded")); value != "" {
		candidates = append(candidates, parseForwardedFor(value)...)
	}
	if value := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); value != "" {
		for _, part := range strings.Split(value, ",") {
			candidates = append(candidates, strings.TrimSpace(part))
		}
	}

	firstValid := ""
	for _, candidate := range candidates {
		ip := normalizeIP(candidate)
		if ip == "" || !isValidIP(ip) {
			continue
		}
		if firstValid == "" {
			firstValid = ip
		}
		if !isLoopbackIP(ip) {
			return ip
		}
	}
	return firstValid
}

func parseForwardedFor(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		for _, field := range strings.Split(part, ";") {
			field = strings.TrimSpace(field)
			if len(field) < 5 || !strings.EqualFold(field[:4], "for=") {
				continue
			}
			raw := strings.TrimSpace(field[4:])
			raw = strings.Trim(raw, `"`)
			if strings.HasPrefix(raw, "[") {
				if end := strings.Index(raw, "]"); end > 1 {
					raw = raw[1:end]
				}
			} else if host, _, err := net.SplitHostPort(raw); err == nil {
				raw = host
			}
			out = append(out, raw)
		}
	}
	return out
}

func remoteAddrIP(remoteAddr string) string {
	remoteAddr = strings.TrimSpace(remoteAddr)
	if remoteAddr == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil {
		return host
	}
	return remoteAddr
}

func normalizeIP(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"'`)
	if value == "" || strings.EqualFold(value, "unknown") {
		return ""
	}
	if strings.HasPrefix(value, "[") {
		if end := strings.Index(value, "]"); end > 1 {
			inside := value[1:end]
			rest := value[end+1:]
			if rest == "" || strings.HasPrefix(rest, ":") {
				value = inside
			}
		}
	} else if strings.Count(value, ":") == 1 {
		// IPv4 host:port only — never split IPv6 / IPv4-mapped forms.
		if host, _, err := net.SplitHostPort(value); err == nil {
			value = host
		}
	}
	if parsed := net.ParseIP(value); parsed != nil {
		if v4 := parsed.To4(); v4 != nil {
			return v4.String()
		}
		return parsed.String()
	}
	return ""
}

func isValidIP(value string) bool {
	return net.ParseIP(strings.TrimSpace(value)) != nil
}

func isLoopbackIP(value string) bool {
	ip := net.ParseIP(strings.TrimSpace(value))
	return ip != nil && ip.IsLoopback()
}

func isTrustedProxy(value string) bool {
	ip := net.ParseIP(strings.TrimSpace(value))
	if ip == nil {
		return false
	}
	if ip.IsLoopback() {
		return true
	}
	// Local/dev reverse proxies (Vite, nginx, docker bridge, private LAN gateways).
	if ip.IsPrivate() || ip.IsLinkLocalUnicast() {
		return true
	}
	return false
}

func requestUserAgent(r *http.Request) string {
	if r == nil {
		return ""
	}
	return r.UserAgent()
}

func cleanPageAccess(values []string, role string) []string {
	expanded := make([]string, 0, len(values)+1)
	for _, value := range values {
		value = strings.TrimSpace(value)
		if strings.EqualFold(value, "Messages & Calls") {
			expanded = append(expanded, "Messages", "Calls")
			continue
		}
		if strings.EqualFold(value, "Manufacturing") {
			expanded = append(expanded, "Orders")
			continue
		}
		expanded = append(expanded, value)
	}
	clean := filterCatalog(cleanStringSlice(expanded), pageCatalogForRole(role))
	if len(clean) == 0 {
		clean = defaultPageAccessForRole(role)
	}
	// Roll out new default pages (e.g. Directory) to users saved before they existed.
	clean = ensureDefaultCatalogItems(clean, defaultPageAccessForRole(role), "Directory")
	for _, page := range clean {
		if page == "Settings" {
			return clean
		}
	}
	return append(clean, "Settings")
}

func ensureDefaultCatalogItems(current []string, defaults []string, required ...string) []string {
	if len(required) == 0 {
		return current
	}
	defaultSet := map[string]bool{}
	for _, item := range defaults {
		defaultSet[item] = true
	}
	have := map[string]bool{}
	for _, item := range current {
		have[item] = true
	}
	out := append([]string(nil), current...)
	for _, item := range required {
		if !defaultSet[item] || have[item] {
			continue
		}
		out = append(out, item)
		have[item] = true
	}
	return out
}

func cleanStringSlice(values []string) []string {
	seen := map[string]bool{}
	clean := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		clean = append(clean, value)
	}
	sort.Strings(clean)
	return clean
}

func cleanPermissionsForRole(values []string, role string) []string {
	return filterCatalog(cleanPermissions(values), permissionCatalogForRole(role))
}

func pageCatalogForRole(role string) []string {
	if strings.EqualFold(strings.TrimSpace(role), "Customer") {
		return customerPageCatalog
	}
	return adminPageCatalog
}

func permissionCatalogForRole(role string) []string {
	if strings.EqualFold(strings.TrimSpace(role), "Customer") {
		return defaultPermissionsForRole("Customer")
	}
	internal := make([]string, 0, len(permissionCatalog))
	for _, permission := range permissionCatalog {
		switch permission {
		case "calls.start", "chat.use", "payments.create", "quotes.create", "shipping.create":
			continue
		default:
			internal = append(internal, permission)
		}
	}
	return internal
}

func filterCatalog(values []string, catalog []string) []string {
	if len(values) == 0 {
		return nil
	}
	requested := map[string]bool{}
	for _, value := range values {
		requested[strings.ToLower(strings.TrimSpace(value))] = true
	}
	filtered := make([]string, 0, len(values))
	for _, value := range catalog {
		if requested[strings.ToLower(value)] {
			filtered = append(filtered, value)
		}
	}
	return filtered
}

func customerIDFromUser(user User) string {
	base := firstNonEmpty(user.Name, strings.Split(user.Email, "@")[0], "customer")
	return "cust_" + safeAccessID(base)
}

func safeAccessID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	builder := strings.Builder{}
	lastSeparator := false
	for _, char := range value {
		switch {
		case char >= 'a' && char <= 'z' || char >= '0' && char <= '9':
			builder.WriteRune(char)
			lastSeparator = false
		case char == ' ' || char == '-' || char == '_' || char == '.':
			if !lastSeparator {
				builder.WriteRune('_')
				lastSeparator = true
			}
		}
	}
	cleaned := strings.Trim(builder.String(), "_")
	if cleaned == "" {
		return "customer"
	}
	return cleaned
}

func defaultPageAccessForRole(role string) []string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "owner", "admin":
		return append([]string(nil), adminPageCatalog...)
	case "manager":
		return []string{"Command", "Customers", "Quotations", "Orders", "Shipping", "Payments", "Directory", "Messages", "Calls"}
	case "accountant":
		return []string{"Command", "Customers", "Payments", "Settings"}
	case "shipping staff":
		return []string{"Command", "Shipping", "Messages"}
	case "production staff":
		return []string{"Command", "Orders", "Messages"}
	case "support agent":
		return []string{"Command", "Customers", "Directory", "Messages", "Calls"}
	default:
		return append([]string(nil), customerPageCatalog...)
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
