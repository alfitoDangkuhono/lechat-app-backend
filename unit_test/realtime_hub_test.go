package controllers_test

import (
	"encoding/json"
	"lechat-backend/internal/services"
	"testing"
	"time"
)

func TestRealtimeHubBroadcastsOnlyToActiveSubscribers(t *testing.T) {
	hub := services.NewRealtimeHub()
	first := hub.Register(1)
	second := hub.Register(2)
	hub.Join(first, "conversation")
	hub.Join(second, "conversation")

	hub.Broadcast("conversation", []uint{1}, map[string]string{"type": "message.created"})

	select {
	case payload := <-first.SendQueue():
		var event map[string]string
		if err := json.Unmarshal(payload, &event); err != nil {
			t.Fatalf("unmarshal first event: %v", err)
		}
		if event["type"] != "message.created" {
			t.Fatalf("first event type = %q, want message.created", event["type"])
		}
	case <-time.After(time.Second):
		t.Fatal("active subscriber did not receive event")
	}

	select {
	case <-second.SendQueue():
		t.Fatal("inactive subscriber received event")
	default:
	}

	hub.Unregister(first)
	hub.Unregister(second)
}
