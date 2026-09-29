package crashguard_test

import (
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates"
	"github.com/giovani-sirbu/mercury/trades/gates/crashguard"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// deepAnchor is the fixture ladder's `buy` anchor: testutil.DeepTrade's last
// fill.
const deepAnchor = 94.0

// armingEvent is the deep fixture ladder arming its next depth (old position
// `buy`, anchored at deepAnchor) on a tick at `tick`.
func armingEvent(trade aggragates.Trades, tick float64) events.Events {
	trade.PositionPrice = tick
	return events.Events{
		Trade:  trade,
		Params: aggragates.Params{OldPosition: "buy", OldPositionPrice: deepAnchor},
	}
}

func slowDecline() aggragates.AIIndicators {
	return aggragates.AIIndicators{HasRegimeVerdict: true, SlowDecline: true}
}

// widenedLevel is the arm level SlowDeclineDepthFactor steps under the
// anchor, in the engines' own arithmetic: percentage = (price − anchor) /
// price, so −(k·p + t) solves to anchor / (1 + (k·p + t)/100).
func widenedLevel(anchor, percentage, tolerance float64) float64 {
	return anchor / (1 + (crashguard.SlowDeclineDepthFactor*percentage+tolerance)/100)
}

// The depth the ladder would arm one step down is held until the price has
// paid SlowDeclineDepthFactor steps, and released on the level itself.
func TestSlowDeclineHoldsTheArmingUntilTheWidenedLevel(t *testing.T) {
	trade := testutil.DeepTrade(true)
	row := trade.StrategyPair.StrategySettings[0]
	level := widenedLevel(deepAnchor, row.Percentage, row.Tolerance)
	ladderArm := deepAnchor / (1 + (row.Percentage+row.Tolerance)/100)

	held := crashguard.ApplyToHold(armingEvent(trade, ladderArm), "stopLoss", slowDecline(), "")
	want := crashguard.SlowDeclineHoldPrefix + ", next depth parked until " + gates.FormatPriceLevel(trade, level)
	if held != want {
		t.Fatalf("arming at the ladder's own level: got %q, want %q", held, want)
	}
	if got := crashguard.ApplyToHold(armingEvent(trade, level*1.0001), "stopLoss", slowDecline(), ""); got != want {
		t.Fatalf("a tick just above the level must stay held, got %q", got)
	}
	if got := crashguard.ApplyToHold(armingEvent(trade, level), "stopLoss", slowDecline(), ""); got != "" {
		t.Fatalf("the level itself releases the depth, got %q", got)
	}
}

// The reason names the level, never the tick: it must be byte-identical for
// as long as the hold stands, or the hold log writes a row on every tick.
func TestSlowDeclineParkedReasonIsStableWhileHeld(t *testing.T) {
	trade := testutil.DeepTrade(true)
	first := crashguard.ApplyToHold(armingEvent(trade, 91.5), "stopLoss", slowDecline(), "")
	second := crashguard.ApplyToHold(armingEvent(trade, 89.25), "stopLoss", slowDecline(), "")
	if first == "" || first != second {
		t.Fatalf("the reason moved with the tick: %q vs %q", first, second)
	}
}

// The step comes from the row the held depth is priced from — the last
// filled entry's own row — not from the base row.
func TestSlowDeclineLevelReadsTheHeldDepthsRow(t *testing.T) {
	trade := testutil.DeepTrade(true)
	trade.StrategyPair.StrategySettings = []aggragates.StrategySettings{
		{Percentage: 1, Tolerance: 0.1},
		{Percentage: 1, Tolerance: 0.1},
		{Percentage: 1, Tolerance: 0.1},
		{Percentage: 3, Tolerance: 0.5},
	}
	level := widenedLevel(deepAnchor, 3, 0.5)

	held := crashguard.ApplyToHold(armingEvent(trade, level*1.001), "stopLoss", slowDecline(), "")
	if !strings.HasSuffix(held, gates.FormatPriceLevel(trade, level)) {
		t.Fatalf("expected the fourth row's level %s, got %q", gates.FormatPriceLevel(trade, level), held)
	}
}

// Without an anchor or a settings row the level cannot be priced and the
// hold fails open, the posture of every price-released gate.
func TestSlowDeclineHoldFailsOpenWithoutALevel(t *testing.T) {
	noSettings := testutil.DeepTrade(true)
	noSettings.StrategyPair.StrategySettings = nil
	if got := crashguard.ApplyToHold(armingEvent(noSettings, 93), "stopLoss", slowDecline(), ""); got != "" {
		t.Fatalf("no settings row must hold nothing, got %q", got)
	}

	noAnchor := armingEvent(testutil.DeepTrade(true), 93)
	noAnchor.Params.OldPositionPrice = 0
	if got := crashguard.ApplyToHold(noAnchor, "stopLoss", slowDecline(), ""); got != "" {
		t.Fatalf("no anchor must hold nothing, got %q", got)
	}
}

// With nothing to say the gate hands back the hold it was given; when it
// speaks, its reason replaces that hold.
func TestApplyToHoldReplacesOnlyWhenItSpeaks(t *testing.T) {
	trade := testutil.DeepTrade(true)
	const earlier = "regime: add not allowed"

	if got := crashguard.ApplyToHold(armingEvent(trade, 93), "stopLoss", aggragates.AIIndicators{}, earlier); got != earlier {
		t.Fatalf("no verdict must keep the incoming hold, got %q", got)
	}
	freeFall := slowDecline()
	freeFall.FreeFall = true
	if got := crashguard.ApplyToHold(armingEvent(trade, 93), "stopLoss", freeFall, earlier); got != crashguard.FreeFallHoldReason {
		t.Fatalf("a free-fall hold must replace the incoming hold, got %q", got)
	}
}
