package config

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"strings"
	"time"
)

// WebRTC holds STUN/TURN settings used by CallConfig for peer connections.
type WebRTC struct {
	StunURLs           []string
	TurnURLs           []string
	TurnUsername       string
	TurnCredential     string
	TurnSharedSecret   string
	TurnCredentialTTL  time.Duration
}

func (cfg Config) WebRTC() WebRTC {
	stun := splitCSV(value("CC_WEBRTC_STUN_URLS", "stun:stun.l.google.com:19302,stun:stun1.l.google.com:19302"))
	turn := splitCSV(value("CC_TURN_URLS", ""))
	ttlSeconds := intValue("CC_TURN_CREDENTIAL_TTL", 86400)
	return WebRTC{
		StunURLs:          stun,
		TurnURLs:          turn,
		TurnUsername:      strings.TrimSpace(value("CC_TURN_USERNAME", "")),
		TurnCredential:    strings.TrimSpace(value("CC_TURN_CREDENTIAL", "")),
		TurnSharedSecret:  strings.TrimSpace(value("CC_TURN_SHARED_SECRET", "")),
		TurnCredentialTTL: time.Duration(ttlSeconds) * time.Second,
	}
}

// TurnConfigured reports whether at least one TURN URL is set with credentials.
func (w WebRTC) TurnConfigured() bool {
	if len(w.TurnURLs) == 0 {
		return false
	}
	if w.TurnSharedSecret != "" {
		return true
	}
	return w.TurnUsername != "" && w.TurnCredential != ""
}

// IceServers builds browser/Flutter-compatible RTCIceServer maps for the given user.
// Prefer coturn-style time-limited credentials when CC_TURN_SHARED_SECRET is set.
func (w WebRTC) IceServers(userID string) []map[string]interface{} {
	servers := make([]map[string]interface{}, 0, 2)
	if len(w.StunURLs) > 0 {
		servers = append(servers, map[string]interface{}{
			"urls": cloneStrings(w.StunURLs),
		})
	}
	if !w.TurnConfigured() {
		return servers
	}

	turn := map[string]interface{}{
		"urls": cloneStrings(w.TurnURLs),
	}
	if w.TurnSharedSecret != "" {
		username, credential := mintTurnCredentials(w.TurnSharedSecret, userID, w.TurnCredentialTTL)
		turn["username"] = username
		turn["credential"] = credential
	} else {
		turn["username"] = w.TurnUsername
		turn["credential"] = w.TurnCredential
	}
	servers = append(servers, turn)
	return servers
}

func mintTurnCredentials(secret, userID string, ttl time.Duration) (username, credential string) {
	if ttl < time.Minute {
		ttl = 24 * time.Hour
	}
	safeUser := sanitizeTurnUser(userID)
	expiry := time.Now().UTC().Add(ttl).Unix()
	username = fmt.Sprintf("%d:%s", expiry, safeUser)
	mac := hmac.New(sha1.New, []byte(secret))
	_, _ = mac.Write([]byte(username))
	credential = base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return username, credential
}

func sanitizeTurnUser(userID string) string {
	cleaned := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			return r
		default:
			return '_'
		}
	}, strings.TrimSpace(userID))
	if cleaned == "" {
		return "call"
	}
	if len(cleaned) > 64 {
		return cleaned[:64]
	}
	return cleaned
}

func splitCSV(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n'
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func cloneStrings(values []string) []string {
	out := make([]string, len(values))
	copy(out, values)
	return out
}
