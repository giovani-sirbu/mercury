package gates

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// No direction, no entry — whatever the flags say. This is the guard that
// keeps a directionless tick out of CheckOldFuturesOrders, which cancels every
// open order on the symbol and closes any position there.
func TestBlocksNewFuturesEntryWithoutADirection(t *testing.T) {
	for _, action := range []string{"", aggragates.ActionHold, "MAYBE"} {
		verdict := aggragates.AIIndicators{AIAction: action}

		if !BlocksNewFuturesEntry(aggragates.StrategyParams{}, verdict) {
			t.Errorf("action %q carries no direction and must block a no-flag strategy", action)
		}
		if !BlocksNewFuturesEntry(aggragates.StrategyParams{CrashGuard: true}, verdict) {
			t.Errorf("action %q must block whatever other flags are on", action)
		}
	}
}

// A real direction opens, and the flagged HOLD vetoes stay as they were.
func TestBlocksNewFuturesEntryLetsADirectionThrough(t *testing.T) {
	for _, action := range []string{aggragates.ActionLong, aggragates.ActionShort} {
		verdict := aggragates.AIIndicators{AIAction: action}

		if BlocksNewFuturesEntry(aggragates.StrategyParams{UseAI: true}, verdict) {
			t.Errorf("a %q verdict must open", action)
		}
	}

	// A pattern HOLD still vetoes, but only for the flag that owns it.
	held := aggragates.AIIndicators{AIAction: aggragates.ActionLong, PatternAction: aggragates.ActionHold}
	if !BlocksNewFuturesEntry(aggragates.StrategyParams{UsePatterns: true}, held) {
		t.Error("a pattern HOLD must veto a UsePatterns strategy")
	}
	if BlocksNewFuturesEntry(aggragates.StrategyParams{UseAI: true}, held) {
		t.Error("a pattern HOLD must not veto a strategy that does not run patterns")
	}
}
