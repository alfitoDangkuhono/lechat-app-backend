package services

import (
	"encoding/base64"
	"strings"
	"testing"

	"lechat-backend/internal/constant"
	"lechat-backend/internal/request"
)

func encryptedRequest(ciphertext, iv string) request.SendMessageRequest {
	version := 1
	return request.SendMessageRequest{
		Type:              "text",
		EncryptionVersion: &version,
		Ciphertext:        &ciphertext,
		IV:                &iv,
	}
}

func TestValidateDirectEncryptedMessageAcceptsCanonicalBase64(t *testing.T) {
	ciphertext := base64.StdEncoding.EncodeToString(make([]byte, 32))
	iv := base64.StdEncoding.EncodeToString(make([]byte, gcmNonceSize))
	if err := validateDirectEncryptedMessage(encryptedRequest(ciphertext, iv), false); err != nil {
		t.Fatalf("canonical payload must be accepted, got %v", err)
	}
}

func TestValidateDirectEncryptedMessageRejectsNonStandardBase64(t *testing.T) {
	nonZero := make([]byte, 32)
	for i := range nonZero {
		nonZero[i] = 0xFF
	}
	nonZeroIV := make([]byte, gcmNonceSize)
	for i := range nonZeroIV {
		nonZeroIV[i] = 0xFF
	}
	urlIV := base64.URLEncoding.EncodeToString(nonZeroIV)
	if !strings.Contains(urlIV, "_") {
		t.Fatalf("test setup: URL-encoded IV must contain '-' or '_' to differ from the standard alphabet")
	}
	validCiphertext := base64.StdEncoding.EncodeToString(nonZero)
	validIV := base64.StdEncoding.EncodeToString(nonZeroIV)

	tests := []struct {
		name       string
		ciphertext string
		iv         string
	}{
		{"base64url ciphertext", base64.RawURLEncoding.EncodeToString(nonZero), validIV},
		{"base64url iv", validCiphertext, urlIV},
		{"unpadded ciphertext", strings.TrimRight(validCiphertext, "="), validIV},
		{"unpadded iv", validCiphertext, strings.TrimRight(base64.StdEncoding.EncodeToString(append(nonZeroIV, 0)), "=")},
		{"hex iv", validCiphertext, "00112233445566778899aabb"},
		{"bytea text iv", validCiphertext, "\\x" + validIV},
		{"iv wrong length", validCiphertext, base64.StdEncoding.EncodeToString(make([]byte, 11))},
		{"ciphertext under minimum", base64.StdEncoding.EncodeToString(make([]byte, 15)), validIV},
		{"ciphertext over maximum", base64.StdEncoding.EncodeToString(make([]byte, maxCiphertextBytes+1)), validIV},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDirectEncryptedMessage(encryptedRequest(tt.ciphertext, tt.iv), false)
			if err != constant.ErrChatInvalidRequest && err != constant.ErrPayloadTooLarge {
				t.Fatalf("non-standard base64 payload must be rejected, got %v", err)
			}
		})
	}
}

func TestValidateDirectEncryptedMessageRejectsPlaintextBody(t *testing.T) {
	ciphertext := base64.StdEncoding.EncodeToString(make([]byte, 32))
	iv := base64.StdEncoding.EncodeToString(make([]byte, gcmNonceSize))
	if err := validateDirectEncryptedMessage(encryptedRequest(ciphertext, iv), true); err != constant.ErrChatEncryptionRequired {
		t.Fatalf("direct message with body must be rejected, got %v", err)
	}
}

func TestUpdatePublicKeysRejectsInvalidKeyBeforeTouchingRepository(t *testing.T) {
	service := &AuthenticationService{authenticationRepository: nil}
	if _, err := service.UpdatePublicKeys(1, request.UpdatePublicKeysRequest{PublicKey: "not-standard-base64!!"}); err != constant.ErrInvalidPublicKey {
		t.Fatalf("invalid public key must be rejected before repository access, got %v", err)
	}
}
