package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"testing"
)

// TestDecodeReturnsErrorOnInvalidBase64 guards the fix that removed the panic
// from Decode. Before: any corrupt exchange secret in the DB would crash the
// service on the first request that needed it. After: the error propagates
// and the caller can decide what to do.
func TestDecodeReturnsErrorOnInvalidBase64(t *testing.T) {
	_, err := Decode("!!!not-base64!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64, got nil")
	}
}

// TestDecodeRoundTripsEncodedBytes keeps the golden path honest — Encode +
// Decode should be a pure round trip.
func TestDecodeRoundTripsEncodedBytes(t *testing.T) {
	payload := []byte("super-secret-exchange-api-key")
	encoded := Encode(payload)

	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(decoded) != string(payload) {
		t.Fatalf("round-trip mismatch: got %q, want %q", decoded, payload)
	}
}

// TestDecryptReturnsErrorOnCorruptCiphertext guards the Decrypt path end to
// end. A DB row with a mangled ApiSecret used to crash the caller (panic
// inside Decode); now it returns an error and the service stays up.
func TestDecryptReturnsErrorOnCorruptCiphertext(t *testing.T) {
	_, err := Decrypt("!!!not-valid-base64!!!", "0123456789abcdef")
	if err == nil {
		t.Fatal("expected error for corrupt ciphertext, got nil")
	}
}

// TestEncryptDecryptRoundTrip pins the AES-256-GCM round trip. The secret can
// be any length (it is hashed to a 32-byte key), and the ciphertext must carry
// the GCM version marker.
func TestEncryptDecryptRoundTrip(t *testing.T) {
	for _, secret := range []string{
		"0123456789abcdef", // 16 bytes
		"a-much-longer-high-entropy-secret-value-2026!!", // arbitrary length
	} {
		plain := "binance-api-secret-goes-here"

		encrypted, err := Encrypt(plain, secret)
		if err != nil {
			t.Fatalf("Encrypt failed: %v", err)
		}
		if encrypted == plain {
			t.Fatal("Encrypt produced plaintext output")
		}

		raw, err := Decode(encrypted)
		if err != nil || len(raw) == 0 || raw[0] != versionGCM {
			t.Fatalf("expected GCM version marker, got raw[0]=%v err=%v", raw, err)
		}

		decrypted, err := Decrypt(encrypted, secret)
		if err != nil {
			t.Fatalf("Decrypt failed: %v", err)
		}
		if decrypted != plain {
			t.Fatalf("round-trip mismatch: got %q, want %q", decrypted, plain)
		}
	}
}

// TestEncryptUsesFreshNonce ensures two encryptions of the same plaintext under
// the same key differ — i.e. no fixed IV/nonce reuse (the legacy CFB flaw).
func TestEncryptUsesFreshNonce(t *testing.T) {
	secret := "0123456789abcdef"
	a, _ := Encrypt("same-plaintext", secret)
	b, _ := Encrypt("same-plaintext", secret)
	if a == b {
		t.Fatal("two encryptions of the same plaintext produced identical ciphertext — nonce reuse")
	}
}

// TestDecryptWithWrongKeyReturnsError is the security upgrade over the legacy
// AES-CFB behaviour: GCM authenticates, so a wrong key fails the tag check and
// surfaces an error instead of returning silent garbage.
func TestDecryptWithWrongKeyReturnsError(t *testing.T) {
	encrypted, err := Encrypt("some-plaintext", "0123456789abcdef")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if _, err := Decrypt(encrypted, "fedcba9876543210"); err == nil {
		t.Fatal("expected error decrypting with wrong key, got nil")
	}
}

// TestDecryptDetectsTampering flips a byte in the authenticated ciphertext and
// asserts the GCM tag rejects it.
func TestDecryptDetectsTampering(t *testing.T) {
	secret := "0123456789abcdef"
	encrypted, err := Encrypt("integrity-matters", secret)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	raw, err := Decode(encrypted)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	raw[len(raw)-1] ^= 0xFF // flip a byte inside the tag
	if _, err := Decrypt(Encode(raw), secret); err == nil {
		t.Fatal("expected error decrypting tampered ciphertext, got nil")
	}
}

// TestDecryptLegacyReadsCFBData proves backward compatibility: secrets written
// by the old AES-CFB Encrypt are still readable via DecryptLegacy (used by the
// re-encryption migration to read pre-migration data with the old key).
func TestDecryptLegacyReadsCFBData(t *testing.T) {
	secret := "0123456789abcdef" // 16-byte legacy AES-128 key
	plain := "legacy-binance-secret"

	legacy := encryptLegacyCFB(t, plain, secret)
	got, err := DecryptLegacy(legacy, secret)
	if err != nil {
		t.Fatalf("DecryptLegacy: %v", err)
	}
	if got != plain {
		t.Fatalf("legacy round-trip mismatch: got %q, want %q", got, plain)
	}
}

// encryptLegacyCFB reproduces the pre-migration AES-CFB Encrypt so tests can
// generate legacy ciphertext to exercise the backward-compat read path.
func encryptLegacyCFB(t *testing.T, text, secret string) string {
	t.Helper()
	block, err := aes.NewCipher([]byte(secret))
	if err != nil {
		t.Fatalf("aes.NewCipher: %v", err)
	}
	cfb := cipher.NewCFBEncrypter(block, legacyIV)
	ct := make([]byte, len(text))
	cfb.XORKeyStream(ct, []byte(text))
	return Encode(ct)
}
