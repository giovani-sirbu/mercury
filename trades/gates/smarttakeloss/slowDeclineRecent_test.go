package smarttakeloss

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// A watched ladder goes pending on the decline read recently: a bar among the
// last closed ones on which the verdict stood, and the ladder's newest fill
// stamped at or after the oldest of them. The marker row carries the newest
// fill's price and names the recent bar's reasons, and the ladder is pending
// from that fill; under the band nothing sells, and at it the ladder goes
// pending and sells on the same tick.
func TestApplyGoesPendingOnTheDeclineReadRecently(t *testing.T) {
	newest, stood := testutil.At("17:38:00"), testutil.At("16:00:00")
	marker := SlowDeclineMessage("buy", slowDeclineRecentReasons)
	for name, from := range map[string]time.Time{
		"an oldest bar before the fill's": testutil.At("09:00:00"),
		"the fill's own bar":              testutil.At("17:00:00"),
		"the fill's very stamp":           newest,
	} {
		got := Apply(watchedTrade(), "", underTheBand, withRecent(brokenAtTheBand(), stood, from))
		if got.SlowDecline == nil || got.SlowDecline.Message != marker || got.SlowDecline.Price != slowDeclineLastFill || got.Position != "" || got.Reason != "" {
			t.Fatalf("%s: the ladder must go pending at its newest fill without forcing, got %+v", name, got)
		}
		pending := withRow(watchedTrade(), *got.SlowDecline, testutil.At("18:05:00"))
		if st := rebuildState(pending); !st.slowDeclinePending || st.slowDeclinePendingFrom != slowDeclineLastFill {
			t.Fatalf("%s: the row makes the ladder pending from its newest fill, got %+v", name, st)
		}
		sold := Apply(watchedTrade(), "stopLoss", slowDeclineBand, withRecent(brokenAtTheBand(), stood, from))
		assertRow(t, sold.SlowDecline, marker, slowDeclineLastFill)
		assertForced(t, sold, reasonSellBand)
	}
}

// The recent path fails closed: a newest fill before the oldest bar sophos
// looks back over — the ladder bought no depth within those bars — or a
// recent bar or an oldest bar not served marks nothing and sells nothing, and
// neither does a newest fill without a stamp, which the verdict no longer
// marks either: the recent-fill rule fails closed on it. A ladder short of
// SlowDeclineArmDepth is not watched, and a ladder already pending writes no
// second row.
func TestApplyReadsTheDeclineRecentlyOnlyFromAFillWithinTheLookBack(t *testing.T) {
	newest := testutil.At("17:38:00")
	stood, from := testutil.At("16:00:00"), testutil.At("17:00:00")
	for name, reading := range map[string]aggragates.AIIndicators{
		"a fill before the oldest bar": withRecent(brokenAtTheBand(), stood, newest.Add(time.Millisecond)),
		"no recent bar":                withRecent(brokenAtTheBand(), time.Time{}, from),
		"no oldest bar":                withRecent(brokenAtTheBand(), stood, time.Time{}),
	} {
		for _, price := range []float64{underTheBand, slowDeclineBand} {
			if got := Apply(watchedTrade(), "", price, reading); got.SlowDecline != nil || got.Reason != "" {
				t.Fatalf("%s at %v: nothing is marked or sold, got %+v", name, price, got)
			}
		}
	}

	unstamped := watchedTrade()
	for index := range unstamped.History {
		unstamped.History[index].CreatedAt = time.Time{}
	}
	assertNoSlowDeclineRow(t, Apply(unstamped, "", underTheBand, withRecent(brokenAtTheBand(), stood, from)), "")
	assertNoSlowDeclineRow(t, Apply(unstamped, "", underTheBand, withRecent(slowDeclineBlock(true), stood, from)), "")

	shallower := testutil.LadderTrade(false, fills(SlowDeclineArmDepth-1, "17:38:00")...)
	assertNoSlowDeclineRow(t, Apply(shallower, "", underTheBand, withRecent(brokenAtTheBand(), stood, from)), "")
	assertNoSlowDeclineRow(t, Apply(pendingTrade(), "", underTheBand, withRecent(brokenAtTheBand(), stood, from)), "")
}

