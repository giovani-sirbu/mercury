package slowpattern

import (
	"slices"
	"testing"
)

// staircaseTurns is a decline of four steps — each down leg from a lower high,
// each bounce smaller than the leg before it — four bars a leg, starting a
// leg below its first high so that high is a swing, and ending on a leg still
// going down: bars 0 to 32.
var staircaseTurns = []float64{94, 100, 96, 98, 93, 95, 90, 92, 88}

// The hand-built staircase reads exactly as its legs say: four down legs of
// 100 to 96, 98 to 93, 95 to 90 and the provisional 92 to 88, three bounces
// between them, three lower highs, and a net move from the 94 the window
// starts at to the 88 it ends at.
func TestReadStaircaseExactly(t *testing.T) {
	series := staircase(4, staircaseTurns...)
	reading, readable := Read(series, barOpenAt(0), barOpenAt(32), 1, 4)
	if !readable || !reading.Holds {
		t.Fatalf("the staircase must be read and hold, got readable %v, %+v", readable, reading)
	}
	fall := -move(100, 96) - move(98, 93) - move(95, 90) - move(92, 88)
	rise := move(96, 98) + move(93, 95) + move(90, 92)
	assertNear(t, "weight", reading.Weight, (fall-rise)/(fall+rise), 1e-9)
	assertNear(t, "weight, rounded", reading.Weight, 0.487, 0.0005)
	assertNear(t, "largest leg share", reading.MaxLegShare, -move(95, 90)/fall, 1e-9)
	assertNear(t, "net", reading.NetPct, move(94, 88), 1e-9)
	if reading.DownLegs != 4 || reading.LowerHighs != 3 || reading.FromDepth != 1 || reading.ToDepth != 4 {
		t.Errorf("want 4 down legs, 3 lower highs, depth 1 to 4, got %+v", reading)
	}
	if reading.StartAt != barOpenAt(0) || reading.EndAt != barOpenAt(32) || reading.Breaks != nil {
		t.Errorf("want the window's bars named and no breaks, got %+v", reading)
	}
	want := []string{
		"depth 1 to 4, 1h bars 2021-10-01 00:00 to 2021-10-02 08:00 UTC",
		"weight 0.49 over 0.33",
		"4 legs down of 3 needed",
		"3 lower highs of 2 needed",
		"largest leg 28% of the fall under 60%",
		"down 6.4% past 4.0%",
	}
	if !slices.Equal(reading.Reasons, want) {
		t.Errorf("reasons:\ngot  %q\nwant %q", reading.Reasons, want)
	}
}

// A window starts at the highest swing high at or after its start bar: cut at
// the 98 the first high and its leg fall away, leaving three down legs and two
// lower highs. A swing high on the start bar counts, one a bar before does not.
func TestReadStartsAtTheHighestSwingHighAfterTheStart(t *testing.T) {
	series := staircase(4, staircaseTurns...)
	reading, readable := Read(series, barOpenAt(12), barOpenAt(32), 2, 4)
	if !readable || !reading.Holds {
		t.Fatalf("the cut staircase must be read and hold, got %v, %+v", readable, reading)
	}
	fall := -move(98, 93) - move(95, 90) - move(92, 88)
	rise := move(93, 95) + move(90, 92)
	assertNear(t, "weight", reading.Weight, (fall-rise)/(fall+rise), 1e-9)
	assertNear(t, "net", reading.NetPct, move(98, 88), 1e-9)
	if reading.DownLegs != 3 || reading.LowerHighs != 2 {
		t.Errorf("want 3 down legs and 2 lower highs, got %+v", reading)
	}

	onTheBar, _ := Read(series, barOpenAt(4), barOpenAt(32), 1, 4)
	pastTheBar, _ := Read(series, barOpenAt(5), barOpenAt(32), 1, 4)
	if onTheBar.DownLegs != 4 || pastTheBar.DownLegs != 3 {
		t.Errorf("the swing high on the start bar counts and one before it does not, got %d and %d down legs", onTheBar.DownLegs, pastTheBar.DownLegs)
	}
}

// Bars after the end bar never change a reading: its pivots are the ones known
// at its end, so the same fills read the same on any later tick.
func TestReadIgnoresBarsAfterTheEnd(t *testing.T) {
	short := staircase(4, staircaseTurns...)
	long := staircase(4, append(append([]float64{}, staircaseTurns...), 99, 80, 120)...)
	want, _ := Read(short, barOpenAt(0), barOpenAt(32), 1, 4)
	got, _ := Read(long, barOpenAt(0), barOpenAt(32), 1, 4)
	if want.Weight != got.Weight || want.DownLegs != got.DownLegs || want.NetPct != got.NetPct || !slices.Equal(want.Reasons, got.Reasons) {
		t.Fatalf("later bars changed the reading:\nbefore %+v\nafter  %+v", want, got)
	}
}

