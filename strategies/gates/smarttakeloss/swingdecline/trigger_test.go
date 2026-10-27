package swingdecline

import (
	"slices"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/series"
)

// stairFills are the fewest fills the rule reads over the staircaseTurns
// staircase: the first in its first bar, the newest on its last turn.
func stairFills() []series.Fill {
	return []series.Fill{fillIn(0, 100), fillIn(2*legBars, 99), fillIn(4*legBars, 98), fillIn(lastTurn, 97)}
}

// Fewer fills than one more than MinDepthsBetween are never read,
// whatever the bars say: three fills over a staircase that holds at depth four
// read nothing, with the reason named.
func TestTriggerNeverReadsFewerFillsThanTheDepthsBetweenNeed(t *testing.T) {
	tape := staircase(legBars, staircaseTurns...)
	ladder := stairFills()
	for _, fills := range [][]series.Fill{nil, ladder[:1], ladder[:MinDepthsBetween]} {
		reading, holds := Trigger(fills, tape, bearishTrend)
		want := []string{fewFillsBreak(len(fills))}
		if holds || reading.Holds || !slices.Equal(reading.Breaks, want) || reading.ToDepth != len(fills) {
			t.Errorf("%d fills: want nothing read, got holds %v, %+v", len(fills), holds, reading)
		}
	}
	if reading, holds := Trigger(ladder, tape, bearishTrend); !holds || reading.FromDepth != 1 || reading.ToDepth != MinDepthsBetween+1 {
		t.Fatalf("the fourth fill must read the window from the first, got holds %v, %+v", holds, reading)
	}
}

// The windows run from every earlier fill at least MinDepthsBetween
// back, longest first, and the ladder triggers if ANY of them holds: here the
// series does not reach back to the first fill's bar, so its window is
// unreadable and the one from the second fill answers.
func TestTriggerReadsTheNextWindowWhenTheFirstFillsBarIsNotServed(t *testing.T) {
	tape := staircase(legBars, staircaseTurns...)
	young := cut(tape, 1, len(tape.Opens))
	fills := []series.Fill{fillIn(0, 130), fillIn(2*legBars, 92), fillIn(4*legBars, 86), fillIn(6*legBars, 81), fillIn(lastTurn, 77)}
	reading, holds := Trigger(fills, young, bearishTrend)
	if !holds || reading.FromDepth != 2 || reading.ToDepth != 5 || reading.StartAt != barOpenAt(2*legBars) || reading.EndAt != barOpenAt(lastTurn) {
		t.Fatalf("the window from the second fill must hold, got holds %v, %+v", holds, reading)
	}
	if _, readable := Read(young, barOpenAt(0), barOpenAt(lastTurn), 1, 5); readable {
		t.Fatal("the window from the first fill must be unreadable on bars that start after it")
	}
}

// Longest first: when more than one window holds, the one from the earliest
// fill is the answer.
func TestTriggerReadsTheLongestWindowFirst(t *testing.T) {
	tape := staircase(legBars, staircaseTurns...)
	fills := []series.Fill{fillIn(0, 100), fillIn(3*legBars, 94), fillIn(5*legBars, 88), fillIn(6*legBars, 81), fillIn(lastTurn, 77)}
	for from, startBar := range map[int]int{1: 0, 2: 3 * legBars} {
		if window, readable := Read(tape, barOpenAt(startBar), barOpenAt(lastTurn), from, 5); !readable || !window.Holds {
			t.Fatalf("the window from depth %d must hold on its own, got readable %v, %+v", from, readable, window)
		}
	}
	reading, holds := Trigger(fills, tape, bearishTrend)
	if !holds || reading.FromDepth != 1 || reading.StartAt != barOpenAt(0) {
		t.Fatalf("the window from the first fill must answer first, got holds %v, %+v", holds, reading)
	}
}

// No window holding, the answer is the longest readable reading with the
// breaks that decided it, and two fills in the first bar read as one window.
func TestTriggerNoWindowHoldsAnswersTheLongestBreaks(t *testing.T) {
	tape := staircase(legBars, rallyTurns...)
	end := (len(rallyTurns) - 1) * legBars
	fills := []series.Fill{fillIn(0, 100), fillIn(0, 99), fillIn(2*legBars, 92), fillIn(4*legBars, 90), fillIn(end, 86)}
	reading, holds := Trigger(fills, tape, bearishTrend)
	if holds || reading.Holds || reading.FromDepth != 1 || reading.ToDepth != 5 {
		t.Fatalf("nothing holds from the first bar, want the window from depth 1, got holds %v, %+v", holds, reading)
	}
	window, _ := Read(tape, barOpenAt(0), barOpenAt(end), 1, 5)
	if len(reading.Breaks) != 3 || !slices.Equal(reading.Breaks, window.Breaks) || reading.Breaks[2] != upLegsReason(window, true) {
		t.Errorf("want the frame, the swings and the up legs short, got %q", reading.Breaks)
	}
}

