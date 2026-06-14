package auth

import (
	"errors"
	"os"
	"testing"
	"time"
)

// TestSecretKey_ReadsCurrentEnvValue guards against the previous bug where
// secretKey was a package-level var initialized to os.Getenv(...) before
// godotenv.Load() ran in main(). The fix reads the env var on every call;
// changing the env between calls must change the signing key.
func TestSecretKey_ReadsCurrentEnvValue(t *testing.T) {
	t.Setenv("ENCRYPTION_TOKEN_KEY", "first-key")
	first := string(secretKey())
	if first != "first-key" {
		t.Fatalf("expected first-key, got %q", first)
	}

	t.Setenv("ENCRYPTION_TOKEN_KEY", "second-key")
	second := string(secretKey())
	if second != "second-key" {
		t.Fatalf("expected second-key after env change, got %q", second)
	}
}

// TestGenerateAndVerifyTokens_RoundTrip ensures tokens signed with the env
// key verify with the same env key, and that a key swap invalidates them.
func TestGenerateAndVerifyTokens_RoundTrip(t *testing.T) {
	t.Setenv("ENCRYPTION_TOKEN_KEY", "round-trip-key")
	t.Setenv("ACCESS_TOKEN_DURATION", "1h")
	t.Setenv("REFRESH_TOKEN_DURATION", "24h")

	tokens, err := GenerateTokens(42, "user@example.com", "user")
	if err != nil {
		t.Fatalf("GenerateTokens failed: %v", err)
	}
	if tokens.AccessToken == "" || tokens.RefreshToken == "" {
		t.Fatal("expected non-empty access and refresh tokens")
	}

	if err := VerifyToken(tokens.AccessToken); err != nil {
		t.Fatalf("VerifyToken should accept token signed with current key: %v", err)
	}

	claims, err := ParseToken(tokens.AccessToken)
	if err != nil {
		t.Fatalf("ParseToken failed: %v", err)
	}
	if claims.Id != 42 || claims.Email != "user@example.com" || claims.Role != "user" {
		t.Fatalf("unexpected claims: %+v", claims)
	}

	// Rotating the key must reject tokens signed with the old one.
	t.Setenv("ENCRYPTION_TOKEN_KEY", "different-key")
	if err := VerifyToken(tokens.AccessToken); err == nil {
		t.Fatal("VerifyToken accepted token after key rotation — secretKey is being cached")
	}
}

// TestVerifyToken_RejectsExpired confirms the lazy-read path still respects
// the exp claim. Sanity check that the fix didn't regress expiry handling.
func TestVerifyToken_RejectsExpired(t *testing.T) {
	t.Setenv("ENCRYPTION_TOKEN_KEY", "expiry-key")

	expired, err := createToken(UserClaims{
		Id:    1,
		Email: "a@b.c",
		Role:  "user",
		Exp:   time.Now().Add(-time.Hour).Unix(),
	})
	if err != nil {
		t.Fatalf("createToken failed: %v", err)
	}

	if err := VerifyToken(expired); err == nil {
		t.Fatal("VerifyToken accepted an expired token")
	}
}

// TestEmptyKey_FailsClosed verifies that an unset ENCRYPTION_TOKEN_KEY makes
// signing error and verification reject — rather than authenticating with a
// zero-byte (trivially forgeable) HMAC key.
func TestEmptyKey_FailsClosed(t *testing.T) {
	t.Setenv("ENCRYPTION_TOKEN_KEY", "real-key-for-this-test")
	t.Setenv("ACCESS_TOKEN_DURATION", "1h")
	t.Setenv("REFRESH_TOKEN_DURATION", "24h")
	tokens, err := GenerateTokens(1, "a@b.c", "user")
	if err != nil {
		t.Fatalf("GenerateTokens: %v", err)
	}

	// Clear the key: sign + verify must both fail closed.
	t.Setenv("ENCRYPTION_TOKEN_KEY", "")

	if _, err := GenerateTokens(1, "a@b.c", "user"); !errors.Is(err, ErrMissingTokenKey) {
		t.Fatalf("expected ErrMissingTokenKey when signing with empty key, got %v", err)
	}
	if err := VerifyToken(tokens.AccessToken); err == nil {
		t.Fatal("VerifyToken accepted a token while ENCRYPTION_TOKEN_KEY was empty")
	}
	if _, err := ParseToken(tokens.AccessToken); err == nil {
		t.Fatal("ParseToken accepted a token while ENCRYPTION_TOKEN_KEY was empty")
	}
}

// TestExtractJwtToken covers the simple header-parsing helper to keep
// the package's test surface minimal but useful.
func TestExtractJwtToken(t *testing.T) {
	cases := []struct {
		name    string
		header  string
		want    string
		wantErr bool
	}{
		{"empty header", "", "", true},
		{"missing scheme", "abc", "", true},
		{"valid bearer", "Bearer xyz.token.value", "xyz.token.value", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ExtractJwtToken(tc.header)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}

	// Defensive: prevent the env from leaking across packages.
	_ = os.Setenv("ENCRYPTION_TOKEN_KEY", "")
}
