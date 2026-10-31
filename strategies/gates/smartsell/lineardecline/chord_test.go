package lineardecline

import (
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/series"
	"math"
	"testing"
	"time"
)

// A chord is the straight line through the first fill and a later one, price
// against stamp: it takes the price of each fill at its stamp, runs on past the
// later one, and its slope is the net move in percent of the first fill's price per
// hour between the stamps, so the same fall reads alike at any price and a longer
// time reads gentler.
func TestAChordIsTheLineThroughTheFirstFillAndALaterOne(t *testing.T) {
	first, later := fillIn(0, 100), fillIn(12, 88)
	line, broken := newChord(first, later)
	if broken != 0 {
		t.Fatalf("two fills in order draw a line, got misses %b", broken)
	}
	firstMs, laterMs := first.At.UnixMilli(), later.At.UnixMilli()
	assertNear(t, "price at the first fill", line.at(firstMs), 100, 1e-9)
	assertNear(t, "price at the later fill", line.at(laterMs), 88, 1e-9)
	assertNear(t, "price halfway", line.at((firstMs+laterMs)/2), 94, 1e-9)
	assertNear(t, "price an hour past the later fill", line.at(laterMs+time.Hour.Milliseconds()), 87, 1e-9)
	for _, tc := range []struct {
		name         string
		first, later series.Fill
		want         float64
	}{
		{"a fall over twelve hours", first, later, -1},
		{"the same fall over six hours", fillIn(0, 100), fillIn(6, 94), -1},
		{"ten times the price", fillIn(0, 1000), fillIn(6, 940), -1},
		{"half the fall in the same time", fillIn(0, 100), fillIn(12, 94), -0.5},
		{"a rise", fillIn(0, 100), fillIn(4, 104), 1},
	} {
		line, _ := newChord(tc.first, tc.later)
		assertNear(t, tc.name+": slope", line.slope, tc.want, 1e-9)
	}
}

// A bar is compared with the line at its close time, its open plus the bar's
// length, not at its open: a close that sits on the line where the bar opened is a
// distance from it where the bar closes.
func TestABarIsReadAgainstTheLineAtItsCloseTime(t *testing.T) {
	fills, tape := holding(Window)
	depth := len(fills)
	bars, found := spanOf(tape, fills[0], fills[depth-2], fills[depth-1])
	if !found {
		t.Fatal("the bars of the fills are served")
	}
	newest, _ := newChord(fills[0], fills[depth-1])
	previous, _ := newChord(fills[0], fills[depth-2])
	index := bars.start + 1
	tape.Closes[index] = newest.at(tape.Opens[index])
	got, _, ok := deviations(tape, bars, newest, previous)
	atClose := newest.at(tape.Opens[index] + series.BarMs)
	want := math.Abs(tape.Closes[index]/atClose-1) * 100
	if !ok || want < 0.01 {
		t.Fatalf("the fixture must put a close off the line at its close time, got %v read %v", want, ok)
	}
	assertNear(t, "deviation", got, want, 1e-9)
}

// read is the deviation a bump of pct percent leaves on a chord that reads the bar,
// either side of the line, and none on a chord that does not.
func read(reads bool, pct float64) float64 {
	if reads {
		return math.Abs(pct)
	}
	return 0
}

// Each chord reads the bars from the first fill's bar to its own end fill's bar, both
// inclusive: a bar before the first or after the newest is in neither, and a bar
// after the fill before the newest, up to the newest fill's, is the newest chord's
// alone. A close off the line counts whichever side it lies on.
func TestEachChordReadsItsOwnBarsFromTheWindowsFirstFillsBar(t *testing.T) {
	depth := Window
	before, newest := newestBar(depth-1), newestBar(depth)
	for _, tc := range []struct {
		name             string
		index            int
		newest, previous bool
	}{
		{"the bar before the window's first fill's", leadBars - 1, false, false},
		{"the window's first fill's bar", leadBars, true, true},
		{"a bar both chords run over", leadBars + 1, true, true},
		{"the bar of the fill before the newest", before, true, true},
		{"the bar after it", before + 1, true, false},
		{"the bar of the newest fill", newest, true, false},
		{"the bar after the newest fill's", newest + 1, false, false},
	} {
		for _, pct := range []float64{5, -5} {
			fills, tape := holding(depth)
			bump(tape, tc.index, pct)
			reading, _ := assess(fills, tape)
			assertNear(t, tc.name+": newest chord", reading.Deviation, read(tc.newest, pct), 1e-6)
			assertNear(t, tc.name+": previous chord", reading.PreviousDeviation, read(tc.previous, pct), 1e-6)
		}
	}
}

