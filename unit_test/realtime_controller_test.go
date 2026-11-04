package controllers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"lechat-backend/internal/api/controllers"
	"lechat-backend/internal/database/dto"
	"lechat-backend/internal/database/models"
	"lechat-backend/internal/interfaces"
	"lechat-backend/internal/request"
	"lechat-backend/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
)

type realtimeChatStub struct {
	interfaces.IChat
}

func (realtimeChatStub) ListMessages(uint, string, string, int) ([]dto.MessageDto, error) {
	return []dto.MessageDto{{ID: "message-id"}}, nil
}

func (realtimeChatStub) SendMessage(userID uint, conversationID string, req request.SendMessageRequest) (dto.MessageDto, error) {
	return dto.MessageDto{
		ID:              "message-id",
		ConversationID:  conversationID,
		SenderID:        int64(userID),
		ClientMessageID: req.ClientMessageID,
		Type:            req.Type,
		Body:            req.Body,
		CreatedAt:       time.Now(),
	}, nil
}

func (realtimeChatStub) ActiveMemberIDs(string) ([]uint, error) {
	return []uint{7}, nil
}

type receiptChatStub struct {
	realtimeChatStub
}

func (receiptChatStub) MarkRead(uint, string, request.MarkReadRequest) (dto.ReadReceiptDto, error) {
	return dto.ReadReceiptDto{
		ConversationID:   "123e4567-e89b-12d3-a456-426614174000",
		ThroughMessageID: "123e4567-e89b-12d3-a456-426614174001",
		SenderIDs:        []uint{8},
	}, nil
}

type realtimeJWTStub struct {
	interfaces.IJwtInterface
}

func (realtimeJWTStub) ValidateToken(string) (*models.JwtClaims, error) {
	return &models.JwtClaims{
		UserId:    7,
		TokenType: models.TokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
		},
	}, nil
}

type receiptJWTStub struct {
	interfaces.IJwtInterface
}

func (receiptJWTStub) ValidateToken(token string) (*models.JwtClaims, error) {
	userID := uint(7)
	if token == "sender-token" {
		userID = 8
	}
	return &models.JwtClaims{
		UserId:    userID,
		TokenType: models.TokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
		},
	}, nil
}

func TestRealtimeSocketAuthenticatesAndBroadcastsMessages(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	hub := services.NewRealtimeHub()
	controller := controllers.NewRealtimeController(
		realtimeChatStub{},
		realtimeJWTStub{},
		hub,
		[]string{"http://localhost"},
	)
	router.GET("/api/v1/ws", controller.Serve)

	server := httptest.NewServer(router)
	defer server.Close()

	header := http.Header{"Origin": []string{"http://localhost"}}
	conn, _, err := websocket.DefaultDialer.Dial("ws"+server.URL[len("http"):]+"/api/v1/ws", header)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}

	defer conn.Close()

	if err := conn.WriteJSON(map[string]string{"type": "auth", "token": "test-token"}); err != nil {
		t.Fatalf("send auth: %v", err)
	}
	var event map[string]interface{}
	if err := conn.ReadJSON(&event); err != nil {
		t.Fatalf("read auth response: %v", err)
	}
	if event["type"] != "auth.ok" {
		t.Fatalf("auth event type = %v, want auth.ok", event["type"])
	}

	conversationID := "123e4567-e89b-12d3-a456-426614174000"
	if err := conn.WriteJSON(map[string]interface{}{
		"type":            "conversation.join",
		"conversation_id": conversationID,
	}); err != nil {
		t.Fatalf("send join: %v", err)
	}
	if err := conn.ReadJSON(&event); err != nil {
		t.Fatalf("read history: %v", err)
	}
	if event["type"] != "conversation.history" {
		t.Fatalf("history event type = %v, want conversation.history", event["type"])
	}

	if err := conn.WriteJSON(map[string]interface{}{
		"type":              "message.send",
		"conversation_id":   conversationID,
		"client_message_id": "123e4567-e89b-12d3-a456-426614174001",
		"message_type":      "text",
		"body":              "hello",
	}); err != nil {
		t.Fatalf("send message: %v", err)
	}
	if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	if err := conn.ReadJSON(&event); err != nil {
		t.Fatalf("read message event: %v", err)
	}
	if event["type"] != "message.created" {
		t.Fatalf("message event type = %v, want message.created", event["type"])
	}
}

func TestReadReceiptIsSentToMessageSenderOutsideJoinedRoom(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	hub := services.NewRealtimeHub()
	controller := controllers.NewRealtimeController(
		receiptChatStub{},
		receiptJWTStub{},
		hub,
		[]string{"http://localhost"},
	)
	router.GET("/api/v1/ws", controller.Serve)

	server := httptest.NewServer(router)
	defer server.Close()

	dial := func(token string) *websocket.Conn {
		t.Helper()
		header := http.Header{"Origin": []string{"http://localhost"}}
		conn, _, err := websocket.DefaultDialer.Dial("ws"+server.URL[len("http"):]+"/api/v1/ws", header)
		if err != nil {
			t.Fatalf("dial websocket: %v", err)
		}
		if err := conn.WriteJSON(map[string]string{"type": "auth", "token": token}); err != nil {
			t.Fatalf("send auth: %v", err)
		}
		var event map[string]interface{}
		if err := conn.ReadJSON(&event); err != nil {
			t.Fatalf("read auth response: %v", err)
		}
		if event["type"] != "auth.ok" {
			t.Fatalf("auth event type = %v, want auth.ok", event["type"])
		}
		return conn
	}

	reader := dial("reader-token")
	defer reader.Close()
	sender := dial("sender-token")
	defer sender.Close()

	conversationID := "123e4567-e89b-12d3-a456-426614174000"
	if err := reader.WriteJSON(map[string]interface{}{
		"type":            "conversation.join",
		"conversation_id": conversationID,
	}); err != nil {
		t.Fatalf("send join: %v", err)
	}
	var event map[string]interface{}
	if err := reader.ReadJSON(&event); err != nil {
		t.Fatalf("read history: %v", err)
	}
	if event["type"] != "conversation.history" {
		t.Fatalf("history event type = %v, want conversation.history", event["type"])
	}

	if err := reader.WriteJSON(map[string]interface{}{
		"type":               "receipt.read",
		"conversation_id":    conversationID,
		"through_message_id": "123e4567-e89b-12d3-a456-426614174001",
	}); err != nil {
		t.Fatalf("send read receipt: %v", err)
	}
	if err := sender.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("set sender read deadline: %v", err)
	}
	if err := sender.ReadJSON(&event); err != nil {
		t.Fatalf("read receipt event: %v", err)
	}
	if event["type"] != "receipt.read" || event["conversation_id"] != conversationID {
		t.Fatalf("unexpected receipt event: %#v", event)
	}
	data, ok := event["data"].(map[string]interface{})
	if !ok || data["through_message_id"] != "123e4567-e89b-12d3-a456-426614174001" {
		t.Fatalf("unexpected receipt data: %#v", event["data"])
	}
}
