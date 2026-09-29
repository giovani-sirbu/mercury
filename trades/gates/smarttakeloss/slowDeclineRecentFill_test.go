package smarttakeloss

import (
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// Trade 88395's shape on the w3s ladder: marked pending at its sixth fill on
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
// seventhFillAt.
func pendingAtSixWithTheSeventh(t *testing.T) aggragates.Trades {
	t.Helper()
	trade, got := engineTick(testutil.LadderTrade(false, fills(6, "12:58:00")...), "", underJudgeBand, testutil.At("18:05:00"), verdictReading())
	assertRow(t, got.SlowDecline, SlowDeclineMessage("buy", slowDeclineReasons), w3sPrice(6))
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
// the flush bar broke the leg on and quiet — what broke named beside the band
// — and the verdict stood on the bar opening at stood, the look-back reaching
// back to lookBackFrom.
func flushReadRecently(stood time.Time) aggragates.AIIndicators {
	return withRecent(brokenReading(), stood, lookBackFrom)
}

// The seventh fill, on the flush bar, is CONFIRMED by the decline read
// recently: the marker row names the recent bar's reasons and carries the
// seventh fill's price, nothing sells under the band, and the ladder is
// pending from that fill. The judgement does not ask whether the ladder stood
// before the recent bar, so a bar that opened before its first fill confirms
// it all the same. From then on no reading judges it again and the band sells
// it whatever the last closed bar reads; the eighth fill, its last depth, is
// never judged and the band sells that ladder too.
func TestApplyConfirmsTheFillOnAFlushBarReadRecently(t *testing.T) {
	trade := pendingAtSixWithTheSeventh(t)
	if !barBeforeTheLadder.Before(firstFillAt) || !firstFillAt.Equal(trade.History[0].CreatedAt) {
		t.Fatal("fixture drifted: one recent bar opens before the ladder's first fill")
	}
	for _, stood := range []time.Time{recentBar, barBeforeTheLadder} {
		confirmed, got := engineTick(trade, "", underJudgeBand, seventhJudged, flushReadRecently(stood))
		assertRow(t, got.SlowDecline, SlowDeclineMessage("buy", slowDeclineRecentReasons), w3sPrice(7))
		assertNoSale(t, got, "")
		if st := rebuildState(confirmed); !st.slowDeclinePending || st.slowDeclinePendingFrom != w3sPrice(7) || slowDeclineFillUnjudged(confirmed, st) {
			t.Fatalf("recent bar %s: a fill confirmed on it leaves the ladder pending from it, judged, got %+v", stood, st)
		}
		selling := Apply(trade, "stopLoss", judgeBand, flushReadRecently(stood))
		assertRow(t, selling.SlowDecline, SlowDeclineMessage("buy", slowDeclineRecentReasons), w3sPrice(7))
		assertForced(t, selling, reasonSellBand)
	}

	confirmed, _ := engineTick(trade, "", underJudgeBand, seventhJudged, flushReadRecently(recentBar))
	lastDepth := withFill(confirmed, w3sPrice(8), testutil.At("01:52:00").Add(24*time.Hour))
	for _, pending := range []aggragates.Trades{confirmed, lastDepth} {
		for _, reading := range []aggragates.AIIndicators{flushReadRecently(recentBar), brokenReading(), judgeBandAlone()} {
			assertNoSlowDeclineRow(t, Apply(pending, "", underJudgeBand, reading), "")
			sold := Apply(pending, "", judgeBand, reading)
			assertForced(t, sold, reasonSellBand)
			if sold.SlowDecline != nil {
				t.Fatalf("%d fills: a judged ladder writes no row at the band, got %+v", len(pending.History), sold)
			}
		}
	}
}

// The same fill on the same flush bar with no recent bar served — the
// look-back's oldest bar and reasons served, its newest held bar zero — is
// CANCELLED as before: the cancel row names what broke and carries the seventh
// fill's price, nothing sells at the band on that tick or after it, and the
// ladder is watched and not pending.
func TestApplyCancelsTheFillOnAFlushBarWithoutARecentBar(t *testing.T) {
	trade := pendingAtSixWithTheSeventh(t)
	noRecentBar := withRecent(brokenReading(), time.Time{}, lookBackFrom)
	cancelled, got := engineTick(trade, "", judgeBand, seventhJudged, noRecentBar)
	assertRow(t, got.SlowDecline, SlowDeclineCancelMessage("buy", slowDeclineBreakReasons), w3sPrice(7))
	assertNoSale(t, got, "")
	if st := rebuildState(cancelled); st.slowDeclinePending || st.slowDeclinePendingFrom != 0 || !st.slowDeclineWatched {
		t.Fatalf("a cancelled ladder is watched and not pending, got %+v", st)
	}
	for _, price := range []float64{judgeBand, overJudgeBand} {
		assertNoSlowDeclineRow(t, Apply(cancelled, "", price, noRecentBar), "")
	}
}

// A watched ladder that is not pending goes pending on the decline read
// recently only while its newest fill is within the look-back and the ladder
// stood before the recent bar: its first fill stamped strictly before the
// bar's open. The cancelled ladder's first fill is stamped at firstFillAt and
// its seventh in the bar opening at 22:00. A recent bar the next evening marks
// it pending — the marker names the recent bar's reasons at the seventh fill —
// on a look-back whose oldest bar opens at 22:00, and nothing on one whose
// oldest bar opens an hour later. On a look-back over the whole day, a recent
// bar opening at the first fill's very stamp marks nothing, and one opening an
// hour later marks it pending. Nothing marked, nothing sells at the band.
func TestApplyGoesPendingAgainOnlyOnALookBackOverTheNewestFill(t *testing.T) {
	cancelled, _ := engineTick(pendingAtSixWithTheSeventh(t), "", judgeBand, seventhJudged, brokenReading())
	if st := rebuildState(cancelled); st.slowDeclinePending || !st.slowDeclineWatched {
		t.Fatalf("fixture drifted: the seventh fill cancelled the exit, got %+v", st)
	}
	later, fillsBar, day := testutil.At("21:00:00").Add(24*time.Hour), testutil.At("22:00:00"), testutil.At("00:00:00")
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
		for _, price := range []float64{underJudgeBand, judgeBand} {
			got := Apply(cancelled, "", price, reading)
			if !tc.pending {
				if got.SlowDecline != nil || got.Reason != "" {
					t.Fatalf("%s at %v: nothing is marked or sold, got %+v", name, price, got)
				}
				continue
			}
			assertRow(t, got.SlowDecline, SlowDeclineMessage("buy", slowDeclineRecentReasons), w3sPrice(7))
			if price == judgeBand {
				assertForced(t, got, reasonSellBand)
			} else {
				assertNoSale(t, got, "")
			}
		}
	}
}

// A watched ladder goes pending only on a fill within the fill window sophos
// serves: its newest fill a millisecond before SlowDeclineFillFrom, or no
// window served, marks nothing — on the verdict, on the leg on and quiet
// counted from that fill, or on the decline read recently — and sells
// nothing, while at SlowDeclineFillFrom each marks it and sells at the band
// on the same tick. A pending ladder's new fill is judged whatever the window.
func TestApplyGoesPendingOnlyOnAFillWithinTheFillWindow(t *testing.T) {
	newest := testutil.At("17:38:00")
	for name, reading := range map[string]aggragates.AIIndicators{
		"the verdict":               slowDeclineBlock(true),
		"the leg on and quiet":      quietLegBlock(false, testutil.At("17:00:00"), fillBarClosed),
		"the decline read recently": withRecent(brokenAtTheBand(), testutil.At("16:00:00"), testutil.At("09:00:00")),
	} {
		for _, tc := range []struct {
			from    int64
			pending bool
		}{{newest.Add(time.Millisecond).UnixMilli(), false}, {0, false}, {newest.UnixMilli(), true}} {
			reading.SmartTakeLoss.SlowDeclineFillFrom = tc.from
			got := Apply(watchedTrade(), "", slowDeclineBand, reading)
			if !tc.pending && (got.SlowDecline != nil || got.Reason != "") {
				t.Fatalf("%s, window from %d: nothing is marked or sold, got %+v", name, tc.from, got)
			}
			if tc.pending && (got.SlowDecline == nil || got.Reason != reasonSellBand) {
				t.Fatalf("%s, window from the newest fill: marked and sold, got %+v", name, got)
			}
		}
	}
	noWindow := legOnAndQuiet()
	noWindow.SmartTakeLoss.SlowDeclineFillFrom = 0
	assertRow(t, Apply(pendingAtFive(), "", underJudgeBand, noWindow).SlowDecline, SlowDeclineMessage("buy", slowDeclineReasons), sixthFill)
}
