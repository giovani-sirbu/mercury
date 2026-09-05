package events

import (
	"errors"
	"fmt"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// backoffDuration reads the recorded backoff for a trade, or zero when the
// trade has never been backed off.
func backoffDuration(tradeID uint) int64 {
	rwLocker.RLock()
	defer rwLocker.RUnlock()

	return int64(backoffTries[tradeID].duration)
}

// runChain drives one action that returns err, the way a real chain does.
func runChain(tradeID uint, err error) {
	event := Events{
		Trade:       aggragates.Trades{ID: tradeID},
		EventsNames: []string{"only"},
		Events: map[string]func(Events) (Events, error){
			"only": func(e Events) (Events, error) { return e, err },
		},
	}
	event.Run()
}

// A hold is a decision, not a transport failure. Backing it off extended the
// trade lock on every held tick, so a held trade was throttled to one
// evaluation per minute and the gate holding it could not re-examine it until
// the lock expired.
func TestHoldDoesNotExtendTheTradeLock(t *testing.T) {
	const tradeID = uint(910001)

	runChain(tradeID, fmt.Errorf("%w: stopLoss regime: add not allowed", ErrTradeHeld))

	if got := backoffDuration(tradeID); got != 0 {
		t.Fatalf("a hold recorded a %v backoff; it must record none", got)
	}
}

// Any other error still backs off: that is what stops a chain that keeps
// failing from being retried on every tick.
func TestARealErrorStillBacksOff(t *testing.T) {
	const tradeID = uint(910002)

	runChain(tradeID, errors.New("exchange rejected the order"))

	if backoffDuration(tradeID) == 0 {
		t.Fatal("a real error must record a backoff")
	}
}
