package binanceAdaptor

import (
	"errors"
	"testing"

	"github.com/adshao/go-binance/v2/common"
)

// TestApiErrorKeepsResponse pins the field set ApiError carries over. Response
// used to be dropped, and it is the only field binance fills when it answers
// with something that is not a {code,msg} document — a 404 from an endpoint the
// network does not serve. Losing it turned every such failure into a bare
// "<APIError> rsp=" that named neither the endpoint nor the status.
func TestApiErrorKeepsResponse(t *testing.T) {
	tests := []struct {
		name        string
		in          error
		wantNil     bool
		wantCode    int64
		wantMessage string
		wantErrText string
	}{
		{
			name:    "nil stays nil",
			in:      nil,
			wantNil: true,
		},
		{
			name:        "business error keeps code and message",
			in:          &common.APIError{Code: -2015, Message: "Invalid API-key, IP, or permissions for action."},
			wantCode:    -2015,
			wantMessage: "Invalid API-key, IP, or permissions for action.",
			wantErrText: "<APIError> code=-2015, msg=Invalid API-key, IP, or permissions for action.",
		},
		{
			name:        "non-json body survives the wrap",
			in:          &common.APIError{Response: []byte(`{"error":"404 Not Found"}`)},
			wantErrText: `<APIError> rsp={"error":"404 Not Found"}`,
		},
		{
			name:        "plain error becomes a message",
			in:          errors.New("dial tcp: lookup failed"),
			wantMessage: "dial tcp: lookup failed",
			wantErrText: "<APIError> code=0, msg=dial tcp: lookup failed",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := ApiError(test.in)

			if test.wantNil {
				if got != nil {
					t.Fatalf("expected nil, got %#v", got)
				}
				return
			}

			if got == nil {
				t.Fatal("expected an error, got nil")
			}
			if got.Code != test.wantCode {
				t.Errorf("code: got %d, want %d", got.Code, test.wantCode)
			}
			if got.Message != test.wantMessage {
				t.Errorf("message: got %q, want %q", got.Message, test.wantMessage)
			}
			if got.Error() != test.wantErrText {
				t.Errorf("Error(): got %q, want %q", got.Error(), test.wantErrText)
			}
		})
	}
}
