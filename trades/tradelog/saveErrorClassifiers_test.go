package tradelog

import "testing"

func TestIsAPIErrorDetectsAPIErrorPrefix(t *testing.T) {
	if !isAPIError("<APIError> code=-2010, msg=something") {
		t.Fatal("expected isAPIError true when input contains <APIError>")
	}
	if isAPIError("plain error") {
		t.Fatal("expected isAPIError false when prefix absent")
	}
}

func TestHasInsufficientBalanceDetectsPhrase(t *testing.T) {
	if !HasInsufficientBalance("Insufficient funds (1.5 USDT) for requested action") {
		t.Fatal("expected HasInsufficientBalance true")
	}
	if HasInsufficientBalance("Order failed") {
		t.Fatal("expected HasInsufficientBalance false when phrase absent")
	}
}
