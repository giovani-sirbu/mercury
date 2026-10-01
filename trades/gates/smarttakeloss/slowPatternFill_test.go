package smarttakeloss

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/gates/slowpattern"
)

// stairDecline is the staircase carried on down, so a ladder's later fills read
// the same decline: it holds at the sixth fill's bar (36) from the first depth.
var stairDecline = append(slices.Clone(stairTurns), 84, 83, 82, 81, 80, 79)

// stairRally is the staircase with a rally in place of its last leg, carried
// on down again in a second staircase. It breaks at the fifth fill's bar (30)
// and at the sixth's bar 36, and holds on the window from the second depth at
// bar 60, where a ladder's sixth fill reads it afresh.
var stairRally = append(slices.Clone(stairBounce), 108, 110, 104, 106, 100, 102, 96, 98, 92, 94, 88)

// A fill the series has not closed the bar of is not judged, and an unjudged
// fill sells nothing: tick after tick the band is reached and the ladder is
// left alone, until the tick whose series has closed the fill's bar, which
// confirms the exit and sells on that same tick; the ladder is judged from
// then on and the band keeps selling, with no second row.
func TestSlowPatternAnUnjudgedFillSellsNothingUntilItsBarCloses(t *testing.T) {
	ticked, band := stairFifthFill(t), slowDeclineBand+1
	for _, now := range []time.Time{stairOpen(30).Add(20 * time.Minute), stairOpen(30).Add(40 * time.Minute)} {
		var got Result
		ticked, got = engineTick(ticked, "", band, now, withBlock(stairBlock(stairTurns, 29)))
		assertUntouched(t, got, "")
	}
	if st := rebuildState(ticked); !st.slowPatternPending || st.slowPatternPendingFrom != stairPrices[3] || !slowPatternFillUnjudged(ticked, st) || slowPatternSells(ticked, st) {
		t.Fatalf("the ladder waits pending from its fourth fill with the fifth unjudged, selling nothing, got %+v", st)
	}

	closed := withBlock(stairBlock(stairTurns, 30))
	judged, got := engineTick(ticked, "", band, stairOpen(31), closed)
	assertRow(t, got.SlowDecline, SlowPatternMessage("buy", patternReasonsAt30), stairPrices[4])
	assertForced(t, got, reasonSlowPatternBand)
	if st := rebuildState(judged); st.slowPatternPendingFrom != stairPrices[4] || !slowPatternSells(judged, st) {
		t.Fatalf("the confirmation moves the ladder to its fifth fill, judged, got %+v", st)
	}
	again := Apply(judged, "", band, closed)
	if again.SlowDecline != nil || again.Indecision != nil {
		t.Errorf("a judged fill writes no second row, got %+v", again)
	}
	assertForced(t, again, reasonSlowPatternBand)
	assertNoSale(t, Apply(judged, "", underTheBand, closed), "")
}

// A pending ladder whose newest fill's bar is older than the first bar the
// series serves — the window cannot be read at all — is cancelled, not waited
// on: the row names why nothing was read, the latch the pattern took stays, and
// the band sells nothing on that tick.
func TestSlowPatternCancelsAFillOlderThanTheServedSeries(t *testing.T) {
	later := stairBlock(stairDecline, 42)
	later.SlowPatternOpens, later.SlowPatternCloses = later.SlowPatternOpens[31:], later.SlowPatternCloses[31:]
	if later.SlowPatternOpens[0] <= stairOpen(30).UnixMilli() {
		t.Fatal("fixture drifted: the series must start after the fifth fill's bar")
	}
	ticked, got := engineTick(stairFifthFill(t), "", slowDeclineBand+1, stairOpen(43), withBlock(later))
	if got.SlowDecline == nil || got.SlowDecline.Event != EventCancelled || got.SlowDecline.Price != stairPrices[4] {
		t.Fatalf("want a cancelled row at the fifth fill, got %+v", got)
	}
	if want := []string{"depth 5, no window between the fills is readable in the 1h bars served"}; !slices.Equal(got.SlowDecline.Reasons, want) {
		t.Errorf("the cancel names why nothing was read, got %q", got.SlowDecline.Reasons)
	}
	assertNoSale(t, got, "")
	if st := rebuildState(ticked); st.slowPatternPending || !st.slowPatternWatched || !st.indecision {
		t.Fatalf("watched, no longer pending and still latched, got %+v", st)
	}
}

