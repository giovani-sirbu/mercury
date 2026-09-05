package smarttakeloss

import (
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/crashguard"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// protectedPositions are the ladder decisions this overlay must never replace.
// addPositions are the ones it exists to override.
var (
	protectedPositions = []string{"sell", "takeProfit", "update_takeProfit", "sellParent", "impasse", "sellLoss"}
	addPositions       = []string{"buy", "stopLoss", "update_stopLoss"}
)

// The risk trigger forces an exit instead of adding capital. On a close the
// ladder already priced as profitable there is no add to prevent: forcing
// sellLoss over it swaps hasProfit for acceptLoss and sells below break even,
// and forcing the trail defers a close that can then no longer sell at all.
func TestApplyProtectiveTickNeverOverridesADecidedClose(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()

	for _, position := range protectedPositions {
		trade := testutil.DeepLadderTrade(9, false)
		trade.Strategy.Params.SmartTakeLoss = true
		trade.CreatedAt = now

		got := ApplyProtectiveTick(trade, position, 90, now, highRiskAI(false), false)

		if got.Position != position {
			t.Errorf("high risk rewrote %q into %q", position, got.Position)
		}
		if got.STLForced {
			t.Errorf("%q must not be recorded as a forced exit", position)
		}
	}
}

// The same set survives the stale cut, which used to exempt takeProfit alone
// and so turned the other five into a below-break-even close.
func TestApplyProtectiveTickStaleCutNeverOverridesADecidedClose(t *testing.T) {
	for _, position := range protectedPositions {
		trade := testutil.DeepLadderTrade(crashguard.DeRiskMinDepth, false)
		trade.Strategy.Params.SmartTakeLoss = true
		trade.CreatedAt = time.Unix(1_700_000_000, 0).UTC()
		now := trade.CreatedAt.Add(StaleAfter)

		got := ApplyProtectiveTick(trade, position, 90, now, aggragates.AIIndicators{}, false)

		if got.Position != position {
			t.Errorf("the stale cut rewrote %q into %q", position, got.Position)
		}
		if got.STLForced || got.Eval.StaleCut {
			t.Errorf("%q must not be cut as a stale bag", position)
		}
	}
}

// The add side is what the overlay is for, so it must still be overridden —
// otherwise the guard above would have disabled the feature.
func TestApplyProtectiveTickStillOverridesTheAddSide(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()

	for _, position := range addPositions {
		trade := testutil.DeepLadderTrade(9, false)
		trade.Strategy.Params.SmartTakeLoss = true
		trade.CreatedAt = now

		got := ApplyProtectiveTick(trade, position, 90, now, highRiskAI(false), false)

		if !got.STLForced || got.Position == position {
			t.Errorf("high risk must force an exit over %q, got %+v", position, got)
		}
	}
}