// bruteDeviation reads a chord's largest deviation the long way, in hours and prices:
// each close from the start fill's bar to the end fill's bar against the straight
// line between the two fills at the bar's close time.
func bruteDeviation(fills []series.Fill, tape series.Series, start, end int) float64 {
	first, last := fills[start], fills[end]
	var worst float64
	for index, open := range tape.Opens {
		if open < series.BarOpen(first.At) || open > series.BarOpen(last.At) {
			continue
		}
		hours := time.UnixMilli(open + series.BarMs).Sub(first.At).Hours()
		line := first.Price + (last.Price-first.Price)*hours/last.At.Sub(first.At).Hours()
		worst = math.Max(worst, math.Abs(tape.Closes[index]/line-1)*100)
	}
	return worst
}

// The deviations of a ladder whose closes wander and whose fill before the newest
// sits off the line are the largest of each chord's own, read the long way. So are
// those of a ladder whose fill before the newest was stamped after the newest, out
// of order: its chord is the longer one and reads the bars the newest chord does not.
func TestDeviationsAreTheLargestDistanceOfACloseFromEachChord(t *testing.T) {
	fills, tape := holding(Window + 1)
	depth := len(fills)
	for index := range tape.Closes {
		tape.Closes[index] *= 1 + MaxDeviationPct/4/100*math.Sin(float64(index))
	}
	fills[depth-2].Price *= 1.01
	reading, _ := assess(fills, tape)
	assertNear(t, "newest chord", reading.Deviation, bruteDeviation(fills, tape, depth-Window, depth-1), 1e-9)
	assertNear(t, "previous chord", reading.PreviousDeviation, bruteDeviation(fills, tape, depth-Window, depth-2), 1e-9)
	if math.Abs(reading.Deviation-reading.PreviousDeviation) < 1e-6 {
		t.Errorf("the two chords must read apart, both read %v", reading.Deviation)
	}

	fills, tape = holding(depth)
	fills[depth-2].At, fills[depth-1].At = fills[depth-1].At, fills[depth-2].At
	bump(tape, newestBar(depth), over)
	reading, _ = assess(fills, tape)
	assertNear(t, "newest chord, out of order", reading.Deviation, bruteDeviation(fills, tape, depth-Window, depth-1), 1e-9)
	assertNear(t, "previous chord, out of order", reading.PreviousDeviation, bruteDeviation(fills, tape, depth-Window, depth-2), 1e-9)
}

// A line that is not above zero has no price to be near: every close is infinitely
// far from it, whether it is a number or not, and a chord steep enough to run
// through zero before the newest fill's bar closes misses on its deviation.
func TestALineAtOrUnderZeroIsInfinitelyFarFromEveryClose(t *testing.T) {
	for _, line := range []float64{0, -5, math.NaN()} {
		if got := deviationOf(100, line); !math.IsInf(got, 1) {
			t.Errorf("a close against a line at %v: deviation %v, want +Inf", line, got)
		}
	}
	depth := Window
	fills, tape := holding(depth)
	repriced(depth-1, 1)(fills, &tape)
	reading, missed := assess(fills, tape)
	if !math.IsInf(reading.Deviation, 1) || missed&missDeviation == 0 {
		t.Errorf("a newest chord through zero must miss on an infinite deviation, got %+v, misses %b", reading, missed)
	}
}
