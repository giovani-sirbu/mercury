package swingdecline

import (
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/series"
)

// ladderOver is a ladder of depth fills stamped in the synthetic series' bars,
// the first in the bar first and the newest in the bar last, the others an even
// step between, each a price under the one before it. A ladder of one fill sits
// in the bar first.
func ladderOver(depth, first, last int) []series.Fill {
	fills := make([]series.Fill, depth)
	for index := range fills {
		fills[index] = fillIn(first+index*(last-first)/max(depth-1, 1), 100-float64(index))
	}
	return fills
}

// wholeDecline is the ladder most tests read: the staircase of declineLegs down
// legs and the fewest fills the rule reads, the first at its first high and the
// newest in the series' last bar. Its one window is the whole decline, which
// holds under every constant.
func wholeDecline() (series.Series, []series.Fill) {
	tape := declineSeries(declineLegs())
	return tape, ladderOver(MinDepthsBetween+1, legBars, len(tape.Opens)-1)
}

// frameOf is the frame the window of the synthetic series between two bars opens
// its reasons with: the depths of its two fills, then the bars' open times.
func frameOf(from, to, startBar, endBar int) string {
	return windowFrame(Reading{FromDepth: from, ToDepth: to, StartAt: barOpenAt(startBar), EndAt: barOpenAt(endBar)})
}

// leadOf is the bar the first fill's window starts on when the fill sits on bar:
// LookbackBars before it, or the first bar.
func leadOf(bar int) int {
	return max(0, bar-max(LookbackBars, 0))
}

// With the series ending at the bar of the newest fill the last bar IS that bar,
// and TriggerAtLastBar answers what Trigger answers, the whole reading, under any
// trend: the fixtures are the ones the Trigger tests read — a window passing, the
// longest first, no window holding, two fills in a bar, fills without a stamp, a
// ladder too shallow.
func TestTriggerAtLastBarAnswersAsTriggerWhenTheSeriesEndsAtTheNewestFill(t *testing.T) {
	declined, declinedFills := wholeDecline()
	staircased, rallied := staircase(legBars, staircaseTurns...), staircase(legBars, rallyTurns...)
	rallyEnd := (len(rallyTurns) - 1) * legBars
	for name, tc := range map[string]struct {
		tape  series.Series
		fills []series.Fill
	}{
		"a staircase":                  {staircased, stairFills()},
		"the longest window first":     {staircased, []series.Fill{fillIn(0, 100), fillIn(3*legBars, 94), fillIn(5*legBars, 88), fillIn(6*legBars, 81), fillIn(lastTurn, 77)}},
		"no window holding":            {rallied, []series.Fill{fillIn(0, 100), fillIn(2*legBars, 92), fillIn(4*legBars, 90), fillIn(rallyEnd, 86)}},
		"two fills in the first bar":   {rallied, []series.Fill{fillIn(0, 100), fillIn(0, 99), fillIn(2*legBars, 92), fillIn(4*legBars, 90), fillIn(rallyEnd, 86)}},
		"an unstamped first fill":      {staircased, []series.Fill{{Price: 130}, fillIn(2*legBars, 92), fillIn(4*legBars, 86), fillIn(6*legBars, 81), fillIn(lastTurn, 77)}},
		"an unstamped newest fill":     {staircased, []series.Fill{fillIn(0, 100), fillIn(2*legBars, 92), fillIn(4*legBars, 86), fillIn(6*legBars, 81), {Price: 77}}},
		"a ladder too shallow":         {staircased, []series.Fill{fillIn(0, 100), fillIn(lastTurn, 97)}},
		"the decline of the constants": {declined, declinedFills},
	} {
		for _, trend := range []series.Trend{bearishTrend, shortTrend, {}} {
			want, wantHolds := Trigger(tc.fills, tc.tape, trend)
			got, gotHolds := TriggerAtLastBar(tc.fills, tc.tape, trend)
			if gotHolds != wantHolds || !reflect.DeepEqual(got, want) {
				t.Errorf("%s, trend %+v: want Trigger's answer %+v, got %+v", name, trend, want, got)
			}
		}
	}
}

