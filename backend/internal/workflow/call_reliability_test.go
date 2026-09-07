package workflow

import "testing"

func TestReconnectRequestIsAValidCallSignal(t *testing.T) {
	if !validSignalType("reconnect-request") {
		t.Fatal("reconnect-request must be accepted for automatic ICE recovery")
	}
}
