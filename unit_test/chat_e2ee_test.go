package controllers_test

import (
	"encoding/base64"
	"strings"
	"testing"

	"lechat-backend/internal/constant"
	"lechat-backend/internal/request"
	"lechat-backend/internal/services"
)

// Constants from internal/services/chat_service.go. Kept as literals here so
// the tests can live in an external test package; update them together if the
// service-side values change.
const (
	gcmNonceE2EE        = 12
	maxCiphertextE2EE   = 48 * 1024
	maxGroupKeyBatchE2E = 250
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
	iv := base64.StdEncoding.EncodeToString(make([]byte, gcmNonceE2EE))
	if err := services.ValidateDirectEncryptedMessage(encryptedRequest(ciphertext, iv), false); err != nil {
		t.Fatalf("canonical payload must be accepted, got %v", err)
	}
}

func TestValidateDirectEncryptedMessageRejectsNonStandardBase64(t *testing.T) {
	nonZero := make([]byte, 32)
	for i := range nonZero {
		nonZero[i] = 0xFF
	}
	nonZeroIV := make([]byte, gcmNonceE2EE)
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
		{"ciphertext over maximum", base64.StdEncoding.EncodeToString(make([]byte, maxCiphertextE2EE+1)), validIV},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := services.ValidateDirectEncryptedMessage(encryptedRequest(tt.ciphertext, tt.iv), false)
			if err != constant.ErrChatInvalidRequest && err != constant.ErrPayloadTooLarge {
				t.Fatalf("non-standard base64 payload must be rejected, got %v", err)
			}
		})
	}
}

func TestValidateDirectEncryptedMessageRejectsPlaintextBody(t *testing.T) {
	ciphertext := base64.StdEncoding.EncodeToString(make([]byte, 32))
	iv := base64.StdEncoding.EncodeToString(make([]byte, gcmNonceE2EE))
	if err := services.ValidateDirectEncryptedMessage(encryptedRequest(ciphertext, iv), true); err != constant.ErrChatEncryptionRequired {
		t.Fatalf("direct message with body must be rejected, got %v", err)
	}
}

func TestUpdatePublicKeysRejectsInvalidKeyBeforeTouchingRepository(t *testing.T) {
	service := &services.AuthenticationService{}
	if _, err := service.UpdatePublicKeys(1, request.UpdatePublicKeysRequest{PublicKey: "not-standard-base64!!"}); err != constant.ErrInvalidPublicKey {
		t.Fatalf("invalid public key must be rejected before repository access, got %v", err)
	}
}

func groupEncryptedRequest(keyVersion int, ciphertext, iv string) request.SendMessageRequest {
	version := 1
	return request.SendMessageRequest{
		Type:              "text",
		EncryptionVersion: &version,
		KeyVersion:        &keyVersion,
		Ciphertext:        &ciphertext,
		IV:                &iv,
	}
}

