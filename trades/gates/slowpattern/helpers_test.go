package slowpattern

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
	return firstBar.UnixMilli() + int64(index)*SlowPatternBarMs
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
		if index > 0 && open != series.Opens[index-1]+SlowPatternBarMs {
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
