package swingdecline

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/series"
)

// firstBar is the open time of the first bar of every synthetic series.
var firstBar = time.Date(2021, time.October, 1, 0, 0, 0, 0, time.UTC)

// dashboardRows is how many rows the SMC trend dashboard reads its series.Trend
// Direction row on: the 1h chart row and the two rows above it.
const dashboardRows = 3

// bearishTrend is a series.Trend Direction row that just meets MinSmcTrendDown
// — as many of the dashboard's rows reading down as it asks — and shortTrend one a
// row short of it, so no test that holds a ladder to the trend leans on the
// constant's value. The tests that read a ladder's shape pass bearishTrend to
// Trigger so the shape is all they decide on. allDown meets any need up to the
// dashboard's rows: the tests of the fold that pass their own need use it.
var (
	bearishTrend = series.Trend{Down: MinSmcTrendDown, Valid: dashboardRows}
	shortTrend   = series.Trend{Down: MinSmcTrendDown - 1, Valid: dashboardRows}
	allDown      = series.Trend{Down: dashboardRows, Valid: dashboardRows}
)

// bearishTrendReason is what a Trigger that holds names last when it is given
// bearishTrend, and shortTrendBreak what one the trend fails names after its
// window when it is given shortTrend. With the condition off — the constant zero
// or below — no trend is named.
var (
	bearishTrendReason = trendHoldReason(bearishTrend, MinSmcTrendDown)
	shortTrendBreak    = trendBreakReason(shortTrend, MinSmcTrendDown)
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

// trendAnswer is what the trend does to a ladder whose shape holds, as a function
// of MinSmcTrendDown: whether the ladder still triggers, and the text
// the trend adds — last of the reasons when it does, after the window in the
// breaks when it does not, none when the condition is off.
func trendAnswer(trend series.Trend) (triggers bool, text string) {
	switch {
	case MinSmcTrendDown <= 0:
		return true, ""
	case trend.Valid > 0 && trend.Down >= MinSmcTrendDown:
		return true, trendHoldReason(trend, MinSmcTrendDown)
	}
	return false, trendBreakReason(trend, MinSmcTrendDown)
}

// seriesOfCloses is a served series of these closes with flat wicks — each
// bar's high and low its close — one bar an hour from firstBar.
func seriesOfCloses(closes ...float64) series.Series {
	opens := make([]int64, len(closes))
	for index := range opens {
		opens[index] = barOpenAt(index)
	}
	return series.Series{Opens: opens, Closes: closes, Highs: slices.Clone(closes), Lows: slices.Clone(closes)}
}

// withWicks is the series with every bar's high raised and its low lowered by
// spread, in percent of the bar's close.
func withWicks(tape series.Series, spread float64) series.Series {
	wicked := tape
	wicked.Highs, wicked.Lows = make([]float64, len(tape.Closes)), make([]float64, len(tape.Closes))
	for index, closed := range tape.Closes {
		wicked.Highs[index] = closed * (1 + spread/100)
		wicked.Lows[index] = closed * (1 - spread/100)
	}
	return wicked
}

// cut is the series' bars from index from to index to, exclusive, every array alike.
func cut(tape series.Series, from, to int) series.Series {
	return series.Series{Opens: tape.Opens[from:to], Closes: tape.Closes[from:to], Highs: tape.Highs[from:to], Lows: tape.Lows[from:to]}
}

// barOpenAt is the open time in ms of the synthetic series' bar at index.
func barOpenAt(index int) int64 {
	return firstBar.UnixMilli() + int64(index)*series.BarMs
}

// staircase is a series that moves straight from each turning price to the
// next in barsPerLeg bars: every turning bar is a swing of its own, high or
// low, once barsPerLeg bars have closed after it. The first turn is the close
// of the first bar.
func staircase(barsPerLeg int, turns ...float64) series.Series {
	closes := []float64{turns[0]}
	for index := 1; index < len(turns); index++ {
		from, to := turns[index-1], turns[index]
		for step := 1; step < barsPerLeg; step++ {
			closes = append(closes, from+(to-from)*float64(step)/float64(barsPerLeg))
		}
		closes = append(closes, to)
	}
	return seriesOfCloses(closes...)
}

// fillIn is a fill stamped inside the synthetic series' bar at index, at an
// odd minute so no test leans on a stamp sitting on a bar's open.
func fillIn(index int, price float64) series.Fill {
	return series.Fill{Price: price, At: firstBar.Add(time.Duration(index)*time.Hour + 17*time.Minute + 9*time.Second)}
}

// legBars is how many bars a leg of the staircases takes: enough for every
// turning bar to be a swing once its own bars closed, and for a down leg to
// count under MinBarsBetween.
var legBars = max(2*PivotBars, MinBarsBetween)

// staircaseTurns is a decline of four steps at legBars bars a leg: every down
// leg falls past MinDropPct and every bounce stays under MaxBouncePct as the
// constants ship (eight percent down, two and a bit up), starting a leg below its
// first high so that high is a swing, and ending on a leg still going down:
// bars 0 to 8 × legBars.
var staircaseTurns = []float64{92, 100, 92, 94, 86, 88, 81, 83, 77}

// rallyTurns is a decline whose every bounce rises past MaxBouncePct: its down
// legs count and no up leg does, so no window over it holds, from any fill.
var rallyTurns = []float64{92, 100, 92, 97, 90, 95, 86}

// declineLegs is how many down legs a staircase needs to hold with one to
// spare: one more than the legs down the rule asks, and than the legs up, which
// lie between them.
func declineLegs() int {
	return max(MinLegsDown, MinLegsUp, 1) + 1
}

// declineTurns are the turning prices of a staircase of legs down legs: a first
// close one fall under the first high, the high at 100, then every down leg to a
// low one and a half MinDropPct under the high before it and every bounce half
// MaxBouncePct up to the next, lower, high; so every leg counts whatever the
// constants, and a staircase of more legs runs on from one of fewer.
func declineTurns(legs int) []float64 {
	fall := 1.5 * MinDropPct / 100
	bounce := MaxBouncePct / 2 / 100
	price := 100.0
	turns := []float64{price * (1 - fall), price}
	for leg := 1; leg <= legs; leg++ {
		price *= 1 - fall
		turns = append(turns, price)
		if leg < legs {
			price *= 1 + bounce
			turns = append(turns, price)
		}
	}
	return turns
}

// declineSeries is the series of that staircase at legBars bars a leg, its first
// high on the bar legBars, where the ladders of the tests start, and its last bar
// the low of its last down leg: bar 2 × legs × legBars.
func declineSeries(legs int) series.Series {
	return staircase(legBars, declineTurns(legs)...)
}

// move is the percentage a price makes to another.
func move(from, to float64) float64 {
	return (to/from - 1) * 100
}

// assertNear fails unless got is within tolerance of want.
func assertNear(t *testing.T, name string, got, want, tolerance float64) {
	t.Helper()
	if math.Abs(got-want) > tolerance {
		t.Errorf("%s = %v, want %v within %v", name, got, want, tolerance)
	}
}

// loadTape is the series a testdata tape holds: its open times, highs, lows and
// closes, the format of sophos' smartsell testdata — open time in ms, then
// the high, low, close and volume as the exchange's decimal strings — read as
// the served series. It fails the test on a tape that is not 1h bars, or whose
// bars are not one bar apart.
func loadTape(t *testing.T, name string) series.Series {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read the tape: %v", err)
	}
	var file struct {
		Interval string              `json:"interval"`
		Fields   []string            `json:"fields"`
		Bars     [][]json.RawMessage `json:"bars"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatalf("decode the tape: %v", err)
	}
	if file.Interval != "1h" || !slices.Equal(file.Fields, []string{"openTime", "high", "low", "close", "volume"}) {
		t.Fatalf("the tape holds %s bars of %q, want 1h bars of open time, high, low, close and volume", file.Interval, file.Fields)
	}
	var tape series.Series
	for index, bar := range file.Bars {
		var open int64
		if len(bar) != len(file.Fields) || json.Unmarshal(bar[0], &open) != nil {
			t.Fatalf("bar %d: %s is not an open time and four values", index, bar)
		}
		if index > 0 && open != tape.Opens[index-1]+series.BarMs {
			t.Fatalf("bar %d opens at %d, want one bar after the bar before it", index, open)
		}
		wicks := make([]float64, 3)
		for column := range wicks {
			var text string
			if json.Unmarshal(bar[column+1], &text) != nil {
				t.Fatalf("bar %d: %s is not a decimal string", index, bar[column+1])
			}
			if wicks[column], err = strconv.ParseFloat(text, 64); err != nil {
				t.Fatalf("bar %d: %v", index, err)
			}
		}
		tape.Opens = append(tape.Opens, open)
		tape.Highs = append(tape.Highs, wicks[0])
		tape.Lows = append(tape.Lows, wicks[1])
		tape.Closes = append(tape.Closes, wicks[2])
	}
	return tape
}