func TestValidateGroupEncryptedMessage(t *testing.T) {
	validCiphertext := base64.StdEncoding.EncodeToString(make([]byte, 32))
	validIV := base64.StdEncoding.EncodeToString(make([]byte, gcmNonceE2EE))

	t.Run("accepts valid encrypted payload", func(t *testing.T) {
		req := groupEncryptedRequest(1, validCiphertext, validIV)
		if err := services.ValidateGroupEncryptedMessage(req, false); err != nil {
			t.Fatalf("valid group encrypted message must be accepted, got %v", err)
		}
	})

	t.Run("rejects mixed plaintext body with encrypted payload", func(t *testing.T) {
		req := groupEncryptedRequest(1, validCiphertext, validIV)
		if err := services.ValidateGroupEncryptedMessage(req, true); err != constant.ErrChatInvalidRequest {
			t.Fatalf("mixed body+ciphertext group payload must be rejected, got %v", err)
		}
	})

	t.Run("rejects missing or zero key_version", func(t *testing.T) {
		req := groupEncryptedRequest(0, validCiphertext, validIV)
		if err := services.ValidateGroupEncryptedMessage(req, false); err != constant.ErrChatInvalidRequest {
			t.Fatalf("zero key_version must be rejected, got %v", err)
		}
		reqNoVersion := groupEncryptedRequest(1, validCiphertext, validIV)
		reqNoVersion.KeyVersion = nil
		if err := services.ValidateGroupEncryptedMessage(reqNoVersion, false); err != constant.ErrChatInvalidRequest {
			t.Fatalf("nil key_version must be rejected, got %v", err)
		}
	})

	t.Run("rejects invalid encryption_version", func(t *testing.T) {
		req := groupEncryptedRequest(1, validCiphertext, validIV)
		badVersion := 2
		req.EncryptionVersion = &badVersion
		if err := services.ValidateGroupEncryptedMessage(req, false); err != constant.ErrChatInvalidRequest {
			t.Fatalf("encryption_version != 1 must be rejected, got %v", err)
		}
	})

	t.Run("rejects non-standard base64", func(t *testing.T) {
		req := groupEncryptedRequest(1, "not-valid-base64", validIV)
		if err := services.ValidateGroupEncryptedMessage(req, false); err != constant.ErrChatInvalidRequest {
			t.Fatalf("bad ciphertext base64 must be rejected, got %v", err)
		}
	})
}

func TestUploadGroupKeysValidation(t *testing.T) {
	service := services.NewChatService(nil).(*services.ChatService)
	validEncKey := base64.StdEncoding.EncodeToString(make([]byte, 48))
	validIV := base64.StdEncoding.EncodeToString(make([]byte, 12))

	t.Run("rejects invalid conversation ID", func(t *testing.T) {
		err := service.UploadGroupKeys(1, "invalid-uuid", request.UploadGroupKeysRequest{
			KeyVersion: 1,
			Keys: []request.UploadGroupKeyItem{
				{UserID: 2, EncryptedKey: validEncKey, IV: validIV},
			},
		})
		if err != constant.ErrChatInvalidRequest {
			t.Fatalf("invalid conversation UUID must return ErrChatInvalidRequest, got %v", err)
		}
	})

	t.Run("rejects key_version < 1", func(t *testing.T) {
		err := service.UploadGroupKeys(1, "123e4567-e89b-12d3-a456-426614174000", request.UploadGroupKeysRequest{
			KeyVersion: 0,
			Keys: []request.UploadGroupKeyItem{
				{UserID: 2, EncryptedKey: validEncKey, IV: validIV},
			},
		})
		if err != constant.ErrChatInvalidRequest {
			t.Fatalf("key_version < 1 must return ErrChatInvalidRequest, got %v", err)
		}
	})

	t.Run("rejects empty keys", func(t *testing.T) {
		err := service.UploadGroupKeys(1, "123e4567-e89b-12d3-a456-426614174000", request.UploadGroupKeysRequest{
			KeyVersion: 1,
			Keys:       nil,
		})
		if err != constant.ErrChatInvalidRequest {
			t.Fatalf("empty keys must return ErrChatInvalidRequest, got %v", err)
		}
	})

	t.Run("rejects duplicate user_id in batch", func(t *testing.T) {
		err := service.UploadGroupKeys(1, "123e4567-e89b-12d3-a456-426614174000", request.UploadGroupKeysRequest{
			KeyVersion: 1,
			Keys: []request.UploadGroupKeyItem{
				{UserID: 2, EncryptedKey: validEncKey, IV: validIV},
				{UserID: 2, EncryptedKey: validEncKey, IV: validIV},
			},
		})
		if err != constant.ErrChatInvalidRequest {
			t.Fatalf("duplicate user_id must return ErrChatInvalidRequest, got %v", err)
		}
	})

	t.Run("rejects invalid encrypted_key length", func(t *testing.T) {
		badEncKey := base64.StdEncoding.EncodeToString(make([]byte, 47))
		err := service.UploadGroupKeys(1, "123e4567-e89b-12d3-a456-426614174000", request.UploadGroupKeysRequest{
			KeyVersion: 1,
			Keys: []request.UploadGroupKeyItem{
				{UserID: 2, EncryptedKey: badEncKey, IV: validIV},
			},
		})
		if err != constant.ErrChatInvalidRequest {
			t.Fatalf("encrypted_key != 48 bytes must return ErrChatInvalidRequest, got %v", err)
		}
	})

	t.Run("rejects invalid IV length", func(t *testing.T) {
		badIV := base64.StdEncoding.EncodeToString(make([]byte, 16))
		err := service.UploadGroupKeys(1, "123e4567-e89b-12d3-a456-426614174000", request.UploadGroupKeysRequest{
			KeyVersion: 1,
			Keys: []request.UploadGroupKeyItem{
				{UserID: 2, EncryptedKey: validEncKey, IV: badIV},
			},
		})
		if err != constant.ErrChatInvalidRequest {
			t.Fatalf("IV != 12 bytes must return ErrChatInvalidRequest, got %v", err)
		}
	})
}

