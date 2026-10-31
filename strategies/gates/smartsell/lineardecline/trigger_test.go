package lineardecline

import (
	"fmt"
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/series"
	"math"
	"reflect"
	"slices"
	"testing"
	"time"
)

// A ladder whose fills sit on one line holds under a trend that meets the need, at
// every depth from the least the rule reads. The reading carries the numbers it
// read and names the line by its depths and bars, the slope of each chord, how far
// the closes lie from the line, the fall and the trend.
func TestTriggerHoldsWhenTheFillsSitOnOneLine(t *testing.T) {
	for depth := Window; depth <= Window+3; depth++ {
		fills, tape := holding(depth)
		reading, holds := Trigger(fills, tape, bearishTrend)
		if !holds || !reading.Holds {
			t.Fatalf("depth %d: a ladder on one line must hold, got breaks %q", depth, Breaks(fills, tape, bearishTrend))
		}
		from := depth - Window + 1
		runsWindowToNewest := reading.FromDepth == from && reading.ToDepth == depth &&
			reading.StartAt == barOpenAt(newestBar(from)) && reading.EndAt == barOpenAt(newestBar(depth))
		if !runsWindowToNewest {
			t.Errorf("depth %d: the line runs from the window's first fill, depth %d, to the newest, got %+v", depth, from, reading)
		}
		assertNear(t, "slope", reading.Slope, lineSlope, 1e-9)
		assertNear(t, "previous slope", reading.PreviousSlope, lineSlope, 1e-9)
		assertNear(t, "deviation", reading.Deviation, 0, 1e-9)
		assertNear(t, "previous deviation", reading.PreviousDeviation, 0, 1e-9)
		assertNear(t, "net move", reading.NetPct, -fall, 1e-9)
		want := append([]string{
			frameOf(depth),
			fmt.Sprintf("slope %.2f%%/h against %.2f%%/h at depth %d, within %.0f%%",
				reading.Slope, reading.PreviousSlope, depth-1, MaxSlopeChange*100),
			fmt.Sprintf("closes within %.1f%% of the line, at most %.1f%%",
				math.Max(reading.Deviation, reading.PreviousDeviation), MaxDeviationPct),
			fmt.Sprintf("down %.1f%% past %.1f%%", -reading.NetPct, -MinFallPct),
		}, trendReasons(bearishTrend)...)
		if !slices.Equal(reading.Reasons, want) {
			t.Errorf("depth %d: reasons\n got %q\nwant %q", depth, reading.Reasons, want)
		}
		assertBreaks(t, fills, tape, bearishTrend)
	}
}

// The hold names the larger of the two chords' deviations: a fill before the newest
// a little off the line makes the chord to it the worse fit, while the chord to the
// newest fill, through the first fill, fits the closes, and the line still holds.
func TestTheHoldNamesTheLargerOfTheTwoDeviations(t *testing.T) {
	depth := Window
	off := 0.25 * math.Min(MaxSlopeChange*fall, MaxDeviationPct)
	if off < 0.01 {
		t.Skip("MaxSlopeChange or MaxDeviationPct is too small to put a fill off the line and hold")
	}
	fills, tape := holding(depth)
	fills[depth-2].Price *= 1 + off/100
	reading, holds := Trigger(fills, tape, bearishTrend)
	if !holds || reading.PreviousDeviation <= reading.Deviation {
		t.Fatalf("the previous chord must fit worse and the line hold, got holds %v, %+v", holds, reading)
	}
	want := fmt.Sprintf("closes within %.1f%% of the line, at most %.1f%%", reading.PreviousDeviation, MaxDeviationPct)
	if reading.Reasons[2] != want {
		t.Errorf("the closes reason is %q, want %q", reading.Reasons[2], want)
	}
}

