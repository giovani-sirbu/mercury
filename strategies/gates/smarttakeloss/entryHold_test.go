package smarttakeloss

import (
	"reflect"
	"testing"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// slowDeclineVerdict is the block sophos serves with the verdict on or off, the
// fill window served either way: sophos read its window.
func slowDeclineVerdict(on bool) aggragates.AIIndicators {
	return aggragates.AIIndicators{SmartTakeLoss: aggragates.SmartTakeLossIndicators{
		QuietDeclineVerdict:     on,
		QuietDeclineFillFrom:    fillWindowFrom.UnixMilli(),
		QuietDeclineExitReasons: slowDeclineReasons,
	}}
}

// The first fill is held on the flag, a parent, a long entry and the verdict
// — all four, or nothing is held — and the refusal names its row's text and
// the held event that goes beside it, filed under the entry hold's gate.
func TestEntryHoldHoldsALongParentWhileTheVerdictStands(t *testing.T) {
	trade := testutil.LadderTrade(false)
	hold := EntryHold(trade, aggragates.SideLong, slowDeclineVerdict(true))
	if hold.Reason != SlowDeclineEntryHoldReason || !hold.Held() {
		t.Fatalf("a long parent entry under the verdict must be held, got %+v", hold)
	}
	if hold.Param != aggragates.StrategyParamSmartTakeLoss || hold.Gate != GateEntryHold || !reflect.DeepEqual(hold.Data, EventData{Event: gates.EventHeld}) {
		t.Fatalf("the hold names the held event of the entry hold gate, got %+v", hold)
	}

	if got := EntryHold(trade, aggragates.SideLong, slowDeclineVerdict(false)); got.Held() {
		t.Fatalf("the fill window alone holds nothing, got %+v", got)
	}
	for _, side := range []string{aggragates.SideShort, ""} {
		if got := EntryHold(trade, side, slowDeclineVerdict(true)); got.Held() {
			t.Fatalf("side %q must not be held, got %+v", side, got)
		}
	}

	child := testutil.LadderTrade(false)
	child.ParentID = 7
	if got := EntryHold(child, aggragates.SideLong, slowDeclineVerdict(true)); got.Held() {
		t.Fatalf("an impasse child belongs to the impasse chain, got %+v", got)
	}

	off := testutil.LadderTrade(false)
	off.Strategy.Params = aggragates.StrategyParams{DynamicParams: true, UsePatterns: true}
	if got := EntryHold(off, aggragates.SideLong, slowDeclineVerdict(true)); got.Held() {
		t.Fatalf("a verdict fetched for another flag holds nothing, got %+v", got)
	}
}

// A refusal that is not a hold is the zero Hold, whole: no reason and no
// event to write beside a row that is never written.
func TestEntryHoldThatHoldsNothingIsTheZeroHold(t *testing.T) {
	trade := testutil.LadderTrade(false)
	if got := EntryHold(trade, aggragates.SideLong, slowDeclineVerdict(false)); !reflect.DeepEqual(got, gates.Hold{}) {
		t.Fatalf("no verdict, no hold at all, got %+v", got)
	}
}

// The hold is the one row of the flag that is a refusal, so it keeps the text
// the first-fill hold has always carried, in the quiet decline's words; the
// frame tradelog.SaveHoldLog puts before it ("Hold entry: ") is the gate's.
func TestEntryHoldReasonIsPinned(t *testing.T) {
	if SlowDeclineEntryHoldReason != "smartTakeLoss: quiet decline, first fill held" {
		t.Fatalf("the entry hold's reason is byte-stable text and must not move, got %q", SlowDeclineEntryHoldReason)
	}
}

// Switched off, the quiet decline holds no first fill, the verdict
// standing or not; switched back on, the same entry is held. The swing decline
// and the early take profit hold no first fill whatever they are switched to.
func TestEntryHoldHoldsNothingWhileSwitchedOff(t *testing.T) {
	trade := testutil.LadderTrade(false)
	withQuietDecline(t, false)
	if got := EntryHold(trade, aggragates.SideLong, slowDeclineVerdict(true)); got.Held() {
		t.Fatalf("switched off, the verdict must hold nothing, got %+v", got)
	}
	withQuietDecline(t, true)
	withSwingDecline(t, false)
	withEarlyTakeProfit(t, false)
	if got := EntryHold(trade, aggragates.SideLong, slowDeclineVerdict(true)); got.Reason != SlowDeclineEntryHoldReason {
		t.Fatalf("the other features' switches leave the hold as it was, got %+v", got)
	}
	withSwingDecline(t, true)
	withEarlyTakeProfit(t, true)
	if got := EntryHold(trade, aggragates.SideLong, slowDeclineVerdict(true)); got.Reason != SlowDeclineEntryHoldReason {
		t.Fatalf("control: switched on, the verdict holds the first fill, got %+v", got)
	}
}

// The hold is one constant whatever the reasons sophos served, so
// tradelog.SaveHoldLog collapses it tick to tick: the reason, the event and its
// document are the same.
func TestEntryHoldIsConstant(t *testing.T) {
	other := slowDeclineVerdict(true)
	other.SmartTakeLoss.QuietDeclineExitReasons = []string{"leg down 9.9% from its high close"}
	trade := testutil.LadderTrade(false)
	if !reflect.DeepEqual(EntryHold(trade, aggragates.SideLong, other), EntryHold(trade, aggragates.SideLong, slowDeclineVerdict(true))) {
		t.Fatal("the hold must not carry the verdict's reasons")
	}
}

// The first fill is held on the verdict alone. The leg on and quiet without
// it — served with the bar it reads smooth from and the bar a fill has to
// precede — reads a ladder's own smoothness from its newest fill, and a pair
// with no ladder has no fill to count it from: nothing is held until the
// verdict comes.
func TestEntryHoldIgnoresTheLegOnAndQuietWithoutTheVerdict(t *testing.T) {
	trade := testutil.LadderTrade(false)
	quiet := slowDeclineVerdict(false)
	quiet.SmartTakeLoss.QuietDeclineLegQuiet = true
	quiet.SmartTakeLoss.QuietDeclineSmoothFrom = 1_640_646_000_000
	quiet.SmartTakeLoss.QuietDeclineFillBefore = 1_640_732_400_000
	if got := EntryHold(trade, aggragates.SideLong, quiet); got.Held() {
		t.Fatalf("the leg on and quiet without the verdict must hold nothing, got %+v", got)
	}
	quiet.SmartTakeLoss.QuietDeclineVerdict = true
	if got := EntryHold(trade, aggragates.SideLong, quiet); got.Reason != SlowDeclineEntryHoldReason {
		t.Fatalf("fixture drifted: the same reading with the verdict must hold, got %+v", got)
	}
}

// The first-fill hold reads the verdict alone: the fill window gates going
// pending, and a pair with no ladder has no fill to weigh against it, so the
// hold stands with no window served and with one past every stamp.
func TestEntryHoldIgnoresTheFillWindow(t *testing.T) {
	trade := testutil.LadderTrade(false)
	for _, from := range []int64{0, testutil.At("23:59:00").UnixMilli()} {
		verdict := slowDeclineVerdict(true)
		verdict.SmartTakeLoss.QuietDeclineFillFrom = from
		if got := EntryHold(trade, aggragates.SideLong, verdict); got.Reason != SlowDeclineEntryHoldReason {
			t.Fatalf("window from %d: the verdict holds the first fill, got %+v", from, got)
		}
	}
}
