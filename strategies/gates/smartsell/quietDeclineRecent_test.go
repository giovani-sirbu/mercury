package smartsell

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/quietdecline"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// recentPending is the row a quiet decline going pending on the decline read
// recently writes at fill: it names the recent bar's reasons.
func recentPending(fill float64) Row {
	return PendingRow(fill, quietFinding(quietDeclineRecentReasons))
}

// A watched ladder goes pending on the decline read recently: a bar among the
// last closed ones on which the verdict stood, and the ladder's newest fill
// stamped at or after the oldest of them. The row carries the newest fill's
// price and names the recent bar's reasons, and the ladder is pending from
// that fill; at a print that books a gain with a buy order resting the row is all the tick writes, and
// at a loss the shared action acts on the trigger on the same tick, the
// trailingBuy of a ladder in buy activated in place of an arming and the ladder
// sold in trailingBuy.
func TestApplyGoesPendingOnTheDeclineReadRecently(t *testing.T) {
	withQuietDecline(t, true)
	newest, stood := testutil.At("17:38:00"), testutil.At("16:00:00")
	for name, from := range map[string]time.Time{
		"an oldest bar before the fill's": testutil.At("09:00:00"),
		"the fill's own bar":              testutil.At("17:00:00"),
		"the fill's very stamp":           newest,
	} {
		got := applyAtAGain(watchedTrade(), "", withRecent(brokenReading(), stood, from))
		if got.Position != "" || got.Reason != "" {
			t.Fatalf("%s: the ladder must go pending at its newest fill without forcing, got %+v", name, got)
		}
		assertRow(t, got, recentPending(slowDeclineLastFill))
		pending := withRow(watchedTrade(), got.Rows[0], testutil.At("18:05:00"))
		if st := rebuildState(pending); !st.slowDeclinePending || st.slowDeclinePendingFrom != slowDeclineLastFill {
			t.Fatalf("%s: the row makes the ladder pending from its newest fill, got %+v", name, st)
		}
		armed := armingPrice(watchedTrade())
		inBuy := Apply(watchedTrade(), "trailingBuy", armed, withRecent(brokenReading(), stood, from))
		assertActivated(t, inBuy, recentPending(slowDeclineLastFill), quietActivation(watchedTrade(), armed))
		inStopLoss := Apply(restingIn(watchedTrade(), "trailingBuy"), "trailingBuy", armed, withRecent(brokenReading(), stood, from))
		assertSoldWith(t, inStopLoss, reasonSlowDecline, recentPending(slowDeclineLastFill))
	}
}

// The recent path fails closed: a newest fill before the oldest bar sophos
// looks back over — the ladder bought no depth within those bars — or a
// recent bar or an oldest bar not served marks nothing and sells nothing, and
// neither does a newest fill without a stamp, which the verdict no longer
// marks either: the recent-fill rule fails closed on it. A ladder short of
// quietdecline.ArmDepth is not watched, and a ladder already
// pending writes no second row of the gate's: at a print that books a gain with a buy order resting it
// writes none at all, and at a loss the one it writes is the shared action's
// activation of its trailingBuy.
func TestApplyReadsTheDeclineRecentlyOnlyFromAFillWithinTheLookBack(t *testing.T) {
	withQuietDecline(t, true)
	newest := testutil.At("17:38:00")
	stood, from := testutil.At("16:00:00"), testutil.At("17:00:00")
	for name, reading := range map[string]aggragates.AIIndicators{
		"a fill before the oldest bar": withRecent(brokenReading(), stood, newest.Add(time.Millisecond)),
		"no recent bar":                withRecent(brokenReading(), time.Time{}, from),
		"no oldest bar":                withRecent(brokenReading(), stood, time.Time{}),
	} {
		for _, position := range []string{"", "trailingBuy"} {
			if got := Apply(watchedTrade(), position, printFor(watchedTrade(), position), reading); len(got.Rows) != 0 || got.Reason != "" {
				t.Fatalf("%s on %q: nothing is marked or sold, got %+v", name, position, got)
			}
		}
	}

	unstamped := watchedTrade()
	for index := range unstamped.History {
		unstamped.History[index].CreatedAt = time.Time{}
	}
	assertNoSlowDeclineRow(t, Apply(unstamped, "", underTheBand, withRecent(brokenReading(), stood, from)), "")
	assertNoSlowDeclineRow(t, Apply(unstamped, "", underTheBand, withRecent(slowDeclineBlock(true), stood, from)), "")

	armed := armingPrice(watchedTrade())
	shallower := testutil.LadderTrade(false, fills(quietdecline.ArmDepth-1, "17:38:00")...)
	assertNoSlowDeclineRow(t, Apply(shallower, "", underTheBand, withRecent(brokenReading(), stood, from)), "")
	assertNoSlowDeclineRow(t, Apply(shallower, "trailingBuy", armed, withRecent(brokenReading(), stood, from)), "trailingBuy")
	assertNoSlowDeclineRow(t, applyAtAGain(pendingTrade(), "", withRecent(brokenReading(), stood, from)), "")
	assertActivated(t, Apply(pendingTrade(), "", underTheBand, withRecent(brokenReading(), stood, from)), quietActivation(pendingTrade(), underTheBand))
}

