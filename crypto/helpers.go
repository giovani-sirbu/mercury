package crypto

import (
	"crypto/sha256"
	"encoding/base64"
)

// legacyIV is the fixed IV of the legacy AES-CFB format, read only for pre-migration secrets.
var legacyIV = []byte{35, 46, 57, 24, 85, 35, 24, 74, 87, 35, 88, 98, 66, 32, 14, 05}

// versionGCM prefixes the AES-256-GCM layout; legacy ciphertext has no prefix.
const versionGCM byte = 0x01

// gcmNonceSize is the standard 96-bit GCM nonce length.
const gcmNonceSize = 12

// deriveKey turns a secret of any length into a 32-byte AES-256 key via SHA-256.
func deriveKey(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}

// Encode returns the base64 representation of b.
func Encode(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}

// Decode returns the decoded bytes of s, or an error when s is not valid base64.
func Decode(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}