// A miss formats nothing and allocates nothing — the cost of a miss is the pass over
// the bars alone — whichever way it misses: a trend short of the need, a ladder too
// shallow, a chord that cannot be read, a line that does not fall.
func TestAMissFormatsNothingAndAllocatesNothing(t *testing.T) {
	needsTheTrend(t)
	needsAFall(t)
	depth := Window
	fills, tape := holding(depth)
	unread := slices.Clone(fills)
	unread[0].At = time.Time{}
	risingFills, risingSeries := lineLadder(depth, -lineSlope)
	for _, tc := range []struct {
		name  string
		fills []series.Fill
		tape  series.Series
		trend series.Trend
	}{
		{"a trend short of the need", fills, tape, shortTrend},
		{"a ladder too shallow", fills[:depth-1], tape, bearishTrend},
		{"a chord that cannot be read", unread, tape, bearishTrend},
		{"a line that does not fall", risingFills, risingSeries, bearishTrend},
	} {
		if reading, holds := Trigger(tc.fills, tc.tape, tc.trend); holds || reading.Reasons != nil {
			t.Fatalf("%s: want a miss with no text, got holds %v, %+v", tc.name, holds, reading)
		}
		if allocs := testing.AllocsPerRun(100, func() { Trigger(tc.fills, tc.tape, tc.trend) }); allocs != 0 {
			t.Errorf("%s: a miss allocated %v times, want none", tc.name, allocs)
		}
	}
}

// The reading is a function of the window's first fill, the fill before the newest and the
// newest, and of the bars from the window's first fill's own to the newest fill's own: the bars
// after it, however absurd, the fills and bars before the window and the fills between change
// nothing.
func TestTheReadingIsTheSameOnEveryLaterTickAndReadsTheWindowAlone(t *testing.T) {
	depth := Window + 2
	fills, tape := holding(depth)
	want, _ := Trigger(fills, tape, bearishTrend)
	grown := series.Series{Opens: slices.Clone(tape.Opens), Closes: slices.Clone(tape.Closes)}
	for extra := range 5 {
		grown.Opens = append(grown.Opens, barOpenAt(len(tape.Opens)+extra))
		grown.Closes = append(grown.Closes, 1e9)
	}
	if got, _ := Trigger(fills, grown, bearishTrend); !reflect.DeepEqual(got, want) {
		t.Errorf("later bars changed the answer:\nbefore %+v\nafter  %+v", want, got)
	}
	before := slices.Clone(fills)
	earlier := series.Series{Opens: slices.Clone(tape.Opens), Closes: slices.Clone(tape.Closes)}
	for index := range depth - Window {
		before[index].Price, before[index].At = 0, time.Time{}
	}
	for index := range newestBar(depth - Window + 1) {
		earlier.Closes[index] = 1e9
	}
	if got, _ := Trigger(before, earlier, bearishTrend); !reflect.DeepEqual(got, want) {
		t.Errorf("the fills and bars before the window changed the answer:\nbefore %+v\nafter  %+v", want, got)
	}
	if Window > previousChordFills {
		between := slices.Clone(fills)
		between[depth-Window+1].Price, between[depth-Window+1].At = 0, time.Time{}
		if got, _ := Trigger(between, tape, bearishTrend); !reflect.DeepEqual(got, want) {
			t.Errorf("a fill between the window's first and the one before the newest changed the answer:\nbefore %+v\nafter  %+v", want, got)
		}
	}
}

// A ladder whose fills before the window sit off every line, flat at the first price with their
// bars' closes, holds once its newest Window fills sit on one line, read from the window's first
// fill; a close off the line inside the window still misses.
func TestALadderHoldsOnItsNewestWindowFillsWhateverItsStartDid(t *testing.T) {
	depth := Window + 2
	fills, tape := holding(depth)
	from := depth - Window + 1
	for index := range depth - Window {
		fills[index].Price = firstPrice
	}
	for index := range newestBar(from) {
		tape.Closes[index] = firstPrice
	}
	reading, holds := Trigger(fills, tape, bearishTrend)
	if !holds || reading.FromDepth != from || reading.StartAt != barOpenAt(newestBar(from)) {
		t.Fatalf("the newest %d fills on one line hold from depth %d, got holds %v, %+v, breaks %q",
			Window, from, holds, reading, Breaks(fills, tape, bearishTrend))
	}
	assertNear(t, "net move", reading.NetPct, -fall, 1e-9)
	bump(tape, newestBar(from), over)
	if _, holds := Trigger(fills, tape, bearishTrend); holds {
		t.Error("a close off the line inside the window must miss")
	}
}