// A later last bar moves the end of the windows and nothing else: the staircase
// runs on past the newest fill's bar, still going down, and TriggerAtLastBar names
// the window from the same fill to the same depth, now ending at the last bar,
// while Trigger's still ends at the fill's — the window Read reads to that bar.
func TestTriggerAtLastBarOnlyMovesTheEndOfTheWindow(t *testing.T) {
	legs := declineLegs()
	tape := declineSeries(legs + 1)
	fills := ladderOver(MinDepthsBetween+1, legBars, 2*legs*legBars)
	atFill, holds := Trigger(fills, tape, bearishTrend)
	if !holds || atFill.EndAt != barOpenAt(2*legs*legBars) {
		t.Fatalf("fixture drifted: the decline must hold at the newest fill's bar, got holds %v, %+v", holds, atFill)
	}
	atLast, holds := TriggerAtLastBar(fills, tape, bearishTrend)
	if !holds || atLast.EndAt != tape.LastBarOpen() || atLast.EndAt == atFill.EndAt {
		t.Fatalf("the decline must hold and end at the last bar, got holds %v, %+v", holds, atLast)
	}
	if atLast.FromDepth != atFill.FromDepth || atLast.ToDepth != atFill.ToDepth || atLast.StartAt != atFill.StartAt {
		t.Errorf("only the end may move: Trigger read %+v, TriggerAtLastBar %+v", atFill, atLast)
	}
	window, _ := Read(tape, atLast.StartAt, tape.LastBarOpen(), atLast.FromDepth, atLast.ToDepth)
	if want := window.withTrend(bearishTrend, MinSmcTrendDown); !reflect.DeepEqual(atLast, want) {
		t.Errorf("the answer is the window Read reads to the last bar:\nwant %+v\ngot  %+v", want, atLast)
	}
}

// The answer follows the market: a window whose only up leg that counts is the
// provisional one — its confirmed bounce over the bound, the last leg a small
// rise so far — holds at the fill's bar, and once the rise runs past the bound
// TriggerAtLastBar takes the hold away while Trigger's window stays as it was.
func TestTriggerAtLastBarFollowsTheMarket(t *testing.T) {
	if MinLegsUp <= 0 {
		t.Skip("MinLegsUp is zero or below: no up leg is needed and a bounce takes nothing away")
	}
	turns := []float64{92.5, 100, 92.5, 96.5, 89, 90.5}
	fillBar := (len(turns) - 1) * legBars
	fills := ladderOver(MinDepthsBetween+1, legBars, fillBar)
	held := staircase(legBars, turns...)
	if reading, holds := Trigger(fills, held, bearishTrend); !holds || reading.UpLegs != 1 || reading.Bounces != 1 {
		t.Skipf("the constants do not hold the fixture on its provisional bounce alone: %+v", reading)
	}
	rallied := staircase(legBars, append(turns, 95)...)
	if reading, holds := Trigger(fills, rallied, bearishTrend); !holds {
		t.Fatalf("Trigger's window ends at the fill's bar and still holds, got %+v", reading)
	}
	reading, holds := TriggerAtLastBar(fills, rallied, bearishTrend)
	if holds || reading.Reasons != nil || reading.UpLegs != 0 || reading.Bounces != 2 || reading.Breaks[0] != frameOf(1, len(fills), leadOf(legBars), len(rallied.Opens)-1) {
		t.Errorf("a rally past the bound to the last bar must end the hold and name its window, got holds %v, %+v", holds, reading)
	}
}

// A shape that missed at the newest fill's bar and holds on a later one — the
// decline went on with no new fill — is TriggerAtLastBar's from that bar and never
// Trigger's: on the series cut at the fill's bar it misses, with Trigger's own
// answer, and on the whole series it holds.
func TestTriggerAtLastBarHoldsWhereTheNewestFillsBarMissed(t *testing.T) {
	tape := declineSeries(declineLegs())
	fillBar := 2 * legBars
	fills := ladderOver(MinDepthsBetween+1, legBars, fillBar)
	missed, holds := Trigger(fills, tape, bearishTrend)
	if holds {
		t.Skip("the constants let the first leg alone hold: no shape misses at the fill and holds later")
	}
	if missed.EndAt != barOpenAt(fillBar) {
		t.Fatalf("fixture drifted: the window to the newest fill's bar must be readable, got %+v", missed)
	}
	if atFill, holds := TriggerAtLastBar(fills, cut(tape, 0, fillBar+1), bearishTrend); holds || !reflect.DeepEqual(atFill, missed) {
		t.Errorf("cut at the newest fill's bar the ladder reads what Trigger reads, a miss, got holds %v, %+v", holds, atFill)
	}
	reading, holds := TriggerAtLastBar(fills, tape, bearishTrend)
	if !holds || reading.EndAt != tape.LastBarOpen() || reading.Reasons[0] != frameOf(1, len(fills), leadOf(legBars), len(tape.Opens)-1) {
		t.Errorf("on the whole series the decline holds, its window ending at the last bar, got holds %v, %+v", holds, reading)
	}
}

