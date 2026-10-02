package smarttakeloss

import (
	"slices"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/swingdecline"
)

// stairDecline is the staircase carried on down, so a ladder's later fills read
// the same decline: it holds at the sixth fill's bar (36) from the first depth.
var stairDecline = append(slices.Clone(stairTurns), 84, 83, 82, 81, 80, 79)

// stairRally is the staircase with a rally in place of its last leg, carried
// on down again in a second staircase. It breaks at the fifth fill's bar (30)
// and at the sixth's bar 36, and reads a decline again at bar 60, where a
// ladder's sixth fill reads it afresh.
var stairRally = append(slices.Clone(stairBounce), 108, 110, 104, 106, 100, 102, 96, 98, 92, 94, 88)

// A fill the series has not closed the bar of is not judged, and an unjudged
// fill sells nothing: tick after tick the band is reached and the ladder is
// left alone, until the tick whose series has closed the fill's bar, which
// confirms the exit and sells on that same tick; the ladder is judged from
// then on and the band keeps selling, with no second row. The quiet decline's
// own reading, taken and missed on every tick the band is served, never
// cancels the fill ahead of the bar.
func TestSwingDeclineAnUnjudgedFillSellsNothingUntilItsBarCloses(t *testing.T) {
	ticked, band := stairFifthFill(t), slowDeclineBand+1
	open := stairBlock(stairTurns, 29)
	for _, now := range []time.Time{stairOpen(30).Add(20 * time.Minute), stairOpen(30).Add(40 * time.Minute)} {
		var got Result
		ticked, got = engineTick(ticked, "", band, now, withBlock(open))
		assertUntouched(t, got, "")
	}
	if st := rebuildState(ticked); !st.slowDeclinePending || st.slowDeclinePendingFrom != stairPrices[3] || !slowDeclineFillUnjudged(ticked, st) || sellsAtTheBand(ticked, st, band, open) {
		t.Fatalf("the ladder waits pending from its fourth fill with the fifth unjudged, selling nothing, got %+v", rebuildState(ticked))
	}

	closed := stairFifthBlock()
	judged, got := engineTick(ticked, "", band, stairOpen(31), withBlock(closed))
	assertRow(t, got, ConfirmedRow(stairPrices[4], swingFinding(stairReading(t, ticked, closed, true).Reasons)))
	assertForced(t, got, reasonSellBand)
	if st := rebuildState(judged); st.slowDeclinePendingFrom != stairPrices[4] || !sellsAtTheBand(judged, st, band, closed) {
		t.Fatalf("the confirmation moves the ladder to its fifth fill, judged, got %+v", st)
	}
	again := Apply(judged, "", band, withBlock(closed))
	if len(again.Rows) != 0 {
		t.Errorf("a judged fill writes no second row, got %+v", again)
	}
	assertForced(t, again, reasonSellBand)
	assertNoSale(t, Apply(judged, "", underTheBand, withBlock(closed)), "")
}

// A pending ladder whose newest fill's bar is older than the first bar the
// series serves — the window cannot be read at all — is cancelled, not waited
// on: the row names why nothing was read, the latch the swing decline took stays,
// and the band sells nothing on that tick.
func TestSwingDeclineCancelsAFillOlderThanTheServedSeries(t *testing.T) {
	later := stairBlock(stairDecline, 42)
	later.SwingDeclineOpens, later.SwingDeclineCloses = later.SwingDeclineOpens[31:], later.SwingDeclineCloses[31:]
	if later.SwingDeclineOpens[0] <= stairOpen(30).UnixMilli() {
		t.Fatal("fixture drifted: the series must start after the fifth fill's bar")
	}
	ticked, got := engineTick(stairFifthFill(t), "", slowDeclineBand+1, stairOpen(43), withBlock(later))
	unreadable := []string{"depth 5, no window between the fills is readable in the 1h bars served"}
	assertRow(t, got, CancelledRow(stairPrices[4], quietFinding(nil), swingFinding(unreadable)))
	if !slices.Equal(got.Rows[0].Reasons, unreadable) {
		t.Errorf("the cancel names why nothing was read, got %q", got.Rows[0].Reasons)
	}
	assertNoSale(t, got, "")
	if st := rebuildState(ticked); st.slowDeclinePending || !st.swingDeclineWatched || !st.indecision {
		t.Fatalf("watched, no longer pending and still latched, got %+v", st)
	}
}

