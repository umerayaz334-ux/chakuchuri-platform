package workflow

import (
	"testing"
	"time"
)

func TestCallEventMustDeliverKeepsSignaling(t *testing.T) {
	for _, eventType := range []string{"ready", "signal", "signal-sent", "signals", "call-ended", "error", "SIGNAL"} {
		if !callEventMustDeliver(eventType) {
			t.Fatalf("%s must be delivered", eventType)
		}
	}
	if callEventMustDeliver("presence") {
		t.Fatal("presence may be dropped under backpressure")
	}
}

func TestCallClientEnqueueDoesNotDropSignals(t *testing.T) {
	client := &callClient{
		send: make(chan callSocketEvent, 1),
		done: make(chan struct{}),
	}
	client.enqueue(callSocketEvent{Type: "presence"})

	done := make(chan struct{})
	go func() {
		client.enqueue(callSocketEvent{Type: "signal"})
		close(done)
	}()

	time.Sleep(25 * time.Millisecond)
	select {
	case <-done:
		t.Fatal("signal enqueue should block while the outbound buffer is full")
	default:
	}

	if event := <-client.send; event.Type != "presence" {
		t.Fatalf("expected presence first, got %s", event.Type)
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("signal enqueue did not complete after buffer space opened")
	}

	if event := <-client.send; event.Type != "signal" {
		t.Fatalf("expected signal, got %s", event.Type)
	}
}
