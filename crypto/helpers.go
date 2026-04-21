package crypto

import "encoding/base64"

// bytes is the fixed 16-byte IV used by Encrypt/Decrypt for AES-CFB.
// TODO(security): IV reuse in CFB mode is a known weakness. Migrate to AES-GCM with
// a per-ciphertext random nonce prepended to the output. Requires a DB migration to
// re-encrypt all stored exchange secrets. Tracked outside mercury v2.
var bytes = []byte{35, 46, 57, 24, 85, 35, 24, 74, 87, 35, 88, 98, 66, 32, 14, 05}

// Encode returns the base64 representation of b.
func Encode(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}

// Decode returns the decoded bytes of s or an error if s is not valid base64.
// Previously this function panicked on bad input, taking down the entire service
// on any corrupt exchange secret. Callers must handle the error path.
func Decode(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}