// A fill in the bar still forming counts on the recent path: nothing bounds
// the stamp from above. A fresh fill the last closed bar reads nothing for
// (TestApplyAFreshFillOnAQuietLegNeitherGoesPendingNorSells) goes pending on
// the recent bar, naming it, once the ladder is watched.
func TestApplyGoesPendingOnAFreshFillReadRecently(t *testing.T) {
	withQuietDecline(t, true)
	lastClosed := testutil.At("16:00:00")
	reading := withRecent(quietLegBlock(false, lastClosed, testutil.At("17:00:00")), lastClosed, testutil.At("00:00:00"))
	for _, depth := range []int{quietdecline.ArmDepth - 1, quietdecline.ArmDepth, watchedFills} {
		t.Run(fmt.Sprintf("%d fills", depth), func(t *testing.T) {
			trade := testutil.LadderTrade(false, fills(depth, "17:38:00")...)
			if depth < quietdecline.ArmDepth {
				assertNoSlowDeclineRow(t, Apply(trade, "", underTheBand, reading), "")
				return
			}
			if depth < 2 {
				// A single fill is the ladder's first fill too, stamped after the
				// recent bar, and the recent path never marks a ladder opened after
				// the bar (TestApplyGoesPendingRecentlyOnlyOnALadderThatStoodBeforeTheBar).
				t.Skipf("quietdecline.ArmDepth is %d: the watched ladder's only fill is its first, not before the recent bar", quietdecline.ArmDepth)
			}
			got := applyAtAGain(trade, "", reading)
			assertRow(t, got, recentPending(trade.PositionPrice))
			assertNoSale(t, got, "")
		})
	}
}

// The last closed bar's own reading comes first: when it reads for the
// ladder — the verdict, or the leg on and quiet counted smooth from the
// newest fill — the row names its reasons, a recent bar served or not.
func TestApplyNamesTheLastClosedBarsReasonsFirst(t *testing.T) {
	withQuietDecline(t, true)
	stood, from := testutil.At("16:00:00"), testutil.At("17:00:00")
	for name, reading := range map[string]aggragates.AIIndicators{
		"the verdict":                        withRecent(slowDeclineBlock(true), stood, from),
		"the leg on and quiet from the fill": withRecent(quietLegBlock(false, testutil.At("17:00:00"), fillBarClosed), stood, from),
	} {
		got := applyAtAGain(watchedTrade(), "", reading)
		if len(got.Rows) != 1 || !reflect.DeepEqual(got.Rows[0], quietPending(slowDeclineLastFill)) {
			t.Fatalf("%s: the row must name the last closed bar's reasons, got %+v", name, got)
		}
	}
}

