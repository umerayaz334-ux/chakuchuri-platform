package workflow

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"chakuchuri/backend/internal/auth"
	"chakuchuri/backend/internal/platform/httpx"
	"chakuchuri/backend/internal/platform/websocket"
)

type workspaceSocketMessage struct {
	Type string `json:"type"`
}

type WorkspaceChange struct {
	Scope      string          `json:"scope"`
	Operation  string          `json:"operation"`
	CustomerID string          `json:"customerId,omitempty"`
	Data       json.RawMessage `json:"data,omitempty"`
}

type workspaceSocketEvent struct {
	Type       string            `json:"type"`
	Revision   uint64            `json:"revision"`
	ServerTime string            `json:"serverTime"`
	Changes    []WorkspaceChange `json:"changes,omitempty"`
}

type workspaceEventHub struct {
	mu      sync.Mutex
	clients map[*workspaceClient]struct{}
}

type workspaceClient struct {
	revision  uint64 // Protected by workspaceEventHub.mu.
	authorize func() bool
	user      auth.User
	conn      *websocket.Conn
	send      chan workspaceSocketEvent
	done      chan struct{}
	once      sync.Once
}

func newWorkspaceEventHub() *workspaceEventHub {
	return &workspaceEventHub{clients: map[*workspaceClient]struct{}{}}
}

func workspaceUpsert(scope string, customerID string, value interface{}) WorkspaceChange {
	raw, err := json.Marshal(value)
	if err != nil {
		return WorkspaceChange{}
	}
	return WorkspaceChange{Scope: scope, Operation: "upsert", CustomerID: strings.TrimSpace(customerID), Data: raw}
}

func (s *Service) notifyWorkspaceUpsert(scope string, customerID string, value interface{}) {
	s.notifyWorkspaceChanges(workspaceUpsert(scope, customerID, value))
}

func (s *Service) notifyWorkspaceChanges(changes ...WorkspaceChange) {
	if s == nil || s.workspaceEvents == nil {
		return
	}
	valid := make([]WorkspaceChange, 0, len(changes))
	for _, change := range changes {
		if strings.TrimSpace(change.Scope) != "" && strings.TrimSpace(change.Operation) != "" {
			valid = append(valid, change)
		}
	}
	if len(valid) > 0 {
		s.workspaceEvents.publish(valid)
	}
}

// NotifyExternalChange lets auth and customer services invalidate only their
// own resource. Operational workflow mutations carry their changed records.
func (s *Service) NotifyExternalChange(scope string) {
	s.notifyWorkspaceChanges(WorkspaceChange{Scope: strings.TrimSpace(scope), Operation: "invalidate"})
}

func (s *Service) NotifyPresence(presence auth.UserPresence) {
	if strings.TrimSpace(presence.UserID) == "" {
		return
	}
	s.notifyWorkspaceUpsert("presence", presence.CustomerID, presence)
}

func (s *Service) handleWorkspaceSocket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpx.Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Workspace realtime requires a WebSocket GET request.")
		return
	}

	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if r.Header.Get("Authorization") == "" && token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	user, ok := s.authService.SessionUserFromRequest(r)
	if !ok {
		httpx.Error(w, r, http.StatusUnauthorized, "not_authenticated", "Sign in is required.")
		return
	}

	conn, err := websocket.Accept(w, r)
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "websocket_upgrade_failed", "Workspace realtime connection could not be opened.")
		return
	}

	client := &workspaceClient{
		user: user,
		conn: conn,
		send: make(chan workspaceSocketEvent, 32),
		done: make(chan struct{}),
	}
	access := workspaceAccessKey(user)
	client.authorize = func() bool {
		current, valid := s.authService.SessionUserFromRequest(r)
		return valid && workspaceAccessKey(current) == access
	}
	s.workspaceEvents.join(client)
	defer func() {
		s.workspaceEvents.leave(client)
		client.close()
	}()

	go client.writePump()

	for {
		var message workspaceSocketMessage
		if err := conn.ReadJSON(&message); err != nil {
			if !errors.Is(err, websocket.ErrClosed) {
				client.close()
			}
			return
		}
		if strings.EqualFold(strings.TrimSpace(message.Type), "ping") {
			if client.authorize() {
				s.authService.Heartbeat(client.user.ID)
			}
			s.workspaceEvents.pong(client)
		}
	}
}