// A fill without a stamp has no bar: its window is skipped and the next fill's
// is read, and a newest fill without one ends no window at all.
func TestTriggerSkipsUnstampedFills(t *testing.T) {
	tape := staircase(legBars, staircaseTurns...)
	fills := []series.Fill{{Price: 130}, fillIn(2*legBars, 92), fillIn(4*legBars, 86), fillIn(6*legBars, 81), fillIn(lastTurn, 77)}
	if reading, holds := Trigger(fills, tape, bearishTrend); !holds || reading.FromDepth != 2 {
		t.Fatalf("the unstamped first fill must be skipped for the second, got holds %v, %+v", holds, reading)
	}
	fills[4] = series.Fill{Price: 77, At: time.Time{}}
	reading, holds := Trigger(fills, tape, bearishTrend)
	if holds || !slices.Equal(reading.Breaks, []string{unstampedBreak(5)}) {
		t.Fatalf("an unstamped newest fill must read nothing, got holds %v, %+v", holds, reading)
	}
}

// A window the served bars do not hold is unreadable and the ladder is not
// triggered, with the reason named: no bars served, no wicks served, or the
// fills' bars outside them — older than the series' first bar, or newer than its last.
func TestTriggerFailsClosedOnBarsNotServed(t *testing.T) {
	tape := staircase(legBars, staircaseTurns...)
	fills := stairFills()
	old := cut(tape, legBars, len(tape.Opens))
	young := cut(tape, 0, lastTurn-1)
	unwicked := series.Series{Opens: tape.Opens, Closes: tape.Closes}
	for name, tc := range map[string]struct {
		tape series.Series
		want string
	}{
		"no bars":                        {series.Series{}, unreadableBreak(series.Series{}, 4)},
		"no wicks":                       {unwicked, unreadableBreak(unwicked, 4)},
		"the first fill older than them": {old, unreadableBreak(old, 4)},
		"the newest fill not closed yet": {young, unreadableBreak(young, 4)},
	} {
		reading, holds := Trigger(fills, tc.tape, bearishTrend)
		if holds || !slices.Equal(reading.Breaks, []string{tc.want}) {
			t.Errorf("%s: want %q, got holds %v, %+v", name, tc.want, holds, reading)
		}
	}
	if unreadableBreak(unwicked, 4) == unreadableBreak(series.Series{}, 4) || unreadableBreak(unwicked, 4) == unreadableBreak(old, 4) {
		t.Error("wicks not served are named apart from no bars and no window")
	}
}

// The answer is the same on every later tick for the same trend: the bars after
// the newest fill's own bar never change it.
func TestTriggerIsDeterministicAsBarsGrow(t *testing.T) {
	staircased := staircase(legBars, staircaseTurns...)
	grown := staircase(legBars, append(append([]float64{}, staircaseTurns...), 99, 120, 60)...)
	fills := stairFills()
	want, _ := Trigger(fills, staircased, bearishTrend)
	got, _ := Trigger(fills, grown, bearishTrend)
	if want.High != got.High || want.Low != got.Low || want.DownLegs != got.DownLegs || want.UpLegs != got.UpLegs || want.Holds != got.Holds || !slices.Equal(want.Reasons, got.Reasons) {
		t.Fatalf("later bars changed the answer:\nbefore %+v\nafter  %+v", want, got)
	}
}

// The first fill's window starts LookbackBars before its bar, so the swing high
// the ladder fell from is in it: a first fill after the 100 reads the 100 as its
// high where a window from its own bar would read the 94; a lead reaching before
// the series starts at the first bar; and a later fill's window is not led.
func TestTriggerLeadsTheFirstFillsWindowByTheLookbackBars(t *testing.T) {
	if LookbackBars <= 0 {
		t.Skip("LookbackBars is zero or below: the first fill's window starts at its bar")
	}
	tape := staircase(legBars, staircaseTurns...)
	first := 2*legBars + 2
	fills := []series.Fill{fillIn(first, 92), fillIn(4*legBars, 86), fillIn(6*legBars, 81), fillIn(lastTurn, 77)}
	reading, holds := Trigger(fills, tape, bearishTrend)
	lead := max(0, first-LookbackBars)
	if !holds || reading.StartAt != barOpenAt(lead) || reading.FromDepth != 1 {
		t.Fatalf("the window must start %d bars before the first fill, at the bar %d, got holds %v, %+v", LookbackBars, lead, holds, reading)
	}
	if lead <= legBars && reading.High != 100 {
		t.Errorf("a lead reaching the 100 reads it as the high, got %v", reading.High)
	}
	own, _ := Read(tape, barOpenAt(first), barOpenAt(lastTurn), 1, 4)
	if own.High != 94 {
		t.Errorf("fixture drifted: the window from the fill's own bar reads the 94, got %v", own.High)
	}

	unled := []series.Fill{{Price: 100}, fillIn(first, 92), fillIn(4*legBars, 86), fillIn(6*legBars, 81), fillIn(lastTurn, 77)}
	if reading, holds := Trigger(unled, tape, bearishTrend); !holds || reading.FromDepth != 2 || reading.StartAt != barOpenAt(first) {
		t.Errorf("the second fill's window starts at its own bar, got holds %v, %+v", holds, reading)
	}
}
