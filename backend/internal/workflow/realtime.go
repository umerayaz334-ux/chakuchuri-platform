package workflow

import (
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"chakuchuri/backend/internal/auth"
	"chakuchuri/backend/internal/platform/httpx"
	"chakuchuri/backend/internal/platform/websocket"
)

type callSocketMessage struct {
	Type       string                 `json:"type"`
	SignalType string                 `json:"signalType,omitempty"`
	Payload    map[string]interface{} `json:"payload,omitempty"`
	After      int                    `json:"after,omitempty"`
}

type callSocketEvent struct {
	Type       string       `json:"type"`
	Mode       string       `json:"mode,omitempty"`
	Message    string       `json:"message,omitempty"`
	ServerTime string       `json:"serverTime,omitempty"`
	Call       *CallRequest `json:"call,omitempty"`
	Signal     *CallSignal  `json:"signal,omitempty"`
	Signals    []CallSignal `json:"signals,omitempty"`
}

type callHub struct {
	mu      sync.Mutex
	clients map[string]map[*callClient]struct{}
}

type callClient struct {
	callID string
	user   auth.User
	conn   *websocket.Conn
	send   chan callSocketEvent
	done   chan struct{}
	once   sync.Once
}

var realtimeCallHub = &callHub{clients: map[string]map[*callClient]struct{}{}}

func (s *Service) handleCallSocket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpx.Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Realtime calls require a WebSocket GET request.")
		return
	}

	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if r.Header.Get("Authorization") == "" && token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	user, ok := s.authService.UserFromRequest(r)
	if !ok {
		httpx.Error(w, r, http.StatusUnauthorized, "not_authenticated", "Sign in is required.")
		return
	}
	if !auth.HasAnyPermission(user, "calls.start", "calls.manage") {
		httpx.Error(w, r, http.StatusForbidden, "forbidden", "You do not have access to realtime calls.")
		return
	}

	callID := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/realtime/calls/"), "/")
	call, err := s.CallForUser(callID, user)
	if err != nil {
		writeResult(w, r, nil, err)
		return
	}
	if call.Status != "In call" {
		httpx.Error(w, r, http.StatusBadRequest, "call_not_started", "Support must start this call before the realtime room opens.")
		return
	}

	conn, err := websocket.Accept(w, r)
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "websocket_upgrade_failed", "Realtime connection could not be opened.")
		return
	}

	client := &callClient{
		callID: call.ID,
		user:   user,
		conn:   conn,
		send:   make(chan callSocketEvent, 256),
		done:   make(chan struct{}),
	}
	realtimeCallHub.join(client)
	defer func() {
		realtimeCallHub.leave(client)
		client.close()
	}()

	go client.writePump()
	client.enqueue(callSocketEvent{Type: "ready", Mode: "websocket", ServerTime: time.Now().UTC().Format(time.RFC3339), Call: &call})
	if signals, err := s.ListCallSignals(call.ID, 0, user); err == nil && len(signals) > 0 {
		client.enqueue(callSocketEvent{Type: "signals", Mode: "websocket", ServerTime: time.Now().UTC().Format(time.RFC3339), Signals: signals})
	}

	for {
		var message callSocketMessage
		if err := conn.ReadJSON(&message); err != nil {
			if !errors.Is(err, websocket.ErrClosed) {
				client.enqueue(callSocketEvent{Type: "error", Message: "Realtime call connection closed."})
			}
			return
		}
		if !s.handleCallSocketMessage(client, message) {
			return
		}
	}
}

