package helpers

import (
	"crypto/ecdh"
	"encoding/base64"
	"strings"
	"testing"
)

func TestIsValidPublicKeyP256AcceptsCanonicalRawKey(t *testing.T) {
	privateKey, err := ecdh.P256().GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate P-256 key: %v", err)
	}
	encoded := base64.StdEncoding.EncodeToString(privateKey.PublicKey().Bytes())
	if !IsValidPublicKeyP256(encoded) {
		t.Fatal("canonical standard-base64 raw P-256 public key must be accepted")
	}
}

func TestIsValidPublicKeyP256RejectsNonStandardBase64(t *testing.T) {
	privateKey, err := ecdh.P256().GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate P-256 key: %v", err)
	}
	raw := privateKey.PublicKey().Bytes()

	tests := map[string]string{
		"base64url alphabet": base64.RawURLEncoding.EncodeToString(raw),
		"missing padding":    strings.TrimRight(base64.StdEncoding.EncodeToString(raw), "="),
		"hex":                "04" + strings.Repeat("ab", 64),
		"bytea text":         "\\x" + base64.StdEncoding.EncodeToString(raw),
		"non-canonical bits": "QR==",
		"empty":              "",
	}
	for name, encoded := range tests {
		if IsValidPublicKeyP256(encoded) {
			t.Errorf("%s: input must be rejected, was accepted", name)
		}
	}
}