// A fill in the bar still forming counts on the recent path: nothing bounds
// the stamp from above. A fresh fill the last closed bar reads nothing for
// (TestApplyAFreshFillOnAQuietLegNeitherGoesPendingNorSells) goes pending on
// the recent bar, naming it, once the ladder is watched.
func TestApplyGoesPendingOnAFreshFillReadRecently(t *testing.T) {
	lastClosed := testutil.At("16:00:00")
	reading := withRecent(quietLegBlock(false, lastClosed, testutil.At("17:00:00")), lastClosed, testutil.At("00:00:00"))
	for _, depth := range []int{SlowDeclineArmDepth - 1, SlowDeclineArmDepth, watchedFills} {
		t.Run(fmt.Sprintf("%d fills", depth), func(t *testing.T) {
			trade := testutil.LadderTrade(false, fills(depth, "17:38:00")...)
			got := Apply(trade, "", underTheBand, reading)
			if depth < SlowDeclineArmDepth {
				assertNoSlowDeclineRow(t, got, "")
				return
			}
			if depth < 2 {
				// A single fill is the ladder's first fill too, stamped after the
				// recent bar, and the recent path never marks a ladder opened after
				// the bar (TestApplyGoesPendingRecentlyOnlyOnALadderThatStoodBeforeTheBar).
				t.Skipf("SlowDeclineArmDepth is %d: the watched ladder's only fill is its first, not before the recent bar", SlowDeclineArmDepth)
			}
			assertRow(t, got.SlowDecline, SlowDeclineMessage("buy", slowDeclineRecentReasons), trade.PositionPrice)
			assertNoSale(t, got, "")
		})
	}
}

// The last closed bar's own reading comes first: when it reads for the
// ladder — the verdict, or the leg on and quiet counted smooth from the
// newest fill — the marker names its reasons, a recent bar served or not.
func TestApplyNamesTheLastClosedBarsReasonsFirst(t *testing.T) {
	stood, from := testutil.At("16:00:00"), testutil.At("17:00:00")
	for name, reading := range map[string]aggragates.AIIndicators{
		"the verdict":                        withRecent(slowDeclineBlock(true), stood, from),
		"the leg on and quiet from the fill": withRecent(quietLegBlock(false, testutil.At("17:00:00"), fillBarClosed), stood, from),
	} {
		got := Apply(watchedTrade(), "", underTheBand, reading)
		if got.SlowDecline == nil || got.SlowDecline.Message != SlowDeclineMessage("buy", slowDeclineReasons) {
			t.Fatalf("%s: the marker must name the last closed bar's reasons, got %+v", name, got)
		}
	}
}

// A pending ladder's new fill is confirmed on the decline read recently: the
// last closed bar broken, a bar among the last closed ones on which the
// verdict stood, and the new fill within those bars. The marker row names the
// recent bar's reasons and carries the new fill's price, the ladder is
// pending from that fill, and at the band it sells on the same tick. The leg
// on and quiet confirms with its own reasons. A new fill before the oldest
// bar, no recent bar, or a new fill without a stamp cancels as before.
func TestApplyConfirmsTheNewFillOnTheDeclineReadRecently(t *testing.T) {
	trade := pendingAtFive()
	stood, from := testutil.At("21:00:00"), testutil.At("00:00:00")
	recent := withRecent(brokenReading(), stood, from)

	got := Apply(trade, "", underJudgeBand, recent)
	assertRow(t, got.SlowDecline, SlowDeclineMessage("buy", slowDeclineRecentReasons), sixthFill)
	assertNoSale(t, got, "")
	confirmed := withRow(trade, *got.SlowDecline, judgeTick)
	if st := rebuildState(confirmed); !st.slowDeclinePending || st.slowDeclinePendingFrom != sixthFill || slowDeclineFillUnjudged(confirmed, st) {
		t.Fatalf("a fill confirmed on the recent bar leaves the ladder pending from it, got %+v", st)
	}
	selling := Apply(trade, "stopLoss", judgeBand, recent)
	assertRow(t, selling.SlowDecline, SlowDeclineMessage("buy", slowDeclineRecentReasons), sixthFill)
	assertForced(t, selling, reasonSellBand)

	quiet := Apply(trade, "", underJudgeBand, withRecent(legOnAndQuiet(), stood, from))
	assertRow(t, quiet.SlowDecline, SlowDeclineMessage("buy", slowDeclineReasons), sixthFill)

	unstamped := pendingAtFive()
	unstamped.History[len(unstamped.History)-1].CreatedAt = time.Time{}
	cancel := SlowDeclineCancelMessage("buy", slowDeclineBreakReasons)
	for name, tc := range map[string]struct {
		trade   aggragates.Trades
		reading aggragates.AIIndicators
	}{
		"a fill before the oldest bar": {trade, withRecent(brokenReading(), stood, testutil.At("23:09:00").Add(time.Millisecond))},
		"no recent bar":                {trade, withRecent(brokenReading(), time.Time{}, from)},
		"a fill without a stamp":       {unstamped, recent},
	} {
		got := Apply(tc.trade, "", judgeBand, tc.reading)
		if got.SlowDecline == nil || got.SlowDecline.Message != cancel || got.SlowDecline.Price != sixthFill || got.Reason != "" {
			t.Fatalf("%s: the new fill cancels the exit and nothing sells, got %+v", name, got)
		}
	}
}