func (h *workspaceEventHub) join(client *workspaceClient) {
	h.mu.Lock()
	h.clients[client] = struct{}{}
	client.revision = 0
	client.enqueue(workspaceSocketEvent{Type: "ready", ServerTime: time.Now().UTC().Format(time.RFC3339)})
	h.mu.Unlock()
}

func (h *workspaceEventHub) leave(client *workspaceClient) {
	h.mu.Lock()
	delete(h.clients, client)
	h.mu.Unlock()
}

func (h *workspaceEventHub) pong(client *workspaceClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, joined := h.clients[client]; !joined {
		return
	}
	client.enqueue(workspaceSocketEvent{
		Type:       "pong",
		Revision:   client.revision,
		ServerTime: time.Now().UTC().Format(time.RFC3339),
	})
}

func (h *workspaceEventHub) publish(changes []WorkspaceChange) {
	h.mu.Lock()
	defer h.mu.Unlock()
	// Keep revision assignment and nonblocking enqueue ordered across publishers.
	for client := range h.clients {
		visible := filterWorkspaceChanges(client.user, changes)
		if len(visible) == 0 {
			continue
		}
		client.revision++
		client.enqueue(workspaceSocketEvent{
			Type:       "workspace.changed",
			Revision:   client.revision,
			ServerTime: time.Now().UTC().Format(time.RFC3339),
			Changes:    visible,
		})
	}
}

func filterWorkspaceChanges(user auth.User, changes []WorkspaceChange) []WorkspaceChange {
	visible := make([]WorkspaceChange, 0, len(changes))
	active := strings.EqualFold(strings.TrimSpace(user.Status), "Active") || strings.TrimSpace(user.Status) == ""
	for _, change := range changes {
		if !active && change.Scope != "users" {
			continue
		}
		if change.CustomerID != "" && !auth.CanAccessCustomer(user, change.CustomerID) {
			continue
		}
		if change.Operation == "upsert" && change.Scope == "notices" {
			var notice CustomerNotice
			if json.Unmarshal(change.Data, &notice) != nil || notice.ID == "" {
				continue
			}
			if !noticeVisibleToUser(notice, user) {
				change.Operation = "remove"
				change.Data, _ = json.Marshal(map[string]string{"id": notice.ID})
			} else if !canManageCustomerNotices(user) {
				notice.CustomerIDs = nil
				change.Data, _ = json.Marshal(notice)
			}
		}
		if change.Operation == "upsert" && change.Scope == "featured" {
			var item FeaturedProduct
			if json.Unmarshal(change.Data, &item) != nil || item.ID == "" {
				continue
			}
			if !item.Active && !canManageCustomerNotices(user) {
				change.Operation = "remove"
				change.Data, _ = json.Marshal(map[string]string{"id": item.ID})
			}
		}
		visible = append(visible, change)
	}
	return visible
}

func (c *workspaceClient) enqueue(event workspaceSocketEvent) {
	select {
	case <-c.done:
		return
	default:
	}
	select {
	case c.send <- event:
	case <-c.done:
	default:
		c.close()
	}
}

func (c *workspaceClient) writePump() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		var event workspaceSocketEvent
		select {
		case event = <-c.send:
		case <-ticker.C:
		case <-c.done:
			return
		}
		event, allowed := c.authorizedEvent(event)
		if event.Type != "" {
			if err := c.conn.WriteJSON(event); err != nil {
				c.close()
				return
			}
		}
		if !allowed {
			c.close()
			return
		}
	}
}

// Check in the writer, not the publisher: auth can publish while holding its lock.
func (c *workspaceClient) authorizedEvent(event workspaceSocketEvent) (workspaceSocketEvent, bool) {
	if c.authorize != nil && c.authorize() {
		return event, true
	}
	return workspaceSocketEvent{
		Type:       "workspace.changed",
		Revision:   event.Revision,
		ServerTime: time.Now().UTC().Format(time.RFC3339),
		Changes:    []WorkspaceChange{{Scope: "users", Operation: "invalidate"}},
	}, false
}

func workspaceAccessKey(user auth.User) string {
	// Ignore presence/profile edits; only authorization changes require reconnecting.
	raw, _ := json.Marshal([]interface{}{
		user.ID, user.TenantID, user.CustomerID, user.Role, user.Status,
		user.AssignedCustomerIDs, user.Permissions, user.PageAccess,
	})
	return string(raw)
}

func (c *workspaceClient) close() {
	c.once.Do(func() {
		close(c.done)
		if c.conn != nil {
			_ = c.conn.Close()
		}
	})
}
