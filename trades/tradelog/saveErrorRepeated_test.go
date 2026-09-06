package tradelog

import (
	"errors"
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// The same funds failure already stands on the trade: nothing is written and
// the error goes back marked as a repeat, so events.Run keeps the backoff but
// does not log the identical line again.
func TestSaveErrorMarksAFailureTheTradeAlreadyCarriesAsRepeated(t *testing.T) {
	first := errors.New("Insufficient funds (16648.360232 USDT) for the requested action (stopLoss). You need at least 30014.646680 USDT to resume this trade.")
	again := errors.New("Insufficient funds (16648.360232 USDT) for the requested action (stopLoss). You need at least 30014.674170 USDT to resume this trade.")

	updates := 0
	event := events.Events{
		Trade: aggragates.Trades{ID: 39380, PositionType: "stopLoss", Status: aggragates.Active},
		Events: map[string]func(events.Events) (events.Events, error){
			"updateTrade": func(e events.Events) (events.Events, error) { updates++; return e, nil },
		},
	}

	blocked, err := SaveError(event, first)
	if errors.Is(err, events.ErrRepeatedFailure) {
		t.Fatal("the first failure is not a repeat")
	}
	if blocked.Trade.Status != aggragates.Blocked || len(blocked.Trade.Logs) != 1 || updates != 1 {
		t.Fatalf("first failure must block, log and persist once: status=%s logs=%d updates=%d", blocked.Trade.Status, len(blocked.Trade.Logs), updates)
	}

	repeated, err := SaveError(blocked, again)
	if !errors.Is(err, events.ErrRepeatedFailure) {
		t.Fatalf("the same failure on a later tick must come back as a repeat, got %v", err)
	}
	if err.Error() != again.Error() {
		t.Fatalf("the message must stay the gate's own text, got %q", err.Error())
	}
	if len(repeated.Trade.Logs) != 1 || updates != 1 {
		t.Fatalf("a repeat writes nothing: logs=%d updates=%d", len(repeated.Trade.Logs), updates)
	}
}
