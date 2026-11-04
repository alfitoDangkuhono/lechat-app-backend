package controllers_test

import (
	"testing"

	"lechat-backend/internal/constant"
	"lechat-backend/internal/request"
	"lechat-backend/internal/services"
)

func TestCreateConversationRejectsInvalidParticipants(t *testing.T) {
	service := services.NewChatService(nil)
	tests := []request.CreateConversationRequest{
		{Type: "unknown", MemberIDs: []uint{2}},
		{Type: "direct", MemberIDs: nil},
		{Type: "direct", MemberIDs: []uint{1}},
		{Type: "group", MemberIDs: nil},
		{Type: "group", MemberIDs: []uint{1, 2}},
	}

	for _, req := range tests {
		if _, err := service.CreateConversation(1, req); err != constant.ErrChatInvalidRequest {
			t.Errorf("CreateConversation(%+v) error = %v, want %v", req, err, constant.ErrChatInvalidRequest)
		}
	}
}

func TestChatServiceValidatesMessageAndPageBounds(t *testing.T) {
	service := services.NewChatService(nil)

	if _, err := service.ListMessages(1, "conversation", "", 0); err != constant.ErrChatInvalidRequest {
		t.Errorf("ListMessages with zero limit error = %v, want %v", err, constant.ErrChatInvalidRequest)
	}
	if _, err := service.ListMessages(1, "conversation", "", 101); err != constant.ErrChatInvalidRequest {
		t.Errorf("ListMessages over max limit error = %v, want %v", err, constant.ErrChatInvalidRequest)
	}
	if _, err := service.SendMessage(1, "conversation", request.SendMessageRequest{Type: "text"}); err != constant.ErrChatInvalidRequest {
		t.Errorf("SendMessage with empty text error = %v, want %v", err, constant.ErrChatInvalidRequest)
	}
	invalidReplyID := "not-a-uuid"
	if _, err := service.SendMessage(1, "conversation", request.SendMessageRequest{
		Type:             "text",
		Body:             "reply",
		ReplyToMessageID: &invalidReplyID,
	}); err != constant.ErrChatInvalidReply {
		t.Errorf("SendMessage with invalid reply ID error = %v, want %v", err, constant.ErrChatInvalidReply)
	}
	if _, err := service.InviteGroupMember(1, "conversation", request.InviteGroupMemberRequest{}); err != constant.ErrChatInvalidRequest {
		t.Errorf("InviteGroupMember with zero user ID error = %v, want %v", err, constant.ErrChatInvalidRequest)
	}
	if _, err := service.RespondToGroupInvitation(1, "invalid-id", true); err != constant.ErrChatInvalidRequest {
		t.Errorf("RespondToGroupInvitation with invalid ID error = %v, want %v", err, constant.ErrChatInvalidRequest)
	}
}
