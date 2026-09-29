package smarttakeloss

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

func slowDeclineVerdict(on bool) aggragates.AIIndicators {
	return aggragates.AIIndicators{SmartTakeLoss: aggragates.SmartTakeLossIndicators{
		SlowDeclineExit:        on,
		SlowDeclineSellBand:    slowDeclineBand,
		SlowDeclineExitReasons: slowDeclineReasons,
	}}
}

// The first fill is held on the flag, a parent, a long entry and the verdict
// — all four, or nothing is held.
func TestEntryHoldReasonHoldsALongParentWhileTheVerdictStands(t *testing.T) {
	trade := testutil.LadderTrade(false)
	if got := EntryHoldReason(trade, aggragates.SideLong, slowDeclineVerdict(true)); got != SlowDeclineEntryHoldReason {
		t.Fatalf("a long parent entry under the verdict must be held, got %q", got)
	}

	if got := EntryHoldReason(trade, aggragates.SideLong, slowDeclineVerdict(false)); got != "" {
		t.Fatalf("the band alone holds nothing, got %q", got)
	}
	for _, side := range []string{aggragates.SideShort, ""} {
		if got := EntryHoldReason(trade, side, slowDeclineVerdict(true)); got != "" {
			t.Fatalf("side %q must not be held, got %q", side, got)
		}
	}

	child := testutil.LadderTrade(false)
	child.ParentID = 7
	if got := EntryHoldReason(child, aggragates.SideLong, slowDeclineVerdict(true)); got != "" {
		t.Fatalf("an impasse child belongs to the impasse chain, got %q", got)
	}

	off := testutil.LadderTrade(false)
	off.Strategy.Params = aggragates.StrategyParams{DynamicParams: true, UsePatterns: true}
	if got := EntryHoldReason(off, aggragates.SideLong, slowDeclineVerdict(true)); got != "" {
		t.Fatalf("a verdict fetched for another flag holds nothing, got %q", got)
	}
}

// Switched off, the quiet slow decline holds no first fill, the verdict
// standing or not; switched back on, the same entry is held.
func TestEntryHoldReasonHoldsNothingWhileSwitchedOff(t *testing.T) {
	trade := testutil.LadderTrade(false)
	withQuietSlowDeclineExit(t, false)
	if got := EntryHoldReason(trade, aggragates.SideLong, slowDeclineVerdict(true)); got != "" {
		t.Fatalf("switched off, the verdict must hold nothing, got %q", got)
	}
	withQuietSlowDeclineExit(t, true)
	if got := EntryHoldReason(trade, aggragates.SideLong, slowDeclineVerdict(true)); got != SlowDeclineEntryHoldReason {
		t.Fatalf("control: switched on, the verdict holds the first fill, got %q", got)
	}
}

// The reason is one constant whatever the reasons sophos served, so
// gates.SaveHoldLog collapses it tick to tick.
func TestEntryHoldReasonIsConstant(t *testing.T) {
	other := slowDeclineVerdict(true)
	other.SmartTakeLoss.SlowDeclineExitReasons = []string{"leg down 9.9% from its high close"}
	trade := testutil.LadderTrade(false)
	if EntryHoldReason(trade, aggragates.SideLong, other) != EntryHoldReason(trade, aggragates.SideLong, slowDeclineVerdict(true)) {
		t.Fatal("the hold reason must not carry the verdict's reasons")
	}
}

// The first fill is held on the verdict alone. The leg on and quiet without
// it — served with the bar it reads smooth from and the bar a fill has to
// precede — reads a ladder's own smoothness from its newest fill, and a pair
// with no ladder has no fill to count it from: nothing is held until the
// verdict comes.
func TestEntryHoldReasonIgnoresTheLegOnAndQuietWithoutTheVerdict(t *testing.T) {
	trade := testutil.LadderTrade(false)
	quiet := slowDeclineVerdict(false)
	quiet.SmartTakeLoss.SlowDeclineLegQuiet = true
	quiet.SmartTakeLoss.SlowDeclineSmoothFrom = 1_640_646_000_000
	quiet.SmartTakeLoss.SlowDeclineFillBefore = 1_640_732_400_000
	if got := EntryHoldReason(trade, aggragates.SideLong, quiet); got != "" {
		t.Fatalf("the leg on and quiet without the verdict must hold nothing, got %q", got)
	}
	quiet.SmartTakeLoss.SlowDeclineExit = true
	if got := EntryHoldReason(trade, aggragates.SideLong, quiet); got != SlowDeclineEntryHoldReason {
		t.Fatalf("fixture drifted: the same reading with the verdict must hold, got %q", got)
	}
}

// The first-fill hold reads the verdict alone: the fill window gates going
// pending, and a pair with no ladder has no fill to weigh against it, so the
// hold stands with no window served and with one past every stamp.
func TestEntryHoldReasonIgnoresTheFillWindow(t *testing.T) {
	trade := testutil.LadderTrade(false)
	for _, from := range []int64{0, testutil.At("23:59:00").UnixMilli()} {
		verdict := slowDeclineVerdict(true)
		verdict.SmartTakeLoss.SlowDeclineFillFrom = from
		if got := EntryHoldReason(trade, aggragates.SideLong, verdict); got != SlowDeclineEntryHoldReason {
			t.Fatalf("window from %d: the verdict holds the first fill, got %q", from, got)
		}
	}
}
