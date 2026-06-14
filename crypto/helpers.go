package crypto

import (
	"crypto/sha256"
	"encoding/base64"
)

// legacyIV is the fixed 16-byte IV used by the legacy AES-CFB format
// (DecryptLegacy / data written before the AES-256-GCM migration). New data is
// written by Encrypt with AES-256-GCM and a fresh random nonce per ciphertext,
// so this IV is only ever used to read pre-migration secrets.
var legacyIV = []byte{35, 46, 57, 24, 85, 35, 24, 74, 87, 35, 88, 98, 66, 32, 14, 05}

// versionGCM tags the AES-256-GCM ciphertext layout:
//
//	base64( versionGCM(1) || nonce(12) || ciphertext+tag )
//
// Legacy AES-CFB ciphertext has no version prefix; Decrypt uses the absence of
// this marker to route to the legacy path.
const versionGCM byte = 0x01

// gcmNonceSize is the standard 96-bit GCM nonce length.
const gcmNonceSize = 12

// deriveKey turns an arbitrary-length secret into a fixed 32-byte AES-256 key
// via SHA-256. This decouples the cipher key size from the API_SECRET string
// length, so the secret can be a long, high-entropy value rather than an exact
// 16/24/32-byte key.
func deriveKey(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}

// Encode returns the base64 representation of b.
func Encode(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}

// Decode returns the decoded bytes of s or an error if s is not valid base64.
// Callers must handle the error path — a corrupt stored secret must not panic
// and take down the service.
func Decode(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}
