package config

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestIceServersStunOnlyByDefault(t *testing.T) {
	w := WebRTC{
		StunURLs: []string{"stun:stun.l.google.com:19302", "stun:stun1.l.google.com:19302"},
	}
	servers := w.IceServers("user_1")
	if len(servers) != 1 {
		t.Fatalf("expected stun-only server list, got %#v", servers)
	}
	urls, _ := servers[0]["urls"].([]string)
	if len(urls) != 2 {
		t.Fatalf("expected two stun urls, got %#v", urls)
	}
	if w.TurnConfigured() {
		t.Fatal("turn should not be configured without urls/credentials")
	}
}

func TestIceServersStaticTurn(t *testing.T) {
	w := WebRTC{
		StunURLs:       []string{"stun:stun.l.google.com:19302"},
		TurnURLs:       []string{"turn:turn.example:3478?transport=udp", "turns:turn.example:443?transport=tcp"},
		TurnUsername:   "static-user",
		TurnCredential: "static-pass",
	}
	if !w.TurnConfigured() {
		t.Fatal("expected turn configured")
	}
	servers := w.IceServers("user_1")
	if len(servers) != 2 {
		t.Fatalf("expected stun+turn, got %#v", servers)
	}
	turn := servers[1]
	if turn["username"] != "static-user" || turn["credential"] != "static-pass" {
		t.Fatalf("unexpected static credentials: %#v", turn)
	}
	urls, _ := turn["urls"].([]string)
	if len(urls) != 2 {
		t.Fatalf("expected two turn urls, got %#v", urls)
	}
}

func TestIceServersSharedSecretCredentials(t *testing.T) {
	secret := "dev-turn-secret"
	w := WebRTC{
		StunURLs:          []string{"stun:stun.l.google.com:19302"},
		TurnURLs:          []string{"turn:127.0.0.1:3478"},
		TurnSharedSecret:  secret,
		TurnCredentialTTL: time.Hour,
	}
	servers := w.IceServers("user:abc/1")
	turn := servers[1]
	username, _ := turn["username"].(string)
	credential, _ := turn["credential"].(string)
	if !strings.Contains(username, ":user_abc_1") {
		t.Fatalf("username should include sanitized user id, got %q", username)
	}
	mac := hmac.New(sha1.New, []byte(secret))
	_, _ = mac.Write([]byte(username))
	want := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	if credential != want {
		t.Fatalf("credential mismatch: got %q want %q", credential, want)
	}
}

func TestSplitCSV(t *testing.T) {
	got := splitCSV(" a,b ; c\n d ")
	if len(got) != 4 || got[0] != "a" || got[3] != "d" {
		t.Fatalf("unexpected split: %#v", got)
	}
}
