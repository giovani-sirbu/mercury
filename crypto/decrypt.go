package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"fmt"
)

// Decrypt extracts the plaintext, auto-detecting the format:
//
//   - AES-256-GCM (new): the payload starts with versionGCM. The key is derived
//     from secret via SHA-256 and the GCM tag is verified, so a wrong key or
//     tampered ciphertext returns an error rather than silent garbage. No
//     fallback to the legacy path is attempted for a versioned payload.
//   - AES-CFB (legacy): no version marker. Decrypted with the raw secret bytes
//     and the fixed legacy IV, for data written before the GCM migration.
func Decrypt(text, secret string) (string, error) {
	raw, err := Decode(text)
	if err != nil {
		return "", err
	}

	if len(raw) >= 1+gcmNonceSize+16 && raw[0] == versionGCM {
		block, err := aes.NewCipher(deriveKey(secret))
		if err != nil {
			return "", err
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return "", err
		}
		nonce := raw[1 : 1+gcm.NonceSize()]
		cipherText := raw[1+gcm.NonceSize():]
		plainText, err := gcm.Open(nil, nonce, cipherText, nil)
		if err != nil {
			return "", fmt.Errorf("gcm open: %w", err)
		}
		return string(plainText), nil
	}

	return decryptLegacyCFB(raw, secret)
}

// DecryptLegacy forces the legacy AES-CFB path regardless of any version byte.
// The re-encryption migration uses this to read pre-migration secrets with the
// old key, so a legacy ciphertext whose first byte happens to equal versionGCM
// can never be misrouted to the GCM path.
func DecryptLegacy(text, secret string) (string, error) {
	raw, err := Decode(text)
	if err != nil {
		return "", err
	}
	return decryptLegacyCFB(raw, secret)
}

func decryptLegacyCFB(cipherText []byte, secret string) (string, error) {
	block, err := aes.NewCipher([]byte(secret))
	if err != nil {
		return "", err
	}
	cfb := cipher.NewCFBDecrypter(block, legacyIV)
	plainText := make([]byte, len(cipherText))
	cfb.XORKeyStream(plainText, cipherText)
	return string(plainText), nil
}
