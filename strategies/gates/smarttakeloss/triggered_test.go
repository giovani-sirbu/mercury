package smarttakeloss

import (
	"testing"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// The slow decline gate stands triggered from its pending row until a new fill cancels it or a
// depth priority resets it, and never on a decline switched off.
func TestTriggeredWhileTheSlowDeclineGateIsPending(t *testing.T) {
	withQuietDecline(t, true)
	if Triggered(watchedTrade()) {
		t.Fatal("a watched ladder with no event stands triggered on nothing")
	}
	pending := pendingTrade()
	if !Triggered(pending) {
		t.Fatal("a pending ladder stands triggered")
	}
	for name, row := range map[string]Row{
		"cancelled": CancelledRow(slowDeclineLastFill, quietFinding(nil)),
		"reset":     ResetRow(slowDeclineLastFill, quietFinding(nil)),
	} {
		if Triggered(withRows(pending, row)) {
			t.Errorf("%s: the gate no longer stands triggered", name)
		}
	}

	withQuietDecline(t, false)
	if Triggered(pending) {
		t.Fatal("a decline switched off watches no ladder and stands triggered on nothing")
	}
}

// A flat market episode stands triggered while open at the newest fill with flatmarket.Enabled
// on, depth priority pause or not; a reset, the sale or a new depth past it leaves nothing.
func TestTriggeredWhileAFlatMarketEpisodeIsOpen(t *testing.T) {
	withFlatMarket(t, true)
	withDepthPriorityPause(t, true)
	trade := flatMarketLadder(false)
	fill := trade.PositionPrice
	const deeper = 96.0
	open := withRows(trade, FlatMarketTriggeredRow(fill, nil))
	if !Triggered(open) {
		t.Fatal("an episode open at the newest fill stands triggered")
	}
	if !Triggered(heldBy(open, testutil.At("23:00:00"))) {
		t.Fatal("the depth priority pause holds the sale, not the episode")
	}

	newDepth := withFill(open, deeper, testutil.At("14:00:00"))
	for name, closed := range map[string]aggragates.Trades{
		"no episode":         trade,
		"sold":               withRows(open, ExitRow(trade, reasonFlatMarket, fill)),
		"a new depth filled": newDepth,
		"reset at the depth": withRows(newDepth, FlatMarketResetRow(deeper)),
	} {
		if Triggered(closed) {
			t.Errorf("%s: no episode stands at the newest fill", name)
		}
	}

	withFlatMarket(t, false)
	if Triggered(open) {
		t.Fatal("with flatmarket.Enabled off the gate watches no ladder and stands triggered on nothing")
	}
}

// A capital protection episode stands triggered while open at the newest fill, never once reset or
// past a new depth.
func TestTriggeredWhileACapitalProtectionEpisodeIsOpen(t *testing.T) {
	capitalProtectionAlone(t)
	trade := lastDepthLadder()
	open := withRows(trade, capitalProtectionTriggered(trade))
	if !Triggered(open) {
		t.Fatal("an episode open at the newest fill stands triggered")
	}
	if Triggered(withRows(open, CapitalProtectionResetRow(trade.PositionPrice))) {
		t.Fatal("a reset episode stands triggered on nothing")
	}
	if Triggered(withFill(open, trade.PositionPrice-1, testutil.At("23:00:00"))) {
		t.Fatal("an episode a new depth passed stands triggered on nothing")
	}
}

// The indecision latch only moves the take profit, so it triggers nothing; neither does any gate
// of a trade without the flag or of a child.
func TestTriggeredIgnoresTheLatchTheFlagOffAndChildren(t *testing.T) {
	withEarlyTakeProfit(t, true)
	withQuietDecline(t, true)
	if latched := latchedBy(watchedTrade()); Triggered(latched) {
		t.Fatal("a latched ladder stands triggered on nothing")
	}

	off := pendingTrade()
	off.Strategy.Params.SmartTakeLoss = false
	child := pendingTrade()
	child.ParentID = 1
	for name, trade := range map[string]aggragates.Trades{"flag off": off, "child": child} {
		if Triggered(trade) {
			t.Errorf("%s: stands triggered on nothing", name)
		}
	}
}
