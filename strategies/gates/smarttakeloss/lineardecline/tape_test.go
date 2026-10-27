package lineardecline

import (
	"encoding/json"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/series"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
	"time"
)

// loadTape is the series a testdata tape holds: its open times and closes, in the
// format of swingdecline's own testdata — open time in ms, then the high, low, close
// and volume as the exchange's decimal strings — read as the served series. It fails
// the test on a tape that holds no bar, is not 1h bars, or whose bars are not one
// bar apart.
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
		var text string
		if len(bar) != len(file.Fields) || json.Unmarshal(bar[0], &open) != nil || json.Unmarshal(bar[3], &text) != nil {
			t.Fatalf("bar %d: %s is not an open time and four values", index, bar)
		}
		if index > 0 && open != tape.Opens[index-1]+series.BarMs {
			t.Fatalf("bar %d opens at %d, want one bar after the bar before it", index, open)
		}
		closed, err := strconv.ParseFloat(text, 64)
		if err != nil {
			t.Fatalf("bar %d: %v", index, err)
		}
		tape.Opens = append(tape.Opens, open)
		tape.Closes = append(tape.Closes, closed)
	}
	if !tape.Served() {
		t.Fatal("the tape holds no bar")
	}
	return tape
}

// stampLayout is how the stamps of the ladders below are written: UTC, to the second.
const stampLayout = "2006-01-02 15:04:05"

// fillRow is a fill as its history carries it: the stamp and the price paid.
type fillRow struct {
	stamp string
	price float64
}

// tapeLadder is a ladder that fell through the bars of a tape in testdata: its fills
// in the order it placed them, and the depths its line holds at under a trend that
// meets its need — the first, and the deeper ones where it holds again, nil for a
// ladder whose window never sits on one line — as read at the constants measuredAt
// names.
type tapeLadder struct {
	tape  string
	fills []fillRow
	holds []int
}

// tapeLadders are three ladders cut out of the market's own 1h bars, each tape
// running from some bars before its first fill to a few bars past its last.
var tapeLadders = []tapeLadder{
	{
		tape: "solusdt-1h-2022-01.json",
		fills: []fillRow{
			{"2022-01-20 15:01:27", 141.99},
			{"2022-01-20 21:42:04", 134.41},
			{"2022-01-20 23:42:26", 129.02},
			{"2022-01-21 08:44:02", 125.72},
			{"2022-01-21 12:11:29", 119.71},
			{"2022-01-21 21:33:17", 115.98},
			{"2022-01-22 05:42:41", 106.31},
			{"2022-01-23 08:59:55", 101.42},
			{"2022-01-23 14:47:20", 97.47},
			{"2022-01-24 00:20:56", 95.61},
		},
		holds: []int{6, 7},
	},
	{
		tape: "solusdt-1h-2022-06.json",
		fills: []fillRow{
			{"2022-06-09 12:22:37", 40.56},
			{"2022-06-09 20:02:34", 39.61},
			{"2022-06-10 12:32:16", 38.68},
			{"2022-06-10 16:28:47", 37.13},
			{"2022-06-11 11:02:44", 36.26},
			{"2022-06-11 13:06:14", 34.80},
			{"2022-06-11 23:45:12", 33.71},
			{"2022-06-12 18:46:10", 32.92},
			{"2022-06-12 22:44:01", 31.60},
			{"2022-06-13 00:59:32", 29.70},
		},
		holds: []int{7},
	},
	{
		tape: "btcusdt-1h-2022-06.json",
		fills: []fillRow{
			{"2022-06-15 22:27:27", 22715.06},
			{"2022-06-16 07:02:07", 21875.71},
			{"2022-06-16 10:41:05", 20919.26},
			{"2022-06-16 23:16:24", 20307.06},
			{"2022-06-18 06:51:52", 19823.53},
			{"2022-06-18 07:23:16", 19109.16},
			{"2022-06-18 17:41:43", 18699.01},
			{"2022-06-18 20:12:35", 17738.22},
		},
		holds: nil,
	},
}

// read is the ladder's fills and the series of its tape. It fails the test on a stamp
// that is not one, or on a tape that does not hold the bars from the first fill's to
// the last fill's: a ladder read on bars it fell through only partly would break on
// the bars, not on its line.
func (ladder tapeLadder) read(t *testing.T) ([]series.Fill, series.Series) {
	t.Helper()
	tape := loadTape(t, ladder.tape)
	fills := make([]series.Fill, len(ladder.fills))
	for index, row := range ladder.fills {
		stamp, err := time.Parse(stampLayout, row.stamp)
		if err != nil {
			t.Fatalf("%s: fill %d: %v", ladder.tape, index+1, err)
		}
		fills[index] = series.Fill{Price: row.price, At: stamp}
	}
	first, last := series.BarOpen(fills[0].At), series.BarOpen(fills[len(fills)-1].At)
	if tape.Opens[0] > first || tape.LastBarOpen() < last {
		t.Fatalf("%s holds the bars %d to %d, the ladder fell through %d to %d", ladder.tape, tape.Opens[0], tape.LastBarOpen(), first, last)
	}
	return fills, tape
}
