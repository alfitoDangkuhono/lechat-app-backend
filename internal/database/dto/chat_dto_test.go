package dto

import (
	"encoding/json"
	"testing"
)

func TestMessageDtoMarshalsReplySummary(t *testing.T) {
	replyID := "123e4567-e89b-12d3-a456-426614174003"
	senderID := int64(19)
	username := "andi"
	body := "Original message"
	messageType := "text"
	isDeleted := false
	message, err := json.Marshal(MessageDto{
		ID:               "123e4567-e89b-12d3-a456-426614174004",
		ReplyToMessageID: &replyID,
		ReplyTo: &MessageReplyDto{
			ID:        replyID,
			SenderID:  19,
			Username:  "andi",
			Body:      "Original message",
			Type:      "text",
			IsDeleted: false,
		},
		ReplyToSenderID:  &senderID,
		ReplyToUsername:  &username,
		ReplyToBody:      &body,
		ReplyToType:      &messageType,
		ReplyToIsDeleted: &isDeleted,
	})
	if err != nil {
		t.Fatalf("marshal message DTO: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(message, &result); err != nil {
		t.Fatalf("unmarshal message DTO: %v", err)
	}
	reply, ok := result["reply_to"].(map[string]interface{})
	if !ok {
		t.Fatalf("reply_to = %T, want object", result["reply_to"])
	}
	if reply["id"] != replyID || reply["username"] != "andi" || reply["is_deleted"] != false {
		t.Fatalf("unexpected reply summary: %#v", reply)
	}
	if _, ok := result["reply_to_body"]; ok {
		t.Fatal("internal reply summary fields must not be serialized")
	}
}
