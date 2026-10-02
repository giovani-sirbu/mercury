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
)

// firstBar is the open time of the first bar of every synthetic series.
var firstBar = time.Date(2021, time.October, 1, 0, 0, 0, 0, time.UTC)

// dashboardRows is how many rows the SMC trend dashboard reads its Trend
// Direction row on: the 1h chart row and the two rows above it.
const dashboardRows = 3

// bearishTrend is a Trend Direction row that just meets SwingDeclineMinSmcTrendDown
// — as many of the dashboard's rows reading down as it asks — and shortTrend one a
// row short of it, so no test that holds a ladder to the trend leans on the
// constant's value. The tests that read a ladder's shape pass bearishTrend to
// Trigger so the shape is all they decide on. allDown meets any need up to the
// dashboard's rows: the tests of the fold that pass their own need use it.
var (
	bearishTrend = Trend{Down: SwingDeclineMinSmcTrendDown, Valid: dashboardRows}
	shortTrend   = Trend{Down: SwingDeclineMinSmcTrendDown - 1, Valid: dashboardRows}
	allDown      = Trend{Down: dashboardRows, Valid: dashboardRows}
)

// bearishTrendReason is what a Trigger that holds names last when it is given
// bearishTrend, and shortTrendBreak what one the trend fails names after its
// window when it is given shortTrend. With the condition off — the constant zero
// or below — no trend is named.
var (
	bearishTrendReason = trendHoldReason(bearishTrend, SwingDeclineMinSmcTrendDown)
	shortTrendBreak    = trendBreakReason(shortTrend, SwingDeclineMinSmcTrendDown)
)

// needsTheTrend skips a test that has a trend fail a ladder when the condition is
// off: at SwingDeclineMinSmcTrendDown zero or below no trend is read, so none can
// fail.
func needsTheTrend(t *testing.T) {
	t.Helper()
	if SwingDeclineMinSmcTrendDown <= 0 {
		t.Skip("SwingDeclineMinSmcTrendDown is zero or below: the condition is off and no trend fails a ladder")
	}
}

// trendAnswer is what the trend does to a ladder whose shape holds, as a function
// of SwingDeclineMinSmcTrendDown: whether the ladder still triggers, and the text
// the trend adds — last of the reasons when it does, after the window in the
// breaks when it does not, none when the condition is off.
func trendAnswer(trend Trend) (triggers bool, text string) {
	switch {
	case SwingDeclineMinSmcTrendDown <= 0:
		return true, ""
	case trend.Valid > 0 && trend.Down >= SwingDeclineMinSmcTrendDown:
		return true, trendHoldReason(trend, SwingDeclineMinSmcTrendDown)
	}
	return false, trendBreakReason(trend, SwingDeclineMinSmcTrendDown)
}

// seriesOfCloses is a served series of these closes, one bar an hour from
// firstBar.
func seriesOfCloses(closes ...float64) Series {
	opens := make([]int64, len(closes))
	for index := range opens {
		opens[index] = barOpenAt(index)
	}
	return Series{Opens: opens, Closes: closes}
}

// barOpenAt is the open time in ms of the synthetic series' bar at index.
func barOpenAt(index int) int64 {
	return firstBar.UnixMilli() + int64(index)*SwingDeclineBarMs
}

// staircase is a series that moves straight from each turning price to the
// next in barsPerLeg bars: every turning bar is a swing of its own, high or
// low, once barsPerLeg bars have closed after it. The first turn is the close
// of the first bar.
func staircase(barsPerLeg int, turns ...float64) Series {
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
func fillIn(index int, price float64) Fill {
	return Fill{Price: price, At: firstBar.Add(time.Duration(index)*time.Hour + 17*time.Minute + 9*time.Second)}
}

// move is the percentage a close makes from one price to another.
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

// loadTape is the series a testdata tape holds: its open times and closes, the
// format of sophos' smarttakeloss testdata — open time in ms, then the high,
// low, close and volume as the exchange's decimal strings — read as the
// served series. It fails the test on a tape that is not 1h bars, or whose
// bars are not one bar apart.
func loadTape(t *testing.T, name string) Series {
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
	var series Series
	for index, bar := range file.Bars {
		var open int64
		var text string
		if len(bar) != len(file.Fields) || json.Unmarshal(bar[0], &open) != nil || json.Unmarshal(bar[3], &text) != nil {
			t.Fatalf("bar %d: %s is not an open time and four values", index, bar)
		}
		if index > 0 && open != series.Opens[index-1]+SwingDeclineBarMs {
			t.Fatalf("bar %d opens at %d, want one bar after the bar before it", index, open)
		}
		closed, err := strconv.ParseFloat(text, 64)
		if err != nil {
			t.Fatalf("bar %d: %v", index, err)
		}
		series.Opens = append(series.Opens, open)
		series.Closes = append(series.Closes, closed)
	}
	return series
}
