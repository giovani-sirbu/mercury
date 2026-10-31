package tradelog

import (
	"errors"
	"testing"

	"github.com/giovani-sirbu/mercury/events"
)

func TestSaveErrorExtractsExchangeMessage(t *testing.T) {
	event := events.Events{
		Events: map[string]func(events.Events) (events.Events, error){
			"updateTrade": func(event events.Events) (events.Events, error) {
				return event, nil
			},
		},
	}

	got, err := SaveError(event, errors.New("<APIError> code=-2010, msg=Account has insufficient balance for requested action."))
	if err == nil {
		t.Fatal("expected original error to be returned")
	}
	if len(got.Trade.Logs) != 1 {
		t.Fatalf("logs count mismatch: got %d, want 1", len(got.Trade.Logs))
	}

	want := "Account has insufficient balance for requested action."
	if got.Trade.Logs[0].Message != want {
		t.Fatalf("log message mismatch: got %q, want %q", got.Trade.Logs[0].Message, want)
	}
}
