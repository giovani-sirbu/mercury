package swingdecline

import (
	"fmt"
	"slices"
	"testing"

	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/series"
)

// lastTurn is the bar of the last turn of the staircaseTurns staircase.
var lastTurn = 8 * legBars

// The hand-built staircase reads exactly as its legs say: the high at the 100
// on the bar legBars, the low at the 77 the window ends on, four down legs that
// count — 100 to 92, 94 to 86, 88 to 81 and the provisional 83 to 77 — and three
// bounces that count between them, none over the bound.
func TestReadStaircaseExactly(t *testing.T) {
	tape := staircase(legBars, staircaseTurns...)
	reading, readable := Read(tape, barOpenAt(0), barOpenAt(lastTurn), 1, 4)
	if !readable || !reading.Holds {
		t.Fatalf("the staircase must be read and hold, got readable %v, %+v", readable, reading)
	}
	if reading.High != 100 || reading.HighAt != barOpenAt(legBars) || reading.Low != 77 || reading.LowAt != barOpenAt(lastTurn) {
		t.Errorf("want the high 100 on the bar %d and the low 77 on the bar %d, got %+v", legBars, lastTurn, reading)
	}
	if reading.DownLegs != 4 || reading.UpLegs != 3 || reading.Bounces != 0 || reading.FromDepth != 1 || reading.ToDepth != 4 {
		t.Errorf("want 4 down legs, 3 up legs, no bounce over the bound, depth 1 to 4, got %+v", reading)
	}
	if reading.StartAt != barOpenAt(0) || reading.EndAt != barOpenAt(lastTurn) || reading.Breaks != nil {
		t.Errorf("want the window's bars named and no breaks, got %+v", reading)
	}
	want := []string{
		fmt.Sprintf("depth 1 to 4, 1h bars %s to %s UTC", barTime(barOpenAt(0)), barTime(barOpenAt(lastTurn))),
		fmt.Sprintf("high 100 at %s UTC, low 77 at %s UTC, down 23.0%%", barTime(barOpenAt(legBars)), barTime(barOpenAt(lastTurn))),
		fmt.Sprintf("4 legs down of %d needed, each %d bars and %.1f%% down at least", MinLegsDown, MinBarsBetween, MinDropPct),
		fmt.Sprintf("3 legs up of %d needed, each %.1f%% up at most", MinLegsUp, MaxBouncePct),
	}
	if !slices.Equal(reading.Reasons, want) {
		t.Errorf("reasons:\ngot  %q\nwant %q", reading.Reasons, want)
	}
}

// A window starts at the highest swing high at or after its start bar: cut at
// the 92 after the first high, the 94 is the high and its leg fall away, leaving
// three down legs and two bounces. A swing high on the start bar counts, one a
// bar before does not.
func TestReadStartsAtTheHighestSwingHighAfterTheStart(t *testing.T) {
	tape := staircase(legBars, staircaseTurns...)
	reading, readable := Read(tape, barOpenAt(2*legBars), barOpenAt(lastTurn), 2, 4)
	if !readable || !reading.Holds {
		t.Fatalf("the cut staircase must be read and hold, got %v, %+v", readable, reading)
	}
	if reading.High != 94 || reading.HighAt != barOpenAt(3*legBars) || reading.DownLegs != 3 || reading.UpLegs != 2 {
		t.Errorf("want the high 94 on the bar %d, 3 down legs and 2 up legs, got %+v", 3*legBars, reading)
	}

	onTheBar, _ := Read(tape, barOpenAt(legBars), barOpenAt(lastTurn), 1, 4)
	pastTheBar, _ := Read(tape, barOpenAt(legBars+1), barOpenAt(lastTurn), 1, 4)
	if onTheBar.High != 100 || pastTheBar.High != 94 {
		t.Errorf("the swing high on the start bar counts and one before it does not, got the highs %v and %v", onTheBar.High, pastTheBar.High)
	}
}

// Bars after the end bar never change a reading: its pivots are the ones known
// at its end, so the same fills read the same on any later tick.
func TestReadIgnoresBarsAfterTheEnd(t *testing.T) {
	short := staircase(legBars, staircaseTurns...)
	long := staircase(legBars, append(append([]float64{}, staircaseTurns...), 99, 70, 120)...)
	want, _ := Read(short, barOpenAt(0), barOpenAt(lastTurn), 1, 4)
	got, _ := Read(long, barOpenAt(0), barOpenAt(lastTurn), 1, 4)
	if want.High != got.High || want.Low != got.Low || want.DownLegs != got.DownLegs || want.UpLegs != got.UpLegs || !slices.Equal(want.Reasons, got.Reasons) {
		t.Fatalf("later bars changed the reading:\nbefore %+v\nafter  %+v", want, got)
	}
}

