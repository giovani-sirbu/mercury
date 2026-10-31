package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"fmt"
)

// Decrypt extracts the plaintext: AES-256-GCM when the payload starts with
// versionGCM, else the legacy AES-CFB path.
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