// A cancelled ladder is made pending again by the trigger alone, on a later
// fill, and only inside the read bars of that fill: the sixth fill's series
// three bars past its bar writes nothing, the series that has just closed it
// writes the pending row, and the latch the swing decline took at the fourth
// fill is not written a second time.
func TestSwingDeclineACancelledLadderGoesPendingAgainOnAFreshRead(t *testing.T) {
	fifth := stairFifthFill(t)
	rally := stairBlock(stairRally, 30)
	breaks := stairReading(t, fifth, rally, false).Breaks
	cancelled, got := engineTick(fifth, "", underTheBand, stairOpen(31), withBlock(rally))
	assertRow(t, got, CancelledRow(stairPrices[4], quietFinding(nil), swingFinding(breaks)))
	if st := rebuildState(cancelled); st.slowDeclinePending || !st.indecision {
		t.Fatalf("fixture drifted: cancelled and latched, got %+v", st)
	}

	sixth := withFill(cancelled, stairPrices[5], stairOpen(60).Add(17*time.Minute))
	assertUntouched(t, Apply(sixth, "", underTheBand, withBlock(stairBlock(stairRally, 63))), "")
	fresh := stairBlock(stairRally, 60)
	pending, got := engineTick(sixth, "", underTheBand, stairOpen(61), withBlock(fresh))
	assertRow(t, got, PendingRow(stairPrices[5], swingFinding(stairReading(t, sixth, fresh, true).Reasons)))
	if st := rebuildState(pending); !st.slowDeclinePending || st.slowDeclinePendingFrom != stairPrices[5] {
		t.Errorf("pending from the sixth fill, got %+v", st)
	}
}

// The fill that takes a pending ladder to its last depth is never judged, even
// when the series it is served would break the swing decline: the exit stays
// pending from where it was, no row is written, and the band sells.
func TestSwingDeclineTheLastDepthFillIsNotJudgedWhateverTheSeriesReads(t *testing.T) {
	eighth := withFill(withRows(stairLadder(0, 9, 15, 24, 30, 36, 42), PendingRow(stairPrices[6], swingFinding(nil))), stairPrices[7], stairOpen(48).Add(time.Minute))
	breaking := stairBlock([]float64{100, 110, 120, 130, 140, 150, 160, 170, 180, 190, 200, 210, 220, 230, 240, 250, 260}, 48)
	if _, holds := swingdecline.Trigger(swingFills(rebuildState(eighth).fills), swingdecline.SeriesOf(breaking), swingdecline.TrendOf(breaking)); holds {
		t.Fatal("fixture drifted: the series must break the swing decline at the eighth fill")
	}
	assertUntouched(t, Apply(eighth, "", underTheBand, withBlock(breaking)), "")
	got := Apply(eighth, "", slowDeclineBand+1, withBlock(breaking))
	assertForced(t, got, reasonSellBand)
	if len(got.Rows) != 0 {
		t.Errorf("the last depth's fill is judged by nobody, got %+v", got)
	}
}

// ExitReached is Apply's own answer for the swing decline's sale: it holds on
// the tick a ladder goes pending at the band and on the tick a new fill's
// judgement confirms the exit, and misses on a tick that cancels it, on one
// whose new fill is not judged yet, and under the band.
func TestSwingDeclineExitReachedAgreesWithApply(t *testing.T) {
	band := slowDeclineBand + 1
	fourth, fifth := stairFourth(), stairFifthFill(t)
	for name, tc := range map[string]struct {
		reached bool
		price   float64
		got     Result
		exit    bool
	}{
		"going pending at the band":    {true, band, Apply(fourth, "", band, withBlock(stairFourthBlock())), ExitReached(fourth, band, stairFourthBlock())},
		"going pending under the band": {false, underTheBand, Apply(fourth, "", underTheBand, withBlock(stairFourthBlock())), ExitReached(fourth, underTheBand, stairFourthBlock())},
		"a confirming judgement":       {true, band, Apply(fifth, "", band, withBlock(stairFifthBlock())), ExitReached(fifth, band, stairFifthBlock())},
		"a cancelling judgement":       {false, band, Apply(fifth, "", band, withBlock(stairBlock(stairRally, 30))), ExitReached(fifth, band, stairBlock(stairRally, 30))},
		"a fill not judged yet":        {false, band, Apply(fifth, "", band, withBlock(stairBlock(stairTurns, 29))), ExitReached(fifth, band, stairBlock(stairTurns, 29))},
	} {
		if tc.exit != tc.reached || (tc.got.Reason == reasonSellBand) != tc.reached {
			t.Errorf("%s: ExitReached %v, Apply %+v, want the sale %v", name, tc.exit, tc.got, tc.reached)
		}
	}
}

