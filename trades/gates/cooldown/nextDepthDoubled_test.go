package cooldown

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// doubledTrade held at a reference of 100, was released above it, and then
// filled the given entries — one distinct exchange order each.
func doubledTrade(inverse bool, fills ...float64) aggragates.Trades {
	trade := testutil.NewHoldTrade("stopLoss", inverse)
	trade = withEvents(trade, activatedEventAt(100), enteredEventAt(102.6))
	side := "BUY"
	if inverse {
		side = "SELL"
	}
	for i, price := range fills {
		trade.History = append(trade.History, aggragates.TradesHistory{
			Type: side, Quantity: 1, Price: price, OrderId: int64(i + 1),
		})
	}
	return trade
}

// The doubling is the one depth after an entry that went to market above
// the reference: no fill yet, a fill below the reference (funds-blocked at
// the release, filled lower later) and a ladder already two deep all keep
// the configured step.
func TestNextDepthDoubledOnlyAfterOneFillAtOrAboveTheReference(t *testing.T) {
	if NextDepthDoubled(doubledTrade(false)) {
		t.Error("no fill yet: nothing to double")
	}
	if !NextDepthDoubled(doubledTrade(false, 102.6)) {
		t.Error("one fill above the reference must double the next depth")
	}
	if !NextDepthDoubled(doubledTrade(false, 100)) {
		t.Error("one fill exactly at the reference still entered at it")
	}
	if NextDepthDoubled(doubledTrade(false, 99.5)) {
		t.Error("a fill below the reference did not enter above it")
	}
	if NextDepthDoubled(doubledTrade(false, 102.6, 100)) {
		t.Error("two fills: the correction was spent on the second depth")
	}

	// An accounting row (a child's profit at the sentinel price) is not a fill.
	accounting := doubledTrade(false, 102.6)
	accounting.History = append(accounting.History, aggragates.TradesHistory{Type: "BUY", Quantity: 3, Price: 1e-13, OrderId: 9})
	if !NextDepthDoubled(accounting) {
		t.Error("an accounting row must not count as the second fill")
	}
}

// Only the entered event says the entry went to market above the reference:
// an armed hold that filled on its bounce entered below it.
func TestNextDepthDoubledNeedsTheEnteredEvent(t *testing.T) {
	waitingOnly := doubledTrade(false, 102.6)
	waitingOnly.StrategyEvents = waitingOnly.StrategyEvents[:1]
	if NextDepthDoubled(waitingOnly) {
		t.Error("without the entered event nothing is doubled")
	}

	noEvents := doubledTrade(false, 102.6)
	noEvents.StrategyEvents = nil
	if NextDepthDoubled(noEvents) {
		t.Error("a trade the gate never held is not doubled")
	}

	bounced := withEvents(doubledTrade(false, 96.65), activatedEventAt(100), armedEventAt(97.4), armedEventAt(96.5))
	if NextDepthDoubled(bounced) {
		t.Error("an entry filled on the bounce entered below the reference")
	}
}

// The inverse ladder sells first: "above the reference" mirrors to at or
// below it.
func TestNextDepthDoubledMirrorsTheInverseLadder(t *testing.T) {
	if !NextDepthDoubled(doubledTrade(true, 97.5)) {
		t.Error("an inverse fill below the reference must double the next depth")
	}
	if NextDepthDoubled(doubledTrade(true, 100.5)) {
		t.Error("an inverse fill above the reference did not enter through it")
	}
}
