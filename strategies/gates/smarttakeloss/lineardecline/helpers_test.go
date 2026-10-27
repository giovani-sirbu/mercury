package lineardecline

import (
	"fmt"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/series"
	"math"
	"slices"
	"testing"
	"time"
)

// firstBar is the open time of the first bar of every synthetic series.
var firstBar = time.Date(2021, time.October, 1, 0, 0, 0, 0, time.UTC)

// dashboardRows is how many rows the SMC trend dashboard reads its Trend Direction
// row on: the 1h chart row and the two rows above it.
const dashboardRows = 3

// bearishTrend is a Trend Direction row that just meets MinSmcTrendDown
// — as many of the dashboard's rows reading down as it asks — and shortTrend one a
// row short of it, so no test that holds a ladder to the trend leans on the
// constant's value. The tests of the line pass bearishTrend to Trigger so the line
// is all they decide on.
var (
	bearishTrend = series.Trend{Down: MinSmcTrendDown, Valid: max(dashboardRows, MinSmcTrendDown)}
	shortTrend   = series.Trend{Down: MinSmcTrendDown - 1, Valid: max(dashboardRows, MinSmcTrendDown)}
)

// needsTheTrend skips a test that has a trend fail a ladder when the condition is
// off: at MinSmcTrendDown zero or below no trend is read, so none can
// fail.
func needsTheTrend(t *testing.T) {
	t.Helper()
	if MinSmcTrendDown <= 0 {
		t.Skip("MinSmcTrendDown is zero or below: the condition is off and no trend fails a ladder")
	}
}

// needsAFall skips a test that has a ladder miss on its net fall when the rule asks
// for none: at MinFallPct at or over zero every falling ladder makes it.
func needsAFall(t *testing.T) {
	t.Helper()
	if MinFallPct >= 0 {
		t.Skip("MinFallPct is zero or over: a falling ladder never falls short of it")
	}
}

const (
	// leadBars is how many bars the line ladder's series holds before the bar of its
	// first fill; fillGap how many bars lie between the bars of two successive fills.
	leadBars = 2
	fillGap  = 6
	// fillOffset is how far into its bar each fill is stamped: an odd minute and
	// second, so no test leans on a stamp sitting on a bar's open.
	fillOffset = 17*time.Minute + 9*time.Second
	// firstPrice is the price of the line ladder's first fill.
	firstPrice = 100.0
)

// fall is the net fall, in percent of the window's first fill's price, the line ladder
// makes across the window, from the window's first fill to the newest: half as much
// again as the least the rule asks (MinFallPct), and over one percent so that a retune
// toward zero still leaves a fall to read. over is a distance, in percent, off the line
// that is over the bound of every deviation, however the rule is calibrated. lineSlope
// is the slope, in percent per hour, of the line that falls by fall across the window's
// fills, fillGap bars apart.
var (
	fall      = 1.5 * math.Max(-MinFallPct, 1)
	over      = 1.5 * math.Max(MaxDeviationPct, 1)
	lineSlope = -fall / float64((Window-1)*fillGap)
)

// slopeFor is the slope, in percent of the ladder's first fill's price per hour, of the line
// through it whose window falls by fall at depth fills: the window's first fill sits lower
// than the ladder's, so in percent of the ladder's first fill the line falls slower.
func slopeFor(depth int) float64 {
	return -fall / (float64(fillGap) * (float64(Window-1) + fall*float64(depth-Window)/100))
}

// linePrice is the price of the line of that slope hours after the first fill's stamp.
func linePrice(slope, hours float64) float64 {
	return firstPrice * (1 + slope/100*hours)
}

// barOpenAt is the open time in ms of the synthetic series' bar at index.
func barOpenAt(index int) int64 {
	return firstBar.UnixMilli() + int64(index)*series.BarMs
}

// fillIn is a fill stamped inside the synthetic series' bar at index.
func fillIn(index int, price float64) series.Fill {
	return series.Fill{Price: price, At: firstBar.Add(time.Duration(index)*time.Hour + fillOffset)}
}

// newestBar is the position of the bar of the newest fill of a line ladder of depth fills.
func newestBar(depth int) int {
	return leadBars + (depth-1)*fillGap
}