// The decline read recently marks a watched ladder pending only when its
// first fill, in slice order, is stamped strictly before SlowDeclineRecentAt:
// opened a millisecond before that bar it goes pending and sells at the band
// on the same tick; opened at the bar's open, inside its first millisecond,
// after it, or unstamped, nothing — the last closed bar's verdict still marks
// it.
func TestApplyGoesPendingRecentlyOnlyOnALadderThatStoodBeforeTheBar(t *testing.T) {
	stood, from := testutil.At("16:00:00"), testutil.At("09:00:00")
	for name, tc := range map[string]struct {
		first   time.Time
		pending bool
	}{
		"a millisecond before the bar":  {stood.Add(-time.Millisecond), true},
		"at the bar's open":             {stood, false},
		"inside the open's millisecond": {stood.Add(time.Millisecond - time.Nanosecond), false},
		"after the bar":                 {testutil.At("16:30:00"), false},
		"without a stamp":               {time.Time{}, false},
	} {
		trade := testutil.LadderTrade(false, testutil.W3sFills("16:40:00", "16:50:00", "17:00:00", "17:38:00")...)
		trade.History[0].CreatedAt = tc.first
		got := Apply(trade, "", slowDeclineBand, withRecent(brokenAtTheBand(), stood, from))
		verdict := Apply(trade, "", underTheBand, withRecent(slowDeclineBlock(true), stood, from))
		if verdict.SlowDecline == nil || verdict.SlowDecline.Message != SlowDeclineMessage("buy", slowDeclineReasons) || verdict.SlowDecline.Price != slowDeclineLastFill {
			t.Fatalf("first fill %s: the last closed bar's verdict marks the ladder whatever its first fill, got %+v", name, verdict)
		}
		want := Result{}
		if tc.pending {
			row := PendingRow("buy", slowDeclineLastFill, slowDeclineRecentReasons)
			want = Result{Position: "sellLoss", Reason: reasonSellBand, SlowDecline: &row}
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("first fill %s: got %+v, want %+v", name, got, want)
		}
	}
}

// The first fill gates going pending only: a pending ladder's new fill is
// confirmed on the decline read recently whatever its first fill.
func TestApplyConfirmsTheNewFillRecentlyWhateverTheFirstFill(t *testing.T) {
	stood, from := testutil.At("21:00:00"), testutil.At("00:00:00")
	for name, first := range map[string]time.Time{"after the bar": testutil.At("22:00:00"), "without a stamp": {}} {
		trade := pendingAtFive()
		trade.History[0].CreatedAt = first
		got := Apply(trade, "", underJudgeBand, withRecent(brokenReading(), stood, from))
		if got.SlowDecline == nil || got.SlowDecline.Message != SlowDeclineMessage("buy", slowDeclineRecentReasons) || got.SlowDecline.Price != sixthFill || got.Reason != "" {
			t.Fatalf("first fill %s: the new fill is confirmed on the recent bar, got %+v", name, got)
		}
	}
}