func TestGetMyGroupKeyValidation(t *testing.T) {
	service := services.NewChatService(nil).(*services.ChatService)
	badVersion := 0
	if _, err := service.GetMyGroupKey(1, "123e4567-e89b-12d3-a456-426614174000", &badVersion); err != constant.ErrChatInvalidRequest {
		t.Fatalf("key_version < 1 must return ErrChatInvalidRequest, got %v", err)
	}
	if _, err := service.GetMyGroupKey(1, "invalid-uuid", nil); err != constant.ErrChatInvalidRequest {
		t.Fatalf("invalid UUID must return ErrChatInvalidRequest, got %v", err)
	}
}

func TestDirectMessageRejectsKeyVersion(t *testing.T) {
	keyVersion := 1
	req := encryptedRequest(base64.StdEncoding.EncodeToString(make([]byte, 32)), base64.StdEncoding.EncodeToString(make([]byte, 12)))
	req.KeyVersion = &keyVersion
	if err := services.ValidateDirectEncryptedMessage(req, false); err != constant.ErrChatInvalidRequest {
		t.Fatalf("direct E2EE payload with key_version must be rejected, got %v", err)
	}
}

func TestUploadGroupKeysRejectsOversizedBatch(t *testing.T) {
	service := services.NewChatService(nil).(*services.ChatService)
	validEncKey := base64.StdEncoding.EncodeToString(make([]byte, 48))
	validIV := base64.StdEncoding.EncodeToString(make([]byte, 12))
	oversized := make([]request.UploadGroupKeyItem, maxGroupKeyBatchE2E+1)
	for i := range oversized {
		oversized[i] = request.UploadGroupKeyItem{UserID: uint(i + 1), EncryptedKey: validEncKey, IV: validIV}
	}
	err := service.UploadGroupKeys(1, "123e4567-e89b-12d3-a456-426614174000", request.UploadGroupKeysRequest{
		KeyVersion: 1,
		Keys:       oversized,
	})
	if err != constant.ErrPayloadTooLarge {
		t.Fatalf("keys batch over %d items must return ErrPayloadTooLarge, got %v", maxGroupKeyBatchE2E, err)
	}
}

func TestUploadGroupKeysRateLimitedAfterBurst(t *testing.T) {
	service := services.NewChatService(nil).(*services.ChatService)
	req := request.UploadGroupKeysRequest{
		KeyVersion: 1,
		Keys: []request.UploadGroupKeyItem{
			{UserID: 2, EncryptedKey: "not-valid-base64", IV: "not-valid-base64"},
		},
	}
	var lastErr error
	// The key upload limiter allows 10 requests/min/user. Each call consumes a
	// token and then fails item-level validation (bad base64) AFTER the limiter
	// check; calls beyond the burst must short-circuit with ErrRateLimited.
	for i := 0; i < 12; i++ {
		lastErr = service.UploadGroupKeys(1, "123e4567-e89b-12d3-a456-426614174000", req)
	}
	if lastErr != constant.ErrRateLimited {
		t.Fatalf("upload key burst over the limit must return ErrRateLimited, got %v", lastErr)
	}
}
