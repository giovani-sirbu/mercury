package actions

import "testing"

func TestExtractExchangeErrorMessageExtractsAfterPrefix(t *testing.T) {
	got := extractExchangeErrorMessage("<APIError> code=-2010, msg=Account has insufficient balance for requested action.")
	want := "Account has insufficient balance for requested action."
	if got != want {
		t.Errorf("extractExchangeErrorMessage mismatch: got %q, want %q", got, want)
	}
}

func TestExtractExchangeErrorMessageReturnsInputWhenPrefixMissing(t *testing.T) {
	const input = "plain error without prefix"
	if got := extractExchangeErrorMessage(input); got != input {
		t.Errorf("expected input returned unchanged, got %q", got)
	}
}
