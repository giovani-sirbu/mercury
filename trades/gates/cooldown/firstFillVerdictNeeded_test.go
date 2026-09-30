package cooldown

import (
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// The events the gate leaves behind, as firstFillState reads them: the kind
// and the Price matter, the rest does not. Built with the constructor the gate
// itself writes them with, and unstamped, like an event no engine stamped: the
// fold never expires a first fill on it.
func activatedEventAt(price float64) aggragates.TradesStrategyEvents {
	return NewFirstFillEvent(0, FirstFillEvent{Event: FirstFillActivated, Price: price, Reference: price}, time.Time{})
}

func armedEventAt(price float64) aggragates.TradesStrategyEvents {
	return NewFirstFillEvent(0, FirstFillEvent{Event: FirstFillArmed, Price: price, Anchor: price}, time.Time{})
}

func enteredEventAt(price float64) aggragates.TradesStrategyEvents {
	return NewFirstFillEvent(0, FirstFillEvent{Event: FirstFillEntered, Price: price}, time.Time{})
}

// withEvents is the trade carrying exactly these strategy events, in order.
func withEvents(trade aggragates.Trades, stored ...aggragates.TradesStrategyEvents) aggragates.Trades {
	trade.StrategyEvents = stored
	return trade
}

// On spot the verdict is fetched for a new trade until the hold stands, and
// never again once it does or once the entry was released above the
// reference.
func TestFirstFillVerdictNeededOnlyBeforeTheHoldStands(t *testing.T) {
	trade := testutil.NewHoldTrade("buy", false)
	if !FirstFillVerdictNeeded(trade, "new") {
		t.Fatal("a new trade with no events needs the verdict")
	}
	for _, position := range []string{"buy", "stopLoss", "takeProfit", ""} {
		if FirstFillVerdictNeeded(trade, position) {
			t.Fatalf("position %q has no first fill to judge", position)
		}
	}

	activated := withEvents(trade, activatedEventAt(100))
	if FirstFillVerdictNeeded(activated, "new") {
		t.Fatal("a standing hold is priced, not judged again")
	}

	entered := withEvents(trade, activatedEventAt(100), enteredEventAt(102.6))
	if FirstFillVerdictNeeded(entered, "new") {
		t.Fatal("an entry released above the reference is not judged again")
	}

	// An entered event alone finishes the gate with the trade, whatever else
	// the trade carries: it is checked before the price filter.
	enteredAlone := withEvents(trade, enteredEventAt(0))
	if FirstFillVerdictNeeded(enteredAlone, "new") {
		t.Fatal("an entered event needs no price to finish the gate")
	}

	// An activated event without a price carries no reference: not activated.
	unpriced := withEvents(trade, activatedEventAt(0))
	if !FirstFillVerdictNeeded(unpriced, "new") {
		t.Fatal("an event without a price is no activation")
	}

	// Nor is an armed event a standing hold on its own: only the activation
	// stops the verdict from being fetched.
	armedOnly := withEvents(trade, armedEventAt(97.4))
	if !FirstFillVerdictNeeded(armedOnly, "new") {
		t.Fatal("an armed event without its activation is no standing hold")
	}
}

// Futures keep the verdict-only gate: every tick of a new trade fetches,
// whatever the events say.
func TestFirstFillVerdictNeededOnFuturesFollowsThePositionAlone(t *testing.T) {
	trade := testutil.NewHoldTrade("buy", false)
	trade.Strategy.TradeType = aggragates.Futures
	trade = withEvents(trade, activatedEventAt(100), enteredEventAt(102.6))

	if !FirstFillVerdictNeeded(trade, "new") {
		t.Fatal("a new futures trade fetches the verdict on every tick")
	}
	if FirstFillVerdictNeeded(trade, "stopLoss") {
		t.Fatal("an open futures position has no first fill to judge")
	}
}