func (s *Service) handleCallSocketMessage(client *callClient, message callSocketMessage) bool {
	switch strings.ToLower(strings.TrimSpace(message.Type)) {
	case "signal":
		signal, err := s.SendCallSignal(client.callID, CallSignalRequest{SignalType: message.SignalType, Payload: message.Payload}, client.user)
		if err != nil {
			client.enqueue(callSocketEvent{Type: "error", Message: realtimeErrorMessage(err)})
			return true
		}
		event := callSocketEvent{Type: "signal", Mode: "websocket", ServerTime: time.Now().UTC().Format(time.RFC3339), Signal: &signal}
		realtimeCallHub.broadcast(client.callID, event, client)
		client.enqueue(callSocketEvent{Type: "signal-sent", Mode: "websocket", ServerTime: event.ServerTime, Signal: &signal})
		return true
	case "heartbeat":
		call, err := s.HeartbeatCall(client.callID, client.user)
		if err != nil {
			client.enqueue(callSocketEvent{Type: "error", Message: realtimeErrorMessage(err)})
			return true
		}
		event := callSocketEvent{Type: "presence", Mode: "websocket", ServerTime: time.Now().UTC().Format(time.RFC3339), Call: &call}
		realtimeCallHub.broadcast(client.callID, event, nil)
		return true
	case "catch-up":
		signals, err := s.ListCallSignals(client.callID, message.After, client.user)
		if err != nil {
			client.enqueue(callSocketEvent{Type: "error", Message: realtimeErrorMessage(err)})
			return true
		}
		client.enqueue(callSocketEvent{Type: "signals", Mode: "websocket", ServerTime: time.Now().UTC().Format(time.RFC3339), Signals: signals})
		return true
	case "end":
		if _, err := s.EndCall(client.callID, client.user); err != nil {
			client.enqueue(callSocketEvent{Type: "error", Message: realtimeErrorMessage(err)})
			return true
		}
		// EndCall already fans call-ended to the room (including HTTP hang-ups).
		return false
	default:
		client.enqueue(callSocketEvent{Type: "error", Message: "Realtime message type is not supported."})
		return true
	}
}

func (s *Service) CallForUser(id string, actor auth.User) (CallRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	index, call, ok := s.findCallLocked(id)
	if !ok {
		return CallRequest{}, errNotFound
	}
	if !canUseCall(actor, call) {
		return CallRequest{}, errForbidden
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if auth.IsCustomerRole(actor) {
		call.CustomerLastSeenAt = now
	} else {
		call.AdminLastSeenAt = now
	}
	s.calls[index] = call
	return call, nil
}

func (h *callHub) join(client *callClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[client.callID] == nil {
		h.clients[client.callID] = map[*callClient]struct{}{}
	}
	h.clients[client.callID][client] = struct{}{}
}

func (h *callHub) leave(client *callClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	clients := h.clients[client.callID]
	if clients == nil {
		return
	}
	delete(clients, client)
	if len(clients) == 0 {
		delete(h.clients, client.callID)
	}
}

func (h *callHub) broadcast(callID string, event callSocketEvent, skip *callClient) {
	h.mu.Lock()
	clients := make([]*callClient, 0, len(h.clients[callID]))
	for client := range h.clients[callID] {
		if client != skip {
			clients = append(clients, client)
		}
	}
	h.mu.Unlock()
	for _, client := range clients {
		client.enqueue(event)
	}
}

func notifyCallRoomEnded(call CallRequest) {
	copy := call
	realtimeCallHub.broadcast(call.ID, callSocketEvent{
		Type:       "call-ended",
		Mode:       "websocket",
		ServerTime: time.Now().UTC().Format(time.RFC3339),
		Call:       &copy,
	}, nil)
}

func (c *callClient) enqueue(event callSocketEvent) {
	select {
	case <-c.done:
		return
	default:
	}
	if callEventMustDeliver(event.Type) {
		// Never drop ICE / SDP / end events — backpressure until written or closed.
		select {
		case c.send <- event:
		case <-c.done:
		}
		return
	}
	select {
	case c.send <- event:
	case <-c.done:
	default:
		// Presence heartbeats may be skipped under load.
	}
}

func callEventMustDeliver(eventType string) bool {
	switch strings.ToLower(strings.TrimSpace(eventType)) {
	case "ready", "signal", "signal-sent", "signals", "call-ended", "error":
		return true
	default:
		return false
	}
}

func (c *callClient) writePump() {
	for {
		select {
		case event := <-c.send:
			if err := c.conn.WriteJSON(event); err != nil {
				c.close()
				return
			}
		case <-c.done:
			return
		}
	}
}

func (c *callClient) close() {
	c.once.Do(func() {
		close(c.done)
		_ = c.conn.Close()
	})
}

func realtimeErrorMessage(err error) string {
	switch err {
	case nil:
		return ""
	case errForbidden:
		return "You do not have access to this call."
	case errNotFound:
		return "This call room was not found."
	default:
		return "Realtime call action could not be completed."
	}
}
