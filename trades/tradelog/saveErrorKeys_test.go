package tradelog

import (
	"errors"
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// errorEvent is a trade in trailingBuy whose chain failed, its updateTrade a no-op.
func errorEvent(logs ...aggragates.TradesLogs) events.Events {
	return events.Events{
		Events: map[string]func(events.Events) (events.Events, error){
			"updateTrade": nopUpdateTradeForSaveError,
		},
		Trade:  aggragates.Trades{PositionType: "trailingBuy", PositionPrice: 100, Status: aggragates.Active, Logs: logs},
		Params: aggragates.Params{OldPosition: "buy", OldPositionPrice: 99},
	}
}

// Every failure's row is keyed by its kind: a funds refusal, an exchange error, any other failure.
func TestSaveErrorKeysTheRowByTheFailure(t *testing.T) {
	for message, key := range map[string]string{
		"Insufficient funds (12.5 USDT) for the requested action (trailingBuy).": aggragates.LogKeyInsufficientFunds,
		"<APIError> code=-2010, msg=Account has insufficient balance":            aggragates.LogKeyExchangeError,
		"order quantity under the lot size":                                      aggragates.LogKeyActionFailed,
	} {
		got, _ := SaveError(errorEvent(), errors.New(message))
		if len(got.Trade.Logs) != 1 || got.Trade.Logs[0].MessageKey != key {
			t.Errorf("%q: rows %+v, want one keyed %q", message, got.Trade.Logs, key)
		}
	}
}

// A repeat is the same key and the same text without numbers; the same text under another key is a new failure.
func TestSaveErrorRepeatsOnTheKeyAndTheText(t *testing.T) {
	failure := errors.New("order quantity under the lot size 12")
	for name, c := range map[string]struct {
		last     aggragates.TradesLogs
		repeated bool
	}{
		"the same key and text":           {aggragates.TradesLogs{Message: "order quantity under the lot size 7", MessageKey: aggragates.LogKeyActionFailed}, true},
		"an unkeyed row of the same text": {aggragates.TradesLogs{Message: "order quantity under the lot size 7"}, false},
		"another key, the same text":      {aggragates.TradesLogs{Message: "order quantity under the lot size 7", MessageKey: aggragates.LogKeyExchangeError}, false},
		"the same key, another text":      {aggragates.TradesLogs{Message: "price filter", MessageKey: aggragates.LogKeyActionFailed}, false},
	} {
		got, err := SaveError(errorEvent(c.last), failure)
		if errors.Is(err, events.ErrRepeatedFailure) != c.repeated {
			t.Errorf("%s: repeated %v, want %v", name, errors.Is(err, events.ErrRepeatedFailure), c.repeated)
		}
		if want := map[bool]int{true: 1, false: 2}[c.repeated]; len(got.Trade.Logs) != want {
			t.Errorf("%s: %d rows, want %d", name, len(got.Trade.Logs), want)
		}
	}
}

// A funds refusal on a trade found on hold ends blocked, the hold status gone; any other failure
// leaves the hold it found.
func TestAFailureFromHoldBlocksOnlyOnFundsOrTheExchange(t *testing.T) {
	for message, want := range map[string]aggragates.Status{
		"Insufficient funds (12.5 USDT) for the requested action (trailingBuy).": aggragates.Blocked,
		"<APIError> code=-2010, msg=Account has insufficient balance":            aggragates.Blocked,
		"order quantity under the lot size":                                      aggragates.Hold,
	} {
		event := errorEvent()
		event.Trade.Status = aggragates.Hold
		got, _ := SaveError(event, errors.New(message))
		if got.Trade.Status != want {
			t.Errorf("%q: status %q, want %q", message, got.Trade.Status, want)
		}
	}
}
