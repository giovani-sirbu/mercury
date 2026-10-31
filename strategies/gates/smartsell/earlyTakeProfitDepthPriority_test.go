package smartsell

import (
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// withEarlyTakeProfitDepthPriority is withIndecision for the latch on a depth priority hold.
func withEarlyTakeProfitDepthPriority(t *testing.T, on bool) {
	t.Helper()
	previous := earlyTakeProfitDepthPriority
	earlyTakeProfitDepthPriority = on
	t.Cleanup(func() { earlyTakeProfitDepthPriority = previous })
}

// holdReasons is what a latch row names of heldBy's hold.
var holdReasons = []string{"ETH/USDT at depth 5 of 8 holds this ladder at depth 4 of 8"}

// noReading is a block no other latch reads on.
func noReading() aggragates.AIIndicators {
	return withBlock(aggragates.SmartSellIndicators{})
}

// A watched ladder a depth priority holds is latched on the hold alone: one row at the newest
// fill's price naming the hold, the proposal untouched; sophos' reading, when served, names the row
// instead. From the next tick it reads latched and held, writes nothing more, and its take profit
// reads its position price.
func TestADepthPriorityHoldLatchesTheEarlyTakeProfit(t *testing.T) {
	withEarlyTakeProfit(t, true)
	withEarlyTakeProfitDepthPriority(t, true)
	trade := indecisionLadder()
	newest := rebuildState(trade).lastFill().Price
	assertUntouched(t, Apply(trade, "", underTheBand, noReading()), "")

	held := heldBy(trade, testutil.At("18:00:00"))
	want := LatchedRow(newest, Finding{Feature: FeatureDepthPriority, Reasons: holdReasons})
	if want.Message != "smartSell: early take profit latched (depth priority: ETH/USDT at depth 5 of 8 holds this ladder at depth 4 of 8)" {
		t.Fatalf("the row names the hold, got %q", want.Message)
	}
	for _, position := range []string{"", "trailingBuy", "takeProfit"} {
		got := Apply(held, position, underTheBand, noReading())
		assertRow(t, got, want)
		assertNoSale(t, got, position)
	}
	assertRow(t, Apply(held, "", underTheBand, indecisionReading()), LatchedRow(newest, indecisionFinding(indecisionReasons)))

	latched, _ := engineTick(held, "", underTheBand, testutil.At("18:10:00"), noReading())
	if st := rebuildState(latched); !st.earlyTakeProfit || !st.depthPriorityHeld {
		t.Fatalf("the tick after: latched and held, got %+v", st)
	}
	assertUntouched(t, Apply(latched, "", underTheBand, noReading()), "")
	if carriedLatches(latched) != 1 {
		t.Fatalf("one event carries the latch, got %+v", latched.StrategyEvents)
	}
	price, fromAverage, fromPosition := overBreakEven(latched)
	if got := TakeProfitPercentage(held, price, fromAverage); got != fromAverage {
		t.Fatalf("before the latch the take profit reads the average entry %v, got %v", fromAverage, got)
	}
	if got := TakeProfitPercentage(latched, price, fromAverage); got != fromPosition {
		t.Fatalf("latched, the take profit reads the position price %v, got %v", fromPosition, got)
	}
}

// The latch reads the hold whether or not it pauses the gates: with PauseWhenActiveDepthPriority
// off the ladder is not held, and the hold still latches it on the same row.
func TestTheDepthPriorityLatchIgnoresThePause(t *testing.T) {
	withEarlyTakeProfit(t, true)
	withEarlyTakeProfitDepthPriority(t, true)
	withDepthPriorityPause(t, false)
	trade := indecisionLadder()
	held := heldBy(trade, testutil.At("18:00:00"))
	if rebuildState(held).depthPriorityHeld {
		t.Fatal("fixture drifted: with the pause off the ladder is not held")
	}
	assertRow(t, Apply(held, "", underTheBand, noReading()), LatchedRow(rebuildState(trade).lastFill().Price, Finding{Feature: FeatureDepthPriority, Reasons: holdReasons}))
}

// A held pending ladder is reset and latched on the one reset row, the hold's reasons under the
// indecision, and reads latched and not pending from the next tick.
func TestAHeldPendingLadderIsResetAndLatchedOnTheHold(t *testing.T) {
	withQuietDecline(t, true)
	withEarlyTakeProfit(t, true)
	withDepthPriorityPause(t, true)
	withEarlyTakeProfitDepthPriority(t, true)
	want := ResetRow(slowDeclineLastFill, latchMarker, Finding{Feature: FeatureDepthPriority, Reasons: holdReasons}, quietFinding(nil))
	if want.Message != "smartSell: early take profit latched, slow decline reset, depth priority holds this ladder (depth priority: ETH/USDT at depth 5 of 8 holds this ladder at depth 4 of 8)" {
		t.Fatalf("the reset row names the latch first, got %q", want.Message)
	}
	trade, got := engineTick(heldBy(pendingTrade(), testutil.At("18:05:00")), "", underTheBand, testutil.At("18:10:00"), noReading())
	assertRow(t, got, want)
	assertNoSale(t, got, "")
	if st := rebuildState(trade); !st.earlyTakeProfit || st.slowDeclinePending {
		t.Fatalf("the tick after: latched, not pending, got %+v", st)
	}
}

// Nothing else latches on a hold: the switch off, the indecision off, a hold stamped before the
// newest fill or without a stamp, and a ladder already latched.
func TestTheDepthPriorityLatchNeedsAStandingHoldOnAWatchedLadder(t *testing.T) {
	withEarlyTakeProfit(t, true)
	withEarlyTakeProfitDepthPriority(t, true)
	trade := indecisionLadder()
	held := heldBy(trade, testutil.At("18:00:00"))
	cases := map[string]aggragates.Trades{
		"a hold before the newest fill": withFill(held, w3sPrice(indecisionFills+1), testutil.At("18:30:00")),
		"an unstamped hold":             heldBy(trade, time.Time{}),
		"a latched ladder":              heldBy(latchedBy(trade), testutil.At("22:00:00")),
	}
	for name, candidate := range cases {
		t.Run(name, func(t *testing.T) {
			assertUntouched(t, Apply(candidate, "", underTheBand, noReading()), "")
		})
	}
	t.Run("the switch off", func(t *testing.T) {
		withEarlyTakeProfitDepthPriority(t, false)
		assertUntouched(t, Apply(held, "", underTheBand, noReading()), "")
	})
	t.Run("the indecision off", func(t *testing.T) {
		withEarlyTakeProfit(t, false)
		assertUntouched(t, Apply(held, "", underTheBand, noReading()), "")
	})
}