// lineLadder is a ladder of depth fills on the line of that slope, and the served
// bars of that line: every close is the line's price at the bar's close time, from
// leadBars bars before the first fill's bar to the bar after the newest fill's.
func lineLadder(depth int, slope float64) ([]series.Fill, series.Series) {
	fills := make([]series.Fill, depth)
	for index := range fills {
		fills[index] = fillIn(leadBars+index*fillGap, linePrice(slope, float64(index*fillGap)))
	}
	bars := newestBar(depth) + 2
	tape := series.Series{Opens: make([]int64, bars), Closes: make([]float64, bars)}
	for index := range bars {
		tape.Opens[index] = barOpenAt(index)
		tape.Closes[index] = linePrice(slope, float64(index+1-leadBars)-fillOffset.Hours())
	}
	return fills, tape
}

// holding is the line ladder of depth fills that falls by fall across its window:
// under bearishTrend it holds, with every deviation at the rounding of the sums.
func holding(depth int) ([]series.Fill, series.Series) {
	return lineLadder(depth, slopeFor(depth))
}

// bump moves the close at index off the line by pct percent, either side.
func bump(tape series.Series, index int, pct float64) {
	tape.Closes[index] *= 1 + pct/100
}

// frameOf is the frame the reasons of the line ladder of depth fills open with,
// written out against the grammar: the bars of the window's first fill and of the newest.
func frameOf(depth int) string {
	stamp := func(index int) string {
		return firstBar.Add(time.Duration(index) * time.Hour).Format("2006-01-02 15:04")
	}
	from := depth - Window + 1
	return fmt.Sprintf("depth %d to %d, 1h bars %s to %s UTC", from, depth, stamp(newestBar(from)), stamp(newestBar(depth)))
}

// deviationText, risingText and trendText are the reasons a deviation over its
// bound, a chord that does not fall and a trend a row short of its need name,
// written out against the grammar.
func deviationText(deviation float64, depth int) string {
	return fmt.Sprintf("closes %.1f%% from the line at depth %d over %.1f%%", deviation, depth, MaxDeviationPct)
}

func risingText(slope float64, depth int) string {
	return fmt.Sprintf("slope %.2f%%/h rising at depth %d", slope, depth)
}

func trendText(trend series.Trend) string {
	return fmt.Sprintf("smc trend direction down %d/%d short of %d needed", trend.Down, trend.Valid, MinSmcTrendDown)
}

// trendReasons is what the trend adds last to a holding line's reasons: the rows
// down out of the rows read against the need, and nothing with the condition off.
func trendReasons(trend series.Trend) []string {
	if MinSmcTrendDown <= 0 {
		return nil
	}
	return []string{fmt.Sprintf("smc trend direction down %d/%d of %d needed", trend.Down, trend.Valid, MinSmcTrendDown)}
}

// mustMiss fails the test unless the ladder misses under bearishTrend with no text
// formatted, and answers the reading that decided.
func mustMiss(t *testing.T, fills []series.Fill, tape series.Series) Reading {
	t.Helper()
	reading, holds := Trigger(fills, tape, bearishTrend)
	if holds || reading.Holds || reading.Reasons != nil {
		t.Fatalf("the ladder must miss and format nothing, got holds %v, %+v", holds, reading)
	}
	return reading
}

// assertBreaks fails unless Breaks names exactly want.
func assertBreaks(t *testing.T, fills []series.Fill, tape series.Series, trend series.Trend, want ...string) {
	t.Helper()
	if got := Breaks(fills, tape, trend); !slices.Equal(got, want) {
		t.Errorf("breaks\n got %q\nwant %q", got, want)
	}
}

// readingOf is a reading of those measured numbers, and past the first number over
// a bound.
func readingOf(deviation, previousDeviation, slope, previousSlope, netPct float64) Reading {
	return Reading{
		Deviation:         deviation,
		PreviousDeviation: previousDeviation,
		Slope:             slope,
		PreviousSlope:     previousSlope,
		NetPct:            netPct,
	}
}

func past(bound float64) float64 {
	return math.Nextafter(bound, math.Inf(1))
}

// assertNear fails unless got is within tolerance of want.
func assertNear(t *testing.T, name string, got, want, tolerance float64) {
	t.Helper()
	if math.Abs(got-want) > tolerance {
		t.Errorf("%s = %v, want %v within %v", name, got, want, tolerance)
	}
}
