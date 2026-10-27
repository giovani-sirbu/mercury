package smarttakeloss

import (
	"testing"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/earlytakeprofit"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// The early take profit stands latched from its latched row on, a new depth included; never on
// a watched ladder with no latch, on a trade without the flag, on a child or with earlytakeprofit.Enabled off.
func TestEarlyTakeProfitLatchedFromTheLatchedRowOn(t *testing.T) {
	withEarlyTakeProfit(t, true)
	withQuietDecline(t, true)
	watched := watchedTrade()
	if EarlyTakeProfitLatched(watched) {
		t.Fatal("a watched ladder with no latched row stands latched on nothing")
	}
	latched := latchedBy(watched)
	if !EarlyTakeProfitLatched(latched) {
		t.Fatal("a latched ladder stands latched")
	}
	if !EarlyTakeProfitLatched(withFill(latched, latched.PositionPrice-1, testutil.At("23:00:00"))) {
		t.Fatal("a new depth takes the latch away from nothing")
	}

	off := latched
	off.Strategy.Params.SmartTakeLoss = false
	child := latched
	child.ParentID = 1
	for name, trade := range map[string]aggragates.Trades{"flag off": off, "child": child} {
		if EarlyTakeProfitLatched(trade) {
			t.Errorf("%s: stands latched on nothing", name)
		}
	}

	withEarlyTakeProfit(t, false)
	if EarlyTakeProfitLatched(latched) {
		t.Fatal("with earlytakeprofit.Enabled off the direction watches no ladder and stands latched on nothing")
	}
}

// A ladder latched before its first fill (earlytakeprofit.ArmDepth 0) is latched once: its latched
// row carries no fill price, the fold reads it all the same, and the next tick writes nothing.
// Trade 187880 of backtest 313 wrote the row on every print until the trade's UPDATE outgrew
// Postgres' parameter limit.
func TestAnUnfilledLadderIsLatchedOnce(t *testing.T) {
	if earlytakeprofit.ArmDepth > 0 {
		t.Skipf("earlytakeprofit.ArmDepth %d: an unfilled ladder is unwatched", earlytakeprofit.ArmDepth)
	}
	withEarlyTakeProfit(t, true)
	trade := testutil.LadderTrade(false)
	trade.PositionType = "new"
	reads := withBlock(aggragates.SmartTakeLossIndicators{Indecision: true, QuietDeclineBreakReasons: indecisionReasons})

	first := ApplyAt(trade, "", 100, reads, testutil.At("21:45:00"))
	if len(first.Rows) != 1 || first.Rows[0].Event != EventLatched || first.Rows[0].Price != 0 {
		t.Fatalf("the first tick latches on one row without a fill price, got %+v", first.Rows)
	}
	trade = withRow(trade, first.Rows[0], testutil.At("21:45:00"))
	if !EarlyTakeProfitLatched(trade) {
		t.Fatal("the latched row without a price stands latched")
	}
	if again := ApplyAt(trade, "", 100, reads, testutil.At("21:45:05")); len(again.Rows) != 0 {
		t.Fatalf("a latched ladder is latched again on nothing, got %+v", again.Rows)
	}
}