// A pending ladder's new fill is confirmed on the decline read recently: the
// last closed bar broken, a bar among the last closed ones on which the
// verdict stood, and the new fill within those bars. The row names the
// recent bar's reasons and carries the new fill's price, the ladder is
// pending from that fill, and at a loss the shared action acts on the trigger the
// confirmation keeps standing on the same tick: the trailingBuy of a ladder in buy is
// activated in place of an arming and a ladder in trailingBuy is sold, the row beside
// it; at a print that books a gain with a buy order resting the row is all the tick writes. The leg on and
// quiet confirms with its own reasons. A new fill before the oldest bar, no recent
// bar, or a new fill without a stamp cancels as before, and a cancelled ladder is
// neither activated nor sold.
func TestApplyConfirmsTheNewFillOnTheDeclineReadRecently(t *testing.T) {
	withQuietDecline(t, true)
	trade := pendingAtFive()
	stood, from := testutil.At("21:00:00"), testutil.At("00:00:00")
	recent := withRecent(brokenReading(), stood, from)
	confirmedRow := ConfirmedRow(sixthFill, quietFinding(quietDeclineRecentReasons))

	got := applyAtAGain(trade, "", recent)
	assertRow(t, got, confirmedRow)
	assertNoSale(t, got, "")
	confirmed := withRow(trade, got.Rows[0], judgeTick)
	if st := rebuildState(confirmed); !st.slowDeclinePending || st.slowDeclinePendingFrom != sixthFill || slowDeclineFillUnjudged(confirmed, st) {
		t.Fatalf("a fill confirmed on the recent bar leaves the ladder pending from it, got %+v", st)
	}
	armed := armingPrice(trade)
	assertActivated(t, Apply(trade, "trailingBuy", armed, recent), confirmedRow, quietActivation(trade, armed))
	assertSoldWith(t, Apply(restingIn(trade, "trailingBuy"), "trailingBuy", armed, recent), reasonSlowDecline, confirmedRow)

	quiet := applyAtAGain(trade, "", withRecent(legOnAndQuiet(), stood, from))
	assertRow(t, quiet, ConfirmedRow(sixthFill, quietFinding(slowDeclineReasons)))

	unstamped := pendingAtFive()
	unstamped.History[len(unstamped.History)-1].CreatedAt = time.Time{}
	cancelled := CancelledRow(sixthFill, quietFinding(quietDeclineBreakReasons))
	for name, tc := range map[string]struct {
		trade   aggragates.Trades
		reading aggragates.AIIndicators
	}{
		"a fill before the oldest bar": {trade, withRecent(brokenReading(), stood, testutil.At("23:09:00").Add(time.Millisecond))},
		"no recent bar":                {trade, withRecent(brokenReading(), time.Time{}, from)},
		"a fill without a stamp":       {unstamped, recent},
	} {
		for _, state := range []string{"buy", "trailingBuy"} {
			got := Apply(restingIn(tc.trade, state), "trailingBuy", armingPrice(tc.trade), tc.reading)
			if len(got.Rows) != 1 || !reflect.DeepEqual(got.Rows[0], cancelled) || got.Reason != "" || got.Position != "trailingBuy" || got.Activated {
				t.Fatalf("%s, resting in %s: the new fill cancels the exit and nothing is activated or sold, got %+v", name, state, got)
			}
		}
	}
}

// The decline read recently marks a watched ladder pending only when its
// first fill, in slice order, is stamped strictly before QuietDeclineRecentAt:
// opened a millisecond before that bar it goes pending and is acted on on the
// same tick, its trailingBuy activated in place of an arming when it rests in buy and
// the ladder sold when it rests in trailingBuy; opened at the bar's open, inside its
// first millisecond, after it, or unstamped, nothing — the last closed bar's
// verdict still marks it.
func TestApplyGoesPendingRecentlyOnlyOnALadderThatStoodBeforeTheBar(t *testing.T) {
	withQuietDecline(t, true)
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
		armed := armingPrice(trade)
		reading := withRecent(brokenReading(), stood, from)
		verdict := applyAtAGain(trade, "", withRecent(slowDeclineBlock(true), stood, from))
		if len(verdict.Rows) != 1 || !reflect.DeepEqual(verdict.Rows[0], quietPending(slowDeclineLastFill)) {
			t.Fatalf("first fill %s: the last closed bar's verdict marks the ladder whatever its first fill, got %+v", name, verdict)
		}
		wantBuy, wantStopLoss := Result{Position: "trailingBuy"}, Result{Position: "trailingBuy"}
		if tc.pending {
			wantBuy = Result{Position: "trailingBuy", Activated: true, Rows: []Row{recentPending(slowDeclineLastFill), quietActivation(trade, armed)}}
			wantStopLoss = Result{Position: "takeLoss", Reason: reasonSlowDecline, Rows: []Row{recentPending(slowDeclineLastFill)}}
		}
		if got := Apply(trade, "trailingBuy", armed, reading); !reflect.DeepEqual(got, wantBuy) {
			t.Fatalf("first fill %s, resting in buy: got %+v, want %+v", name, got, wantBuy)
		}
		if got := Apply(restingIn(trade, "trailingBuy"), "trailingBuy", armed, reading); !reflect.DeepEqual(got, wantStopLoss) {
			t.Fatalf("first fill %s, resting in trailingBuy: got %+v, want %+v", name, got, wantStopLoss)
		}
	}
}

// The first fill gates going pending only: a pending ladder's new fill is
// confirmed on the decline read recently whatever its first fill.
func TestApplyConfirmsTheNewFillRecentlyWhateverTheFirstFill(t *testing.T) {
	withQuietDecline(t, true)
	stood, from := testutil.At("21:00:00"), testutil.At("00:00:00")
	for name, first := range map[string]time.Time{"after the bar": testutil.At("22:00:00"), "without a stamp": {}} {
		trade := pendingAtFive()
		trade.History[0].CreatedAt = first
		got := applyAtAGain(trade, "", withRecent(brokenReading(), stood, from))
		if len(got.Rows) != 1 || !reflect.DeepEqual(got.Rows[0], ConfirmedRow(sixthFill, quietFinding(quietDeclineRecentReasons))) || got.Reason != "" {
			t.Fatalf("first fill %s: the new fill is confirmed on the recent bar, got %+v", name, got)
		}
	}
}
