package repository

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"lechat-backend/internal/database/dto"
)

func TestFinalizeMessageEncodesIVAsStandardBase64(t *testing.T) {
	raw := []byte{0xFF, 0x00, 0x24, 0x81, 0xFE, 0x6C, 0x08, 0x65, 0x39, 0x32, 0x54, 0x00}
	message := dto.MessageDto{ID: "message-id", IVBytes: raw}

	finalizeMessage(&message)
	if message.IV == nil {
		t.Fatal("finalizeMessage must populate IV")
	}

	if string(message.IVBytes) == *message.IV {
		t.Fatal("IV must be base64 text, not the raw byte string")
	}

	decoded, err := base64.StdEncoding.DecodeString(*message.IV)
	if err != nil {
		t.Fatalf("IV must be valid standard base64: %v", err)
	}
	if string(decoded) != string(raw) {
		t.Fatal("IV base64 must round-trip to the original bytes")
	}

	payload, err := json.Marshal(message)
	if err != nil {
		t.Fatalf("marshal message: %v", err)
	}
	if strings.Contains(string(payload), "\ufffd") {
		t.Fatalf("serialized message must not contain U+FFFD replacement characters: %s", payload)
	}
	var result struct {
		IV string `json:"iv"`
	}
	if err := json.Unmarshal(payload, &result); err != nil {
		t.Fatalf("unmarshal message: %v", err)
	}
	if decodedAgain, err := base64.StdEncoding.DecodeString(result.IV); err != nil || string(decodedAgain) != string(raw) {
		t.Fatalf("json iv field must be standard base64 of the original bytes, got %q", result.IV)
	}
}