// The legs are read on the wicks: a staircase whose closes fall short of
// MinDropPct holds once the bars' wicks reach past it, the high and the low the
// wicks' and not the closes'.
func TestReadMeasuresTheWicks(t *testing.T) {
	flat := staircase(legBars, 96, 100, 96, 96.5, 92.5, 93, 89)
	end := barOpenAt(6 * legBars)
	if reading, readable := Read(flat, barOpenAt(0), end, 1, 4); !readable || reading.Holds || reading.DownLegs != 0 {
		t.Fatalf("on flat wicks the four percent legs must not count, got readable %v, %+v", readable, reading)
	}
	wicked := withWicks(flat, 1)
	reading, readable := Read(wicked, barOpenAt(0), end, 1, 4)
	if !readable || !reading.Holds || reading.DownLegs != 3 || reading.UpLegs != 2 {
		t.Fatalf("on one percent wicks the same legs must count, got readable %v, %+v", readable, reading)
	}
	assertNear(t, "the high wick", reading.High, 101, 1e-9)
	assertNear(t, "the low wick", reading.Low, 89*0.99, 1e-9)
}

// Each count that falls short names its break after the window's frame and the
// swings, and nothing else: down legs too short in bars or in their fall, up
// legs all over the bound — named as over it — or both.
func TestReadNamesTheBreakOfEveryCount(t *testing.T) {
	downShort := func(legs int) string {
		return fmt.Sprintf("%d legs down short of %d needed, each %d bars and %.1f%% down at least", legs, MinLegsDown, MinBarsBetween, MinDropPct)
	}
	upShort := func(legs, over int) string {
		return fmt.Sprintf("%d legs up short of %d needed, each %.1f%% up at most, %d over it", legs, MinLegsUp, MaxBouncePct, over)
	}
	for name, tc := range map[string]struct {
		barsPerLeg int
		turns      []float64
		breaks     []string
	}{
		"down legs short in bars":     {4, []float64{92, 100, 92, 94, 86, 88, 81}, []string{downShort(0)}},
		"down legs short in the fall": {legBars, []float64{97, 100, 97, 98, 95, 96, 93}, []string{downShort(0)}},
		"up legs over the bound":      {legBars, rallyTurns, []string{upShort(0, 2)}},
		"both short":                  {legBars, []float64{96, 100, 96, 99.5, 95.5, 99, 95}, []string{downShort(0), upShort(0, 2)}},
	} {
		end := (len(tc.turns) - 1) * tc.barsPerLeg
		reading, readable := Read(staircase(tc.barsPerLeg, tc.turns...), barOpenAt(0), barOpenAt(end), 1, 4)
		if !readable || reading.Holds || reading.Reasons != nil {
			t.Errorf("%s: want a readable reading that does not hold, got readable %v, %+v", name, readable, reading)
			continue
		}
		if len(reading.Breaks) != len(tc.breaks)+2 || reading.Breaks[0] != windowFrame(reading) || reading.Breaks[1] != swingFrame(reading) || !slices.Equal(reading.Breaks[2:], tc.breaks) {
			t.Errorf("%s: breaks %q, want the frame, the swings, then %q", name, reading.Breaks, tc.breaks)
		}
	}
}

// The provisional leg runs from the last swing to the extreme wick so far: from
// the swing high to the lowest low after it, which is the window's low even
// before a swing low is known, and on a flat tail a fall of nothing.
func TestReadRunsTheProvisionalLegToTheExtremeSoFar(t *testing.T) {
	tape := seriesOfCloses(1, 2, 3, 4, 9, 4, 3, 2, 1)
	reading, readable := Read(tape, barOpenAt(0), barOpenAt(8), 1, 4)
	if !readable || reading.Holds || reading.Low != 1 || reading.LowAt != barOpenAt(8) {
		t.Fatalf("the window must be read with its provisional leg to the 1 on the last bar, got readable %v, %+v", readable, reading)
	}
	flat := seriesOfCloses(1, 2, 3, 4, 9, 9, 9, 9, 9)
	reading, readable = Read(flat, barOpenAt(0), barOpenAt(8), 1, 4)
	if !readable || reading.Low != 9 || reading.Breaks[1] != fmt.Sprintf("high 9 at %s UTC, low 9 at %s UTC, down 0.0%%", barTime(barOpenAt(4)), barTime(barOpenAt(5))) {
		t.Fatalf("a flat tail is a provisional leg of nothing, got readable %v, %+v", readable, reading)
	}
}