// A series that is not served — no bars, two arrays of different lengths, no
// wicks — has no window, and one whose start bar the series does not hold — the
// series older than the first fill's bar, or ending before it — is unreadable:
// with no window read the ladder never triggers, its Breaks naming why.
func TestTriggerAtLastBarFailsClosedOnBarsNotServed(t *testing.T) {
	tape, fills := wholeDecline()
	for name, served := range map[string]series.Series{
		"no bars":                            {},
		"arrays of different lengths":        {Opens: tape.Opens, Closes: tape.Closes[1:], Highs: tape.Highs, Lows: tape.Lows},
		"no wicks":                           {Opens: tape.Opens, Closes: tape.Closes},
		"the first fill older than the bars": cut(tape, legBars+1, len(tape.Opens)),
		"the bars ending before the fill":    cut(tape, 0, legBars),
	} {
		reading, holds := TriggerAtLastBar(fills, served, bearishTrend)
		if holds || reading.Holds || !slices.Equal(reading.Breaks, []string{unreadableBreak(served, len(fills))}) {
			t.Errorf("%s: want nothing read, got holds %v, %+v", name, holds, reading)
		}
	}
}

// The newest fill's bar need not be closed: with the newest fill in the bar after
// the series' last — still forming — the window from the first fill is read to the
// last closed bar and holds, where Trigger has no bar to end at and fails closed.
func TestTriggerAtLastBarDoesNotWaitForTheNewestFillsBarToClose(t *testing.T) {
	tape := declineSeries(declineLegs())
	last := len(tape.Opens) - 1
	fills := append(ladderOver(MinDepthsBetween, legBars, last-1), fillIn(last+1, 90))
	if reading, holds := Trigger(fills, tape, bearishTrend); holds || !slices.Equal(reading.Breaks, []string{unreadableBreak(tape, len(fills))}) {
		t.Fatalf("Trigger must fail closed on a newest fill not closed yet, got holds %v, %+v", holds, reading)
	}
	reading, holds := TriggerAtLastBar(fills, tape, bearishTrend)
	if !holds || reading.EndAt != tape.LastBarOpen() || reading.ToDepth != len(fills) {
		t.Errorf("the window must be read to the last closed bar and hold, got holds %v, %+v", holds, reading)
	}
}

// A newest fill without a stamp reads nothing, whatever the bars say, and fewer
// fills than one more than MinDepthsBetween are never read.
func TestTriggerAtLastBarReadsNothingOfAnUnstampedNewestFillOrAShallowLadder(t *testing.T) {
	tape, fills := wholeDecline()
	unstamped := slices.Clone(fills)
	unstamped[len(unstamped)-1].At = time.Time{}
	if reading, holds := TriggerAtLastBar(unstamped, tape, bearishTrend); holds || !slices.Equal(reading.Breaks, []string{unstampedBreak(len(unstamped))}) {
		t.Errorf("an unstamped newest fill must read nothing, got holds %v, %+v", holds, reading)
	}
	for _, shallow := range [][]series.Fill{nil, fills[:1], fills[:MinDepthsBetween]} {
		reading, holds := TriggerAtLastBar(shallow, tape, bearishTrend)
		if holds || !slices.Equal(reading.Breaks, []string{fewFillsBreak(len(shallow))}) || reading.ToDepth != len(shallow) {
			t.Errorf("%d fills: want nothing read, got holds %v, %+v", len(shallow), holds, reading)
		}
	}
	if reading, holds := TriggerAtLastBar(fills, tape, bearishTrend); !holds || reading.FromDepth != 1 || reading.ToDepth != MinDepthsBetween+1 {
		t.Errorf("the fewest fills the rule reads must read the window from the first, got holds %v, %+v", holds, reading)
	}
}

// Fills of one bar start one window each way: with two in the first bar the
// answer is the window from depth 1, led by the look-back, and with that one
// unstamped it is the second's, from the bar itself.
func TestTriggerAtLastBarReadsFillsOfOneBarAsOneWindow(t *testing.T) {
	tape := declineSeries(declineLegs())
	fills := append([]series.Fill{fillIn(legBars, 100)}, ladderOver(MinDepthsBetween+2, legBars, len(tape.Opens)-1)...)
	if reading, holds := TriggerAtLastBar(fills, tape, bearishTrend); !holds || reading.FromDepth != 1 || reading.StartAt != barOpenAt(leadOf(legBars)) {
		t.Fatalf("the window from the first of the two fills must answer, led by the look-back, got holds %v, %+v", holds, reading)
	}
	fills[0].At = time.Time{}
	if reading, holds := TriggerAtLastBar(fills, tape, bearishTrend); !holds || reading.FromDepth != 2 || reading.StartAt != barOpenAt(legBars) {
		t.Errorf("with the first unstamped the second must answer from the bar itself, got holds %v, %+v", holds, reading)
	}
}