// The judgement of a pending ladder's new fill, on every series the swing decline
// can be served: confirmed when its window and trend read again, cancelled when a
// rally breaks it or the series no longer holds its window, and waited on — no
// row, no sale — when the fill's bar is not closed yet. The band sells on a
// confirmation and on no other tick, and the latch the swing decline took at the
// fourth fill is never written a second time.
func TestSwingDeclineJudgesTheNewFillOfAPendingLadder(t *testing.T) {
	fifth := stairFifthFill(t)
	if st := rebuildState(fifth); !st.slowDeclinePending || !slowDeclineFillUnjudged(fifth, st) || sellsAtTheBand(fifth, st, slowDeclineBand+1, stairFifthBlock()) {
		t.Fatalf("fixture drifted: pending from the fourth fill, the fifth unjudged, got %+v", st)
	}
	late := stairBlock(append(slices.Clone(stairTurns), 84, 83, 82, 81), 42)
	// A series that starts after the third fill's bar holds no bar an earlier fill
	// sits in, so no window between the fills is readable whatever depths apart
	// the rule asks them.
	older := stairFifthBlock()
	older.SwingDeclineOpens, older.SwingDeclineCloses = older.SwingDeclineOpens[16:], older.SwingDeclineCloses[16:]
	for name, tc := range map[string]struct {
		block aggragates.SmartTakeLossIndicators
		event string
	}{
		"confirmed":                           {stairFifthBlock(), EventPending},
		"confirmed twelve bars late":          {late, EventPending},
		"broken by a rally":                   {stairBlock(stairBounce, 30), EventCancelled},
		"a window the series no longer holds": {older, EventCancelled},
		"the fill's bar not closed yet":       {stairBlock(stairTurns, 29), ""},
		"no series served":                    {aggragates.SmartTakeLossIndicators{SlowDeclineSellBand: slowDeclineBand}, EventCancelled},
	} {
		got := Apply(fifth, "", slowDeclineBand+1, withBlock(tc.block))
		if (len(got.Rows) == 0) != (tc.event == "") || (len(got.Rows) == 1 && (got.Rows[0].Event != tc.event || got.Rows[0].Gate != GateSlowDecline || got.Rows[0].Price != stairPrices[4])) || len(got.Rows) > 1 {
			t.Errorf("%s: want a %q row at the fifth fill, got %+v", name, tc.event, got.Rows)
		}
		if len(got.Rows) == 1 && slices.Contains(got.Rows[0].Features, FeatureIndecision) {
			t.Errorf("%s: the latch is taken and never written twice, got %+v", name, got.Rows[0])
		}
		if sold := got.Reason == reasonSellBand; sold != (tc.event == EventPending) {
			t.Errorf("%s: the band sells on a confirmation and on no other tick, got %+v", name, got)
		}
	}

	_, got := engineTick(fifth, "", slowDeclineBand+1, stairOpen(60), withBlock(older))
	if want := []string{"depth 5, no window between the fills is readable in the 1h bars served"}; !slices.Equal(got.Rows[0].Reasons, want) {
		t.Errorf("the cancel names why nothing was read, got %q", got.Rows[0].Reasons)
	}
	broken := stairBlock(stairBounce, 30)
	cancelled, _ := engineTick(fifth, "", slowDeclineBand+1, stairOpen(60), withBlock(broken))
	if st := rebuildState(cancelled); !st.swingDeclineWatched || st.slowDeclinePending || !st.indecision {
		t.Fatalf("a cancelled ladder is watched, not pending, and still latched, got %+v", st)
	}
	assertUntouched(t, Apply(cancelled, "", slowDeclineBand+1, withBlock(broken)), "")
}

// The fill that takes a pending swing ladder to its last depth is never judged:
// it stays pending from where it was and the band sells at once.
func TestSwingDeclineNeverJudgesTheFillThatReachesTheLastDepth(t *testing.T) {
	seventh := withRows(stairLadder(0, 9, 15, 24, 30, 36, 42), PendingRow(stairPrices[6], swingFinding(nil)))
	eighth := withFill(seventh, stairPrices[7], stairOpen(48).Add(time.Minute))
	if st := rebuildState(eighth); !st.slowDeclinePending || slowDeclineFillUnjudged(eighth, st) {
		t.Fatalf("fixture drifted: the last-depth fill must read as judged, got %+v", st)
	}
	block := withBlock(aggragates.SmartTakeLossIndicators{SlowDeclineSellBand: slowDeclineBand})
	assertUntouched(t, Apply(eighth, "", slowDeclineBand-1, block), "")
	assertForced(t, Apply(eighth, "", slowDeclineBand+1, block), reasonSellBand)
}
