package crypto

import (
	"strings"
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
	// 16-byte AES key (required for NewCipher) but clearly bogus ciphertext.
	_, err := Decrypt("!!!not-valid-base64!!!", "0123456789abcdef")
	if err == nil {
		t.Fatal("expected error for corrupt ciphertext, got nil")
	}
}

// TestEncryptDecryptRoundTrip pins the AES-CFB round trip so future IV /
// mode / padding refactors must keep the same payload recoverable.
func TestEncryptDecryptRoundTrip(t *testing.T) {
	secret := "0123456789abcdef" // 16 bytes for AES-128
	plain := "binance-api-key-goes-here"

	encrypted, err := Encrypt(plain, secret)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}
	if encrypted == plain {
		t.Fatal("Encrypt produced plaintext output")
	}

	decrypted, err := Decrypt(encrypted, secret)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}
	if decrypted != plain {
		t.Fatalf("round-trip mismatch: got %q, want %q", decrypted, plain)
	}
}

// TestDecryptWithWrongKeyReturnsGarbage documents the current behaviour: a
// wrong key decodes into garbage bytes rather than returning an error. This
// is an AES-CFB property, not a bug — but it's important context for any
// future migration to AES-GCM, which WOULD authenticate and surface errors.
func TestDecryptWithWrongKeyReturnsGarbage(t *testing.T) {
	secret := "0123456789abcdef"
	wrong := "fedcba9876543210"
	plain := "some-plaintext"

	encrypted, err := Encrypt(plain, secret)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	decrypted, err := Decrypt(encrypted, wrong)
	if err != nil {
		t.Fatalf("Decrypt with wrong key unexpectedly errored: %v", err)
	}
	// Not asserting specific garbage; just asserting it isn't the plaintext.
	if decrypted == plain {
		t.Fatal("wrong key recovered plaintext — AES-CFB invariant broken")
	}
	// And the garbage should not contain the plaintext as a substring.
	if strings.Contains(decrypted, plain) {
		t.Fatalf("wrong-key decryption leaked plaintext substring: %q", decrypted)
	}
}
