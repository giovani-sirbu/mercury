package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"io"
)

// Encrypt encrypts text with AES-256-GCM under a key derived from secret
// (SHA-256). Output is base64( versionGCM || 12-byte random nonce ||
// ciphertext+tag ).
//
// GCM is authenticated (tamper-evident) and uses a fresh random nonce per call,
// which fixes the two weaknesses of the legacy AES-CFB scheme: the hard-coded
// fixed IV and the lack of integrity. Reading legacy ciphertext is still
// supported via Decrypt (auto-detected) / DecryptLegacy during migration.
func Encrypt(text, secret string) (string, error) {
	block, err := aes.NewCipher(deriveKey(secret))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	// out = versionGCM || nonce || (Seal appends ciphertext+tag here)
	out := make([]byte, 0, 1+len(nonce)+len(text)+gcm.Overhead())
	out = append(out, versionGCM)
	out = append(out, nonce...)
	out = gcm.Seal(out, nonce, []byte(text), nil)

	return Encode(out), nil
}
