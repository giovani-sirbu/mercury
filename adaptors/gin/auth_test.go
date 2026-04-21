package ginAdaptors

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// newTestContext returns a gin.Context attached to an in-memory recorder so
// tests can inspect the response status without spinning up a listener.
func newTestContext(method, path, authHeader string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	req := httptest.NewRequest(method, path, nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	c.Request = req
	return c, w
}

// TestExtractBearerTokenRejectsMissingHeader verifies the out-of-range
// Authorization[0] access that the prior code had: an empty header map used
// to panic on `header["Authorization"][0]`. The current extractor returns
// ok=false, which translates into a clean 401 in the middleware.
func TestExtractBearerTokenRejectsMissingHeader(t *testing.T) {
	c, _ := newTestContext(http.MethodGet, "/x", "")

	_, ok := extractBearerToken(c)
	if ok {
		t.Fatal("expected ok=false for missing header")
	}
}

// TestExtractBearerTokenRejectsMalformedHeader covers the space-less header
// case that panicked the pre-fix code at `strings.Split(...)[1]`.
func TestExtractBearerTokenRejectsMalformedHeader(t *testing.T) {
	c, _ := newTestContext(http.MethodGet, "/x", "NoSpaceHere")

	_, ok := extractBearerToken(c)
	if ok {
		t.Fatal("expected ok=false for malformed header")
	}
}

// TestExtractBearerTokenRejectsEmptyToken covers the `Bearer ` case where the
// scheme is present but the token is empty.
func TestExtractBearerTokenRejectsEmptyToken(t *testing.T) {
	c, _ := newTestContext(http.MethodGet, "/x", "Bearer ")

	_, ok := extractBearerToken(c)
	if ok {
		t.Fatal("expected ok=false for empty token")
	}
}

// TestExtractBearerTokenReturnsToken is the happy path pin.
func TestExtractBearerTokenReturnsToken(t *testing.T) {
	c, _ := newTestContext(http.MethodGet, "/x", "Bearer abc.def.ghi")

	got, ok := extractBearerToken(c)
	if !ok {
		t.Fatal("expected ok=true for valid header")
	}
	if got != "abc.def.ghi" {
		t.Fatalf("extracted token mismatch: got %q, want %q", got, "abc.def.ghi")
	}
}

// TestIsAuthUnauthorizedWithNoHeader documents the end-to-end middleware
// behaviour for an unauthenticated request. Paired with the happy-path
// extractor test, this gives us coverage of both failure paths through the
// middleware without requiring a live JWT signer.
func TestIsAuthUnauthorizedWithNoHeader(t *testing.T) {
	c, w := newTestContext(http.MethodGet, "/protected", "")

	IsAuth(c)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected %d, got %d", http.StatusUnauthorized, w.Code)
	}
	if !c.IsAborted() {
		t.Fatal("expected IsAuth to abort the gin chain")
	}
}
