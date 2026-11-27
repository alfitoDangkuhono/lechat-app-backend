package controllers_test

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"lechat-backend/internal/api/controllers"
	"lechat-backend/internal/api/middleware"
	"lechat-backend/internal/constant"
	"lechat-backend/internal/database/models"
	"lechat-backend/internal/interfaces"
	"lechat-backend/internal/request"
	"lechat-backend/internal/services"

	"github.com/gin-gonic/gin"
)

type groupKeyChatStub struct {
	interfaces.IChat
	uploadErr error
}

func (s *groupKeyChatStub) UploadGroupKeys(uint, string, request.UploadGroupKeysRequest) error {
	return s.uploadErr
}

func (s *groupKeyChatStub) ActiveMemberIDs(string) ([]uint, error) {
	return []uint{7, 2}, nil
}

func TestUploadGroupKeysBroadcastsGroupKeyCreated(t *testing.T) {
	gin.SetMode(gin.TestMode)
	hub := services.NewRealtimeHub()
	memberClient := hub.Register(2)
	stub := &groupKeyChatStub{}
	controller := controllers.NewChatController(stub, hub)

	body := `{"key_version": 1, "keys": [{"user_id": 2, "encrypted_key": "QUJDREVGR0hJSktMTU5PUFFSU1RVVldYWVphYmNkZWZnaGlqa2xtbm9wcXJzdHV2d3h5eg==", "iv": "QUJDREVGR0hJSktM"}]}`
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("POST", "/api/v1/conversations/123e4567-e89b-12d3-a456-426614174000/keys", bytes.NewBufferString(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Params = gin.Params{{Key: "conversation_id", Value: "123e4567-e89b-12d3-a456-426614174000"}}
	ctx.Set(middleware.AuthenticatedClaimsKey, &models.JwtClaims{UserId: 7})

	controller.UploadGroupKeys(ctx)

	if recorder.Code != 200 {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	select {
	case payload := <-memberClient.SendQueue():
		var event map[string]interface{}
		if err := json.Unmarshal(payload, &event); err != nil {
			t.Fatalf("unmarshal broadcast: %v", err)
		}
		if event["type"] != "group_key.created" {
			t.Fatalf("event type = %v, want group_key.created", event["type"])
		}
		if event["conversation_id"] != "123e4567-e89b-12d3-a456-426614174000" {
			t.Fatalf("conversation_id = %v", event["conversation_id"])
		}
		data, ok := event["data"].(map[string]interface{})
		if !ok {
			t.Fatalf("data = %T, want object", event["data"])
		}
		if data["key_version"] != float64(1) {
			t.Fatalf("key_version = %v, want 1", data["key_version"])
		}
		if data["sender_id"] != float64(7) {
			t.Fatalf("sender_id = %v, want 7", data["sender_id"])
		}
	case <-time.After(time.Second):
		t.Fatal("member must receive group_key.created event")
	}
}

func TestUploadGroupKeysFailureDoesNotBroadcast(t *testing.T) {
	gin.SetMode(gin.TestMode)
	hub := services.NewRealtimeHub()
	memberClient := hub.Register(2)
	stub := &groupKeyChatStub{uploadErr: constant.ErrChatForbidden}
	controller := controllers.NewChatController(stub, hub)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("POST", "/api/v1/conversations/123e4567-e89b-12d3-a456-426614174000/keys", bytes.NewBufferString(`{"key_version": 1, "keys": [{"user_id": 2, "encrypted_key": "QUJDREVGR0hJSktMTU5PUFFSU1RVVldYWVphYmNkZWZnaGlqa2xtbm9wcXJzdHV2d3h5eg==", "iv": "QUJDREVGR0hJSktM"}]}`))
	ctx.Params = gin.Params{{Key: "conversation_id", Value: "123e4567-e89b-12d3-a456-426614174000"}}
	ctx.Set(middleware.AuthenticatedClaimsKey, &models.JwtClaims{UserId: 7})

	controller.UploadGroupKeys(ctx)

	if recorder.Code != 403 {
		t.Fatalf("status = %d, want 403", recorder.Code)
	}
	select {
	case payload := <-memberClient.SendQueue():
		t.Fatalf("no broadcast expected on failure, got %s", payload)
	case <-time.After(100 * time.Millisecond):
	}
}