// Every threshold flipped to fail, alone where the shape allows it, names its
// break after the window's frame and nothing else. The legs and the lower
// highs go together: the lower highs are pairs of down legs, so a window short
// of the legs is short of them too.
func TestReadNamesTheBreakOfEveryThreshold(t *testing.T) {
	frame := "depth 1 to 4, 1h bars 2021-10-01 00:00 to 2021-10-0"
	for name, tc := range map[string]struct {
		turns  []float64
		breaks []string
	}{
		"weight": {
			turns:  []float64{94, 100, 94, 99, 93, 98, 92, 97, 89},
			breaks: []string{"weight 0.24 under 0.33"},
		},
		"down legs and lower highs": {
			turns:  []float64{94, 100, 92, 98, 88},
			breaks: []string{"2 legs down short of 3 needed", "1 lower highs short of 2 needed"},
		},
		"lower highs": {
			turns:  []float64{94, 100, 96, 98, 93, 99, 88},
			breaks: []string{"1 lower highs short of 2 needed"},
		},
		"largest leg": {
			turns:  []float64{94, 100, 99, 99.5, 98.5, 99.2, 80},
			breaks: []string{"largest leg 91% of the fall over 60%"},
		},
		"net fall": {
			turns:  []float64{99, 100, 97.5, 98.5, 96.5, 97.5, 95.5, 96.5, 95.5},
			breaks: []string{"down 3.5% short of 4.0%"},
		},
		"a net rise": {
			turns:  []float64{80, 100, 96, 98, 93, 95, 90, 92, 88},
			breaks: []string{"up 10.0% short of 4.0% down"},
		},
	} {
		reading, readable := Read(staircase(4, tc.turns...), barOpenAt(0), barOpenAt(4*(len(tc.turns)-1)), 1, 4)
		if !readable || reading.Holds || reading.Reasons != nil {
			t.Errorf("%s: want a readable reading that does not hold, got readable %v, %+v", name, readable, reading)
			continue
		}
		if len(reading.Breaks) != len(tc.breaks)+1 || reading.Breaks[0][:len(frame)] != frame || !slices.Equal(reading.Breaks[1:], tc.breaks) {
			t.Errorf("%s: breaks %q, want the frame then %q", name, reading.Breaks, tc.breaks)
		}
	}
}

// Each reading meets its constant with the bound included: a weight at the
// least, legs and lower highs at the least, a share at the most and a net at
// the fall — or past it — holds, and a hair the other way does not.
func TestHoldsIncludesTheBound(t *testing.T) {
	bound := Reading{
		Weight: SlowPatternMinWeight, DownLegs: SlowPatternMinDownLegs, LowerHighs: SlowPatternMinLowerHighs,
		MaxLegShare: SlowPatternMaxLegShare, NetPct: SlowPatternMinFallPct,
	}
	if !holds(bound) {
		t.Fatalf("every reading at its bound must hold, got %+v", bound)
	}
	for name, miss := range map[string]func(*Reading){
		"weight":      func(r *Reading) { r.Weight -= 1e-9 },
		"down legs":   func(r *Reading) { r.DownLegs-- },
		"lower highs": func(r *Reading) { r.LowerHighs-- },
		"share":       func(r *Reading) { r.MaxLegShare += 1e-9 },
		"net":         func(r *Reading) { r.NetPct += 1e-9 },
	} {
		reading := bound
		miss(&reading)
		if holds(reading) {
			t.Errorf("%s a hair under its bound must not hold, got %+v", name, reading)
		}
	}
}

// A window that cannot be read is unreadable, never a reading that does not
// hold: a start or an end bar the series lacks, an end before the start, too
// few bars, no swing high, no move, a close at zero, arrays not one series.
func TestReadUnreadable(t *testing.T) {
	staircased := staircase(4, staircaseTurns...)
	falling := seriesOfCloses(30, 29, 28, 27, 26, 25, 24, 23, 22, 21, 20, 19)
	valley := seriesOfCloses(10, 9, 8, 7, 6, 5, 4, 5, 6, 7, 8, 9, 10)
	flat := seriesOfCloses(10, 10, 10, 10, 10, 10, 10, 10, 10, 10)
	zeroed := staircase(4, staircaseTurns...)
	zeroed.Closes[10] = 0
	mismatched := Series{Opens: staircased.Opens, Closes: staircased.Closes[:20]}
	gapped := Series{Opens: slices.Clone(staircased.Opens), Closes: staircased.Closes}
	gapped.Opens[20] += SlowPatternBarMs / 2

	for name, tc := range map[string]struct {
		series         Series
		startAt, endAt int64
	}{
		"a start before the series":     {staircased, barOpenAt(-3), barOpenAt(32)},
		"an end after the series":       {staircased, barOpenAt(0), barOpenAt(33)},
		"a start bar not in the series": {gapped, barOpenAt(20), barOpenAt(32)},
		"an end before the start":       {staircased, barOpenAt(20), barOpenAt(4)},
		"six bars":                      {staircased, barOpenAt(10), barOpenAt(15)},
		"no swing high at all":          {falling, barOpenAt(0), barOpenAt(11)},
		"a swing low and no swing high": {valley, barOpenAt(0), barOpenAt(12)},
		"no move between the swings":    {flat, barOpenAt(0), barOpenAt(9)},
		"a close at zero":               {zeroed, barOpenAt(0), barOpenAt(32)},
		"arrays of different lengths":   {mismatched, barOpenAt(0), barOpenAt(19)},
		"no series":                     {Series{}, barOpenAt(0), barOpenAt(32)},
	} {
		if reading, readable := Read(tc.series, tc.startAt, tc.endAt, 1, 4); readable || reading.Holds || reading.Reasons != nil {
			t.Errorf("%s: want unreadable, got readable %v, %+v", name, readable, reading)
		}
	}
}

// Seven bars, a bar and three each side, is the smallest window.
func TestReadAtTheSmallestWindow(t *testing.T) {
	series := seriesOfCloses(1, 2, 3, 9, 3, 2, 1)
	if reading, readable := Read(series, barOpenAt(0), barOpenAt(6), 1, 4); !readable || reading.DownLegs != 1 {
		t.Fatalf("seven bars with a swing high in the middle are readable with one down leg, got readable %v, %+v", readable, reading)
	}
}