// The counts meet their constants with the bound included: as many legs as
// needed hold, one fewer does not.
func TestHoldsIncludesTheBound(t *testing.T) {
	bound := Reading{DownLegs: MinLegsDown, UpLegs: MinLegsUp}
	if !holds(bound) {
		t.Fatalf("every count at its bound must hold, got %+v", bound)
	}
	for name, miss := range map[string]func(*Reading){
		"down legs": func(r *Reading) { r.DownLegs-- },
		"up legs":   func(r *Reading) { r.UpLegs-- },
	} {
		reading := bound
		miss(&reading)
		if (name == "down legs" && MinLegsDown <= 0) || (name == "up legs" && MinLegsUp <= 0) {
			continue
		}
		if holds(reading) {
			t.Errorf("%s one under its bound must not hold, got %+v", name, reading)
		}
	}
}

// A window that cannot be read is unreadable, never a reading that does not
// hold: a start or an end bar the series lacks, an end before the start, too
// few bars, no swing high, a price at zero, a high under its low, wicks not
// served, arrays not one series.
func TestReadUnreadable(t *testing.T) {
	staircased := staircase(legBars, staircaseTurns...)
	falling := seriesOfCloses(30, 29, 28, 27, 26, 25, 24, 23, 22, 21, 20, 19)
	valley := seriesOfCloses(10, 9, 8, 7, 6, 5, 4, 5, 6, 7, 8, 9, 10)
	zeroed := staircase(legBars, staircaseTurns...)
	zeroed.Closes = slices.Clone(zeroed.Closes)
	zeroed.Closes[10] = 0
	crossed := staircase(legBars, staircaseTurns...)
	crossed.Lows = slices.Clone(crossed.Lows)
	crossed.Lows[10] = crossed.Highs[10] + 1
	mismatched := series.Series{Opens: staircased.Opens, Closes: staircased.Closes[:20], Highs: staircased.Highs, Lows: staircased.Lows}
	unwicked := series.Series{Opens: staircased.Opens, Closes: staircased.Closes}
	halfWicked := series.Series{Opens: staircased.Opens, Closes: staircased.Closes, Highs: staircased.Highs, Lows: staircased.Lows[:20]}
	gapped := series.Series{Opens: slices.Clone(staircased.Opens), Closes: staircased.Closes, Highs: staircased.Highs, Lows: staircased.Lows}
	gapped.Opens[20] += series.BarMs / 2

	for name, tc := range map[string]struct {
		tape           series.Series
		startAt, endAt int64
	}{
		"a start before the series":     {staircased, barOpenAt(-3), barOpenAt(lastTurn)},
		"an end after the series":       {staircased, barOpenAt(0), barOpenAt(lastTurn + 1)},
		"a start bar not in the series": {gapped, barOpenAt(20), barOpenAt(lastTurn)},
		"an end before the start":       {staircased, barOpenAt(20), barOpenAt(4)},
		"too few bars":                  {staircased, barOpenAt(10), barOpenAt(10 + 2*PivotBars - 1)},
		"no swing high at all":          {falling, barOpenAt(0), barOpenAt(11)},
		"a swing low and no swing high": {valley, barOpenAt(0), barOpenAt(12)},
		"a close at zero":               {zeroed, barOpenAt(0), barOpenAt(lastTurn)},
		"a high under its low":          {crossed, barOpenAt(0), barOpenAt(lastTurn)},
		"arrays of different lengths":   {mismatched, barOpenAt(0), barOpenAt(19)},
		"no wicks":                      {unwicked, barOpenAt(0), barOpenAt(lastTurn)},
		"lows of another length":        {halfWicked, barOpenAt(0), barOpenAt(19)},
		"no series":                     {series.Series{}, barOpenAt(0), barOpenAt(lastTurn)},
	} {
		if reading, readable := Read(tc.tape, tc.startAt, tc.endAt, 1, 4); readable || reading.Holds || reading.Reasons != nil {
			t.Errorf("%s: want unreadable, got readable %v, %+v", name, readable, reading)
		}
	}
}

// A series that does not move is read and does not hold: every bar is a swing,
// every leg a move of nothing, and no down leg counts.
func TestReadAFlatSeriesDoesNotHold(t *testing.T) {
	flat := seriesOfCloses(10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10)
	if reading, readable := Read(flat, barOpenAt(0), barOpenAt(11), 1, 4); !readable || reading.Holds || reading.DownLegs != 0 {
		t.Fatalf("a flat series is readable and holds nothing, got readable %v, %+v", readable, reading)
	}
}

// Twice PivotBars bars and one, a bar and PivotBars each side, is the smallest window.
func TestReadAtTheSmallestWindow(t *testing.T) {
	closes := make([]float64, 2*PivotBars+1)
	for index := range closes {
		closes[index] = float64(1 + min(index, 2*PivotBars-index))
	}
	closes[PivotBars] = 9
	tape := seriesOfCloses(closes...)
	if reading, readable := Read(tape, barOpenAt(0), barOpenAt(2*PivotBars), 1, 4); !readable || reading.High != 9 {
		t.Fatalf("the smallest window with a swing high in the middle is readable, got readable %v, %+v", readable, reading)
	}
}