// ask is one thing the rule is asked: a ladder, the bars and the trend of the tick.
type ask struct {
	name  string
	fills []series.Fill
	tape  series.Series
	trend series.Trend
}

// asksAbout are asks the rule answers every way: a line that holds, at the least
// depth and deeper, and each way it misses.
func asksAbout() []ask {
	fills, tape := holding(Window)
	deeper, deeperSeries := holding(Window + 3)
	off, offSeries := holding(Window)
	bump(offSeries, newestBar(Window), over)
	rising, risingSeries := lineLadder(Window, -lineSlope)
	unread := slices.Clone(fills)
	unread[0].At = time.Time{}
	return []ask{
		{"a line that holds", fills, tape, bearishTrend},
		{"a deeper line that holds", deeper, deeperSeries, bearishTrend},
		{"a trend short of the need", fills, tape, shortTrend},
		{"a trend not read", fills, tape, series.Trend{}},
		{"a ladder too shallow", fills[:Window-1], tape, bearishTrend},
		{"a ladder with no fill", nil, tape, bearishTrend},
		{"a chord that cannot be read", unread, tape, bearishTrend},
		{"a series that is not served", fills, series.Series{}, bearishTrend},
		{"a line that rises", rising, risingSeries, bearishTrend},
		{"a close off the line", off, offSeries, bearishTrend},
	}
}

// triggerAtOf asks TriggerAt of a ladder as a caller that keeps no slice does: the three
// fills it reads and the depth, zero fills for a ladder shallower than the window.
func triggerAtOf(fills []series.Fill, tape series.Series, trend series.Trend) (Reading, bool) {
	var first, previous, newest series.Fill
	depth := len(fills)
	if depth >= Window {
		first, previous, newest = fills[depth-Window], fills[depth-2], fills[depth-1]
	}
	return TriggerAt(first, previous, newest, depth, tape, trend)
}

// TriggerAt answers exactly what Trigger answers for the ladder its three fills are
// of — a hold with its reasons, every miss with the readings it carries — and a miss
// allocates nothing: the fills are passed by value and the call formats no text.
func TestTriggerAtAnswersAsTriggerAndAllocatesNothingOnAMiss(t *testing.T) {
	asks := asksAbout()
	holds := 0
	for _, one := range asks {
		want, wantHolds := Trigger(one.fills, one.tape, one.trend)
		got, gotHolds := triggerAtOf(one.fills, one.tape, one.trend)
		if gotHolds != wantHolds || !reflect.DeepEqual(got, want) {
			t.Errorf("%s: TriggerAt\n got %v %+v\nwant %v %+v", one.name, gotHolds, got, wantHolds, want)
		}
		if wantHolds {
			holds++
		} else if allocs := testing.AllocsPerRun(100, func() { triggerAtOf(one.fills, one.tape, one.trend) }); allocs != 0 {
			t.Errorf("%s: a miss allocated %v times, want none", one.name, allocs)
		}
	}
	if holds == 0 || holds == len(asks) {
		t.Errorf("the asks must hold and miss, got %d of %d holding", holds, len(asks))
	}
}

// A depth under the least the rule reads is read from none of the fills it is given,
// whatever they hold: fills that would hold at the least depth name the depth alone.
func TestTriggerAtReadsNoFillOfALadderShallowerThanTheRuleReads(t *testing.T) {
	fills, tape := holding(Window)
	for depth := range Window {
		reading, holds := TriggerAt(fills[0], fills[1], fills[2], depth, tape, bearishTrend)
		if holds || !reflect.DeepEqual(reading, Reading{ToDepth: depth}) {
			t.Errorf("depth %d: want nothing read, got holds %v, %+v", depth, holds, reading)
		}
	}
}
