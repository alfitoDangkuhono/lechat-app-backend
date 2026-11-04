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

func TestRealtimeHubBroadcastsToAllConnectionsOfAllowedUsers(t *testing.T) {
	hub := services.NewRealtimeHub()
	conversationSubscriber := hub.Register(1)
	otherConnection := hub.Register(1)
	unrelatedUser := hub.Register(2)
	hub.Join(conversationSubscriber, "conversation")

	if err := hub.BroadcastUsers([]uint{1}, map[string]string{"type": "message.created"}); err != nil {
		t.Fatalf("broadcast to users: %v", err)
	}

	for name, client := range map[string]*services.RealtimeClient{
		"joined connection":     conversationSubscriber,
		"other user connection": otherConnection,
	} {
		select {
		case payload := <-client.SendQueue():
			var event map[string]string
			if err := json.Unmarshal(payload, &event); err != nil {
				t.Fatalf("unmarshal %s event: %v", name, err)
			}
			if event["type"] != "message.created" {
				t.Fatalf("%s event type = %q, want message.created", name, event["type"])
			}
		case <-time.After(time.Second):
			t.Fatalf("%s did not receive event", name)
		}
	}

	select {
	case <-unrelatedUser.SendQueue():
		t.Fatal("unrelated user received event")
	default:
	}

	hub.Unregister(conversationSubscriber)
	hub.Unregister(otherConnection)
	hub.Unregister(unrelatedUser)
}