// A cancelled ladder is made pending again by the trigger alone, on a later
// fill, and only inside the read bars of that fill: the sixth fill's series
// three bars past its bar writes nothing, the series that has just closed it
// writes the pending row from the second depth, and the latch the pattern took
// at the fourth fill is not written a second time.
func TestSlowPatternACancelledLadderGoesPendingAgainOnAFreshRead(t *testing.T) {
	cancelled, got := engineTick(stairFifthFill(t), "", underTheBand, stairOpen(31), withBlock(stairBlock(stairRally, 30)))
	assertRow(t, got.SlowDecline, SlowPatternCancelMessage("buy", patternBreaksAt30), stairPrices[4])
	if st := rebuildState(cancelled); st.slowPatternPending || !st.indecision {
		t.Fatalf("fixture drifted: cancelled and latched, got %+v", st)
	}

	sixth := withFill(cancelled, stairPrices[5], stairOpen(60).Add(17*time.Minute))
	assertUntouched(t, Apply(sixth, "", underTheBand, withBlock(stairBlock(stairRally, 63))), "")
	pending, got := engineTick(sixth, "", underTheBand, stairOpen(61), withBlock(stairBlock(stairRally, 60)))
	row := got.SlowDecline
	if row == nil || row.Gate != GateSlowPattern || row.Event != EventPending || row.Price != stairPrices[5] || !strings.HasPrefix(row.Reasons[0], "depth 2 to 6, 1h bars ") {
		t.Fatalf("want the pending row from the second depth at the sixth fill, got %+v", row)
	}
	if got.Indecision != nil {
		t.Errorf("the latch is taken and never written twice, got %+v", got.Indecision)
	}
	if st := rebuildState(pending); !st.slowPatternPending || st.slowPatternPendingFrom != stairPrices[5] {
		t.Errorf("pending from the sixth fill, got %+v", st)
	}
}

// The fill that takes a pending ladder to its last depth is never judged, even
// when the series it is served would break the pattern: the exit stays pending
// from where it was, no row is written, and the band sells.
func TestSlowPatternTheLastDepthFillIsNotJudgedWhateverTheSeriesReads(t *testing.T) {
	eighth := withFill(withRows(stairLadder(0, 9, 15, 24, 30, 36, 42), SlowPatternPendingRow("buy", stairPrices[6], nil)), stairPrices[7], stairOpen(48).Add(time.Minute))
	breaking := stairBlock([]float64{100, 110, 120, 130, 140, 150, 160, 170, 180, 190, 200, 210, 220, 230, 240, 250, 260}, 48)
	if _, holds := slowpattern.Trigger(patternFills(rebuildState(eighth).fills), slowpattern.SeriesOf(breaking)); holds {
		t.Fatal("fixture drifted: the series must break the pattern at the eighth fill")
	}
	assertUntouched(t, Apply(eighth, "", underTheBand, withBlock(breaking)), "")
	got := Apply(eighth, "", slowDeclineBand+1, withBlock(breaking))
	assertForced(t, got, reasonSlowPatternBand)
	if got.SlowDecline != nil || got.Indecision != nil {
		t.Errorf("the last depth's fill is judged by nobody, got %+v", got)
	}
}

// ExitReached is Apply's own answer for the pattern's sale: it holds on the
// tick a ladder goes pending at the band and on the tick a new fill's judgement
// confirms the exit, and misses on a tick that cancels it, on one whose new fill
// is not judged yet, and under the band.
func TestSlowPatternExitReachedAgreesWithApply(t *testing.T) {
	band := slowDeclineBand + 1
	fourth, fifth := stairLadder(0, 9, 15, 24), stairFifthFill(t)
	for name, tc := range map[string]struct {
		reached bool
		price   float64
		got     Result
		exit    bool
	}{
		"going pending at the band":    {true, band, Apply(fourth, "", band, withBlock(stairBlock(stairTurns, 24))), ExitReached(fourth, band, stairBlock(stairTurns, 24))},
		"going pending under the band": {false, underTheBand, Apply(fourth, "", underTheBand, withBlock(stairBlock(stairTurns, 24))), ExitReached(fourth, underTheBand, stairBlock(stairTurns, 24))},
		"a confirming judgement":       {true, band, Apply(fifth, "", band, withBlock(stairBlock(stairTurns, 30))), ExitReached(fifth, band, stairBlock(stairTurns, 30))},
		"a cancelling judgement":       {false, band, Apply(fifth, "", band, withBlock(stairBlock(stairRally, 30))), ExitReached(fifth, band, stairBlock(stairRally, 30))},
		"a fill not judged yet":        {false, band, Apply(fifth, "", band, withBlock(stairBlock(stairTurns, 29))), ExitReached(fifth, band, stairBlock(stairTurns, 29))},
	} {
		if tc.exit != tc.reached || (tc.got.Reason == reasonSlowPatternBand) != tc.reached {
			t.Errorf("%s: ExitReached %v, Apply %+v, want the sale %v", name, tc.exit, tc.got, tc.reached)
		}
	}
}
