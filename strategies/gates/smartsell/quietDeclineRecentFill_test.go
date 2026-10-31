package smartsell

import (
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// A ladder's shape on the w3s ladder: marked pending at its sixth fill on
// the verdict, its seventh fill landing seconds into the bar after a flush
// bar, which broke the leg on and quiet. The bars before the flush held the
// verdict: sophos serves the newest of them (recentBar) among its last closed
// bars, the oldest of which opens a day before the bar the fill landed in
// (lookBackFrom). The seventh fill is judged on the first tick after it
// (seventhJudged).
var (
	seventhFillAt = testutil.At("22:00:05")
	seventhJudged = testutil.At("22:00:30")
	recentBar     = testutil.At("20:00:00")
	lookBackFrom  = testutil.At("22:00:00").Add(-24 * time.Hour)
)

// w3sPrice is the w3s ladder's fill at a depth, counted from one.
func w3sPrice(depth int) float64 {
	return fills(lastDepthFills, "21:30:00")[depth-1].Price
}

// pendingAtSixWithTheSeventh is the w3s ladder marked pending at its sixth
// fill by the engine's own tick on the verdict, then filled a seventh time at
// seventhFillAt. The tick that marks it runs at a print that books a gain with a buy order resting, where
// the gate's row is all a ladder in buy gets: the shared action activates the
// trailingBuy of a ladder in buy on a real loss, and a ladder it had activated would
// rest in trailingBuy and be sold, never filling a seventh depth.
func pendingAtSixWithTheSeventh(t *testing.T) aggragates.Trades {
	t.Helper()
	six := testutil.LadderTrade(false, fills(6, "12:58:00")...)
	trade, got := tickAtAGain(six, "", testutil.At("18:05:00"), verdictReading())
	assertRow(t, got, quietPending(w3sPrice(6)))
	trade = withFill(trade, w3sPrice(7), seventhFillAt)
	if st := rebuildState(trade); !st.slowDeclinePending || st.slowDeclinePendingFrom != w3sPrice(6) || !slowDeclineFillUnjudged(trade, st) {
		t.Fatalf("fixture drifted: pending from the sixth fill, the seventh unjudged, got %+v", st)
	}
	if lastDepthFilled(trade, 7) || !(w3sPrice(7) < underJudgeBand) {
		t.Fatal("fixture drifted: the seventh fill sits short of the last depth, under both prices the ticks print")
	}
	return trade
}

// firstFillAt stamps the w3s ladder's first fill, and barBeforeTheLadder is a
// look-back bar that opened before it: a bar the ladder did not stand on.
var (
	firstFillAt        = testutil.At("08:00:00")
	barBeforeTheLadder = testutil.At("06:00:00")
)

// flushReadRecently is the reading served on the seventh fill's first tick:
// the flush bar broke the leg on and quiet — what broke named beside the fill
// window — and the verdict stood on the bar opening at stood, the look-back
// reaching back to lookBackFrom.
func flushReadRecently(stood time.Time) aggragates.AIIndicators {
	return withRecent(brokenReading(), stood, lookBackFrom)
}

// The seventh fill, on the flush bar, is CONFIRMED by the decline read
// recently: the row names the recent bar's reasons and carries the seventh
// fill's price, and the ladder is pending from that fill. The judgement does not
// ask whether the ladder stood before the recent bar, so a bar that opened before
// its first fill confirms it all the same. At a print that books no loss the row
// is all the confirming tick writes; at a loss the shared action acts on the
// trigger the confirmation keeps standing on the same tick, the trailingBuy of a
// ladder in buy activated in place of an arming, the row beside the activation,
// and a ladder in trailingBuy sold. From then on no reading judges the fill again and
// the action acts on every tick whatever the last closed bar reads, with no second
// row of the gate's: the activation in buy, the sale in trailingBuy; the eighth fill,
// its last depth, is never judged and the action acts on that ladder too, the
// depth bound being off here.
func TestApplyConfirmsTheFillOnAFlushBarReadRecently(t *testing.T) {
	withQuietDecline(t, true)
	trade := pendingAtSixWithTheSeventh(t)
	if !barBeforeTheLadder.Before(firstFillAt) || !firstFillAt.Equal(trade.History[0].CreatedAt) {
		t.Fatal("fixture drifted: one recent bar opens before the ladder's first fill")
	}
	confirmedRow := ConfirmedRow(w3sPrice(7), quietFinding(quietDeclineRecentReasons))
	armed := armingPrice(trade)
	for _, stood := range []time.Time{recentBar, barBeforeTheLadder} {
		confirmed, got := tickAtAGain(trade, "", seventhJudged, flushReadRecently(stood))
		assertRow(t, got, confirmedRow)
		assertNoSale(t, got, "")
		if st := rebuildState(confirmed); !st.slowDeclinePending || st.slowDeclinePendingFrom != w3sPrice(7) || slowDeclineFillUnjudged(confirmed, st) {
			t.Fatalf("recent bar %s: a fill confirmed on it leaves the ladder pending from it, judged, got %+v", stood, st)
		}
		assertActivated(t, Apply(trade, "trailingBuy", armed, flushReadRecently(stood)), confirmedRow, quietActivation(trade, armed))
		assertSoldWith(t, Apply(restingIn(trade, "trailingBuy"), "trailingBuy", armed, flushReadRecently(stood)), reasonSlowDecline, confirmedRow)
	}

	confirmed, _ := tickAtAGain(trade, "", seventhJudged, flushReadRecently(recentBar))
	lastDepth := withFill(confirmed, w3sPrice(8), testutil.At("01:52:00").Add(24*time.Hour))
	for _, pending := range []aggragates.Trades{confirmed, lastDepth} {
		for _, reading := range []aggragates.AIIndicators{flushReadRecently(recentBar), brokenReading(), windowReadAlone()} {
			assertNoSlowDeclineRow(t, applyAtAGain(pending, "", reading), "")
			for _, price := range []float64{underJudgeBand, judgeBand, armingPrice(pending)} {
				assertActivated(t, Apply(pending, "", price, reading), quietActivation(pending, price))
				sold := Apply(restingIn(pending, "trailingBuy"), "trailingBuy", price, reading)
				assertSold(t, sold, reasonSlowDecline)
			}
		}
	}
}

// The same fill on the same flush bar with no recent bar served — the
// look-back's oldest bar and reasons served, its newest held bar zero — is
// CANCELLED as before: the row names what broke and carries the seventh
// fill's price, nothing is activated or sold on that tick, an arming included,
// or after it, in buy or in trailingBuy, and the ladder is watched and not pending.
func TestApplyCancelsTheFillOnAFlushBarWithoutARecentBar(t *testing.T) {
	withQuietDecline(t, true)
	trade := pendingAtSixWithTheSeventh(t)
	noRecentBar := withRecent(brokenReading(), time.Time{}, lookBackFrom)
	for _, state := range []string{"buy", "trailingBuy"} {
		_, got := engineTick(restingIn(trade, state), "trailingBuy", armingPrice(trade), seventhJudged, noRecentBar)
		assertRow(t, got, CancelledRow(w3sPrice(7), quietFinding(quietDeclineBreakReasons)))
		assertNoSale(t, got, "trailingBuy")
	}
	cancelled, _ := engineTick(trade, "trailingBuy", armingPrice(trade), seventhJudged, noRecentBar)
	if st := rebuildState(cancelled); st.slowDeclinePending || st.slowDeclinePendingFrom != 0 || !st.slowDeclineWatched {
		t.Fatalf("a cancelled ladder is watched and not pending, got %+v", st)
	}
	for _, state := range []string{"buy", "trailingBuy"} {
		for _, position := range append(slices.Clone(ordinaryProposals), armingProposals...) {
			for _, price := range []float64{judgeBand, overJudgeBand, armingPrice(cancelled)} {
				assertNoSlowDeclineRow(t, Apply(restingIn(cancelled, state), position, price, noRecentBar), position)
			}
		}
	}
}

// A watched ladder that is not pending goes pending on the decline read
// recently only while its newest fill is within the look-back and the ladder
// stood before the recent bar: its first fill stamped strictly before the
// bar's open. The cancelled ladder's first fill is stamped at firstFillAt and
// its seventh in the bar opening at 22:00. A recent bar the next evening marks
// it pending — the row names the recent bar's reasons at the seventh fill — on
// a look-back whose oldest bar opens at 22:00, and nothing on one whose oldest
// bar opens an hour later. On a look-back over the whole day, a recent bar
// opening at the first fill's very stamp marks nothing, and one opening an
// hour later marks it pending. Nothing marked, nothing is activated or sold, an
// arming included, in buy or in trailingBuy; marked, the ladder writes its row alone
// at a print that books a gain with a buy order resting, and at a loss the shared action acts on it on the
// same tick, the trailingBuy of a ladder in buy activated in place of whatever it
// proposes and a ladder in trailingBuy sold.
func TestApplyGoesPendingAgainOnlyOnALookBackOverTheNewestFill(t *testing.T) {
	withQuietDecline(t, true)
	cancelled, _ := engineTick(pendingAtSixWithTheSeventh(t), "", judgeBand, seventhJudged, brokenReading())
	if st := rebuildState(cancelled); st.slowDeclinePending || !st.slowDeclineWatched {
		t.Fatalf("fixture drifted: the seventh fill cancelled the exit, got %+v", st)
	}
	later, fillsBar, day := testutil.At("21:00:00").Add(24*time.Hour), testutil.At("22:00:00"), testutil.At("00:00:00")
	loss, gain := armingPrice(cancelled), gainPrint(cancelled)
	for name, tc := range map[string]struct {
		stood, oldest time.Time
		pending       bool
	}{
		"a later bar, the look-back from the seventh fill's bar": {later, fillsBar, true},
		"a later bar, the look-back from the bar after it":       {later, fillsBar.Add(time.Hour), false},
		"a bar opening at the first fill's stamp":                {firstFillAt, day, false},
		"a bar opening an hour after the first fill":             {firstFillAt.Add(time.Hour), day, true},
	} {
		reading := withRecent(brokenReading(), tc.stood, tc.oldest)
		for _, position := range []string{"", "trailingBuy"} {
			if !tc.pending {
				for _, state := range []string{"buy", "trailingBuy"} {
					for _, price := range []float64{gain, loss} {
						if got := Apply(restingIn(cancelled, state), position, price, reading); len(got.Rows) != 0 || got.Reason != "" || got.Activated || got.Position != position {
							t.Fatalf("%s on %q in %s at %v: nothing is marked, activated or sold, got %+v", name, position, state, price, got)
						}
					}
				}
				continue
			}
			marked := recentPending(w3sPrice(7))
			got := Apply(withRestingOrder(cancelled), position, gain, reading)
			assertRow(t, got, marked)
			assertNoSale(t, got, position)
			assertActivated(t, Apply(cancelled, position, loss, reading), marked, quietActivation(cancelled, loss))
			assertSoldWith(t, Apply(restingIn(cancelled, "trailingBuy"), position, loss, reading), reasonSlowDecline, marked)
		}
	}
}

// A watched ladder goes pending only on a fill within the fill window sophos
// serves: its newest fill a millisecond before QuietDeclineFillFrom, or no
// window served, marks nothing — on the verdict, on the leg on and quiet
// counted from that fill, or on the decline read recently — and activates and
// sells nothing, while at QuietDeclineFillFrom each marks it and, at a loss, the
// shared action acts on it on the same tick: the trailingBuy of a ladder in buy is
// activated in place of an arming, the row of the reading that marked it beside the
// activation, and a ladder in trailingBuy is sold. A pending ladder's new fill is
// judged whatever the window opens at, a window opening after it included, and
// waits only where no window is read at all, in buy or in trailingBuy: the quiet
// decline's reading is taken on a window read.
func TestApplyGoesPendingOnlyOnAFillWithinTheFillWindow(t *testing.T) {
	withQuietDecline(t, true)
	newest := testutil.At("17:38:00")
	trade := watchedTrade()
	armed := armingPrice(trade)
	for name, tc := range map[string]struct {
		reading aggragates.AIIndicators
		marks   Row
	}{
		"the verdict":               {slowDeclineBlock(true), quietPending(slowDeclineLastFill)},
		"the leg on and quiet":      {quietLegBlock(false, testutil.At("17:00:00"), fillBarClosed), quietPending(slowDeclineLastFill)},
		"the decline read recently": {withRecent(brokenReading(), testutil.At("16:00:00"), testutil.At("09:00:00")), recentPending(slowDeclineLastFill)},
	} {
		for _, window := range []struct {
			from    int64
			pending bool
		}{{newest.Add(time.Millisecond).UnixMilli(), false}, {0, false}, {newest.UnixMilli(), true}} {
			reading := tc.reading
			reading.SmartSell.QuietDeclineFillFrom = window.from
			if !window.pending {
				for _, state := range []string{"buy", "trailingBuy"} {
					if got := Apply(restingIn(trade, state), "trailingBuy", armed, reading); len(got.Rows) != 0 || got.Reason != "" || got.Activated || got.Position != "trailingBuy" {
						t.Fatalf("%s, window from %d, in %s: nothing is marked, activated or sold, got %+v", name, window.from, state, got)
					}
				}
				continue
			}
			if got := Apply(trade, "trailingBuy", armed, reading); !got.Activated || !reflect.DeepEqual(got.Rows, []Row{tc.marks, quietActivation(trade, armed)}) {
				t.Fatalf("%s, window from the newest fill: marked and activated, got %+v", name, got)
			}
			if got := Apply(restingIn(trade, "trailingBuy"), "trailingBuy", armed, reading); got.Reason != reasonSlowDecline || !reflect.DeepEqual(got.Rows, []Row{tc.marks}) {
				t.Fatalf("%s, window from the newest fill: marked and sold, got %+v", name, got)
			}
		}
	}
	noWindow := legOnAndQuiet()
	noWindow.SmartSell.QuietDeclineFillFrom = 0
	for _, state := range []string{"buy", "trailingBuy"} {
		assertNoSlowDeclineRow(t, Apply(restingIn(pendingAtFive(), state), "", underJudgeBand, noWindow), "")
	}
	afterTheSixth := legOnAndQuiet()
	afterTheSixth.SmartSell.QuietDeclineFillFrom = testutil.At("23:09:00").Add(time.Minute).UnixMilli()
	assertRow(t, applyAtAGain(pendingAtFive(), "", afterTheSixth), ConfirmedRow(sixthFill, quietFinding(slowDeclineReasons)))
}
