package events

import (
	"errors"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// A repeat of a failure the trade already carries keeps the exact message —
// the ERROR rows and the statistics key on it — and is recognisable as a
// repeat, but never as a hold: a hold skips the backoff, a repeat must not.
func TestRepeatedKeepsTheMessageAndIsNotAHold(t *testing.T) {
	cause := errors.New("Insufficient funds (16648.360232 USDT) for the requested action (stopLoss). You need at least 30014.646680 USDT to resume this trade.")
	err := Repeated(cause)

	if err.Error() != cause.Error() {
		t.Fatalf("message changed: %q", err.Error())
	}
	if !errors.Is(err, ErrRepeatedFailure) {
		t.Fatal("a wrapped repeat must report ErrRepeatedFailure")
	}
	if errors.Is(err, ErrTradeHeld) {
		t.Fatal("a repeat is a failure, not a hold")
	}
	if !errors.Is(err, cause) {
		t.Fatal("the cause must stay reachable through Unwrap")
	}
}

// The chain still failed on this tick, so the trade is backed off exactly as
// for the first failure — live that is what keeps a blocked trade from asking
// the exchange for its balances on every print.
func TestARepeatedFailureStillBacksOff(t *testing.T) {
	const tradeID = uint(910003)

	runChain(tradeID, Repeated(errors.New("Insufficient funds (1 USDT) for the requested action (stopLoss). You need at least 2 USDT to resume this trade.")))

	if backoffDuration(tradeID) == 0 {
		t.Fatal("a repeated failure must record a backoff")
	}
}

// The same refusal on consecutive ticks — regulatePriceChange on an add the
// state machine keeps proposing too close to the last fill, with only the
// prices changing — is logged once per backoff episode. A different failure
// is logged, and a chain that gets through ends the episode, so the failure
// is logged again if it comes back.
func TestTheSameFailureIsLoggedOncePerEpisode(t *testing.T) {
	const tradeID = uint(910004)
	event := Events{Trade: aggragates.Trades{ID: tradeID}}

	first := errors.New("Current price 0.361510 is bigger than last position price 0.362476")
	again := errors.New("Current price 0.361530 is bigger than last position price 0.362476")
	other := errors.New("Insufficient funds (1 USDT) for the requested action (stopLoss). You need at least 2 USDT to resume this trade.")

	if event.repeatsLastFailure(first) {
		t.Fatal("the first failure of an episode is new")
	}
	if !event.repeatsLastFailure(again) {
		t.Fatal("the same refusal at another price is a repeat")
	}

	// The backoff bookkeeping runs between two failures and must keep the memory.
	event.LockTradeWithBackOff()
	if !event.repeatsLastFailure(again) {
		t.Fatal("backing the trade off must not forget the failure it logged")
	}

	if event.repeatsLastFailure(other) {
		t.Fatal("a different failure on the same trade is logged")
	}

	// A chain that gets through closes the episode.
	Events{Trade: aggragates.Trades{ID: tradeID}, EventsNames: []string{"last"}}.Next()
	if event.repeatsLastFailure(other) {
		t.Fatal("after a successful chain the failure is new again")
	}
}
