package helpers

import (
	"crypto/ecdh"
	"encoding/base64"
)

const p256PublicKeySize = 65

// canonicalBase64 rejects non-canonical input (missing padding, base64url
// alphabet, dirty trailing bits) so stored strings round-trip byte-identically.
var canonicalBase64 = base64.StdEncoding.Strict()

func IsValidPublicKeyP256(encoded string) bool {
	raw, err := canonicalBase64.DecodeString(encoded)
	if err != nil || len(raw) != p256PublicKeySize || raw[0] != 0x04 {
		return false
	}
	if _, err := ecdh.P256().NewPublicKey(raw); err != nil {
		return false
	}
	return true
}
