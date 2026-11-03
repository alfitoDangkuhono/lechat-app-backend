package services

import (
	"encoding/json"
	"fmt"
	"sync"
)

type RealtimeHub struct {
	mu      sync.RWMutex
	clients map[*RealtimeClient]struct{}
	rooms   map[string]map[*RealtimeClient]struct{}
}

type RealtimeClient struct {
	userID uint
	send   chan []byte
	done   chan struct{}
	once   sync.Once
	rooms  map[string]struct{}
}

func NewRealtimeHub() *RealtimeHub {
	return &RealtimeHub{
		clients: make(map[*RealtimeClient]struct{}),
		rooms:   make(map[string]map[*RealtimeClient]struct{}),
	}
}

func (h *RealtimeHub) Register(userID uint) *RealtimeClient {
	client := &RealtimeClient{
		userID: userID,
		send:   make(chan []byte, 64),
		done:   make(chan struct{}),
		rooms:  make(map[string]struct{}),
	}
	h.mu.Lock()
	h.clients[client] = struct{}{}
	h.mu.Unlock()
	return client
}

func (h *RealtimeHub) Unregister(client *RealtimeClient) {
	h.mu.Lock()
	delete(h.clients, client)
	for room := range client.rooms {
		delete(h.rooms[room], client)
		if len(h.rooms[room]) == 0 {
			delete(h.rooms, room)
		}
	}
	h.mu.Unlock()
	client.stop()
}

func (h *RealtimeHub) Join(client *RealtimeClient, conversationID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[client]; !ok {
		return
	}
	if _, ok := client.rooms[conversationID]; ok {
		return
	}
	room := h.rooms[conversationID]
	if room == nil {
		room = make(map[*RealtimeClient]struct{})
		h.rooms[conversationID] = room
	}
	room[client] = struct{}{}
	client.rooms[conversationID] = struct{}{}
}

func (h *RealtimeHub) Leave(client *RealtimeClient, conversationID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(client.rooms, conversationID)
	delete(h.rooms[conversationID], client)
	if len(h.rooms[conversationID]) == 0 {
		delete(h.rooms, conversationID)
	}
}

func (h *RealtimeHub) IsJoined(client *RealtimeClient, conversationID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, joined := client.rooms[conversationID]
	return joined
}

func (h *RealtimeHub) Send(client *RealtimeClient, event interface{}) bool {
	payload, err := json.Marshal(event)
	if err != nil {
		return false
	}
	select {
	case <-client.done:
		return false
	case client.send <- payload:
		return true
	default:
		client.stop()
		return false
	}
}

func (h *RealtimeHub) Broadcast(conversationID string, allowedUserIDs []uint, event interface{}) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal realtime event: %w", err)
	}
	allowed := make(map[uint]struct{}, len(allowedUserIDs))
	for _, userID := range allowedUserIDs {
		allowed[userID] = struct{}{}
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for client := range h.rooms[conversationID] {
		if _, ok := allowed[client.userID]; !ok {
			continue
		}
		select {
		case <-client.done:
		case client.send <- payload:
		default:
			client.stop()
		}
	}
	return nil
}

func (c *RealtimeClient) SendQueue() <-chan []byte {
	return c.send
}

func (c *RealtimeClient) Done() <-chan struct{} {
	return c.done
}

func (c *RealtimeClient) stop() {
	c.once.Do(func() {
		close(c.done)
	})
}
