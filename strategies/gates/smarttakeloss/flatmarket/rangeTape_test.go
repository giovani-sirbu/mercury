package flatmarket

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/series"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// stampLayout is how the tests write a bar's open time: UTC, to the minute, the
// layout the frame of a reading prints.
const stampLayout = "2006-01-02 15:04"

// tapeFields are the fields of a bar of a testdata tape, in the order the tape lists
// them: the open time in ms, then the open, high, low and close as the exchange's
// decimal strings. A tape is the exchange's 1h klines cut to them, oldest first.
var tapeFields = []string{"openTime", "open", "high", "low", "close"}

// readTape is the bars a tape holds, as the five arrays the gate reads (Bars). It
// refuses what is not JSON, is not 1h bars, lists other fields than tapeFields, holds
// no bar or a bar that is not an open time and four decimal strings, holds bars that
// are not one bar (series.BarMs) apart, or holds bars that are not
// served (Bars.Served).
func readTape(raw []byte) (Bars, error) {
	var file struct {
		Interval string              `json:"interval"`
		Fields   []string            `json:"fields"`
		Bars     [][]json.RawMessage `json:"bars"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		return Bars{}, fmt.Errorf("decode the tape: %w", err)
	}
	if file.Interval != "1h" || !slices.Equal(file.Fields, tapeFields) {
		return Bars{}, fmt.Errorf("the tape holds %s bars of %q, want 1h bars of %q", file.Interval, file.Fields, tapeFields)
	}
	if len(file.Bars) == 0 {
		return Bars{}, errors.New("the tape holds no bar")
	}
	var bars Bars
	for index, cells := range file.Bars {
		open, prices, err := readTapeBar(cells)
		if err != nil {
			return Bars{}, fmt.Errorf("bar %d: %w", index, err)
		}
		if index > 0 && open != bars.Opens[index-1]+series.BarMs {
			return Bars{}, fmt.Errorf("bar %d opens at %d, want one bar after the bar before it", index, open)
		}
		bars.Opens = append(bars.Opens, open)
		bars.OpenPrices = append(bars.OpenPrices, prices[0])
		bars.Highs = append(bars.Highs, prices[1])
		bars.Lows = append(bars.Lows, prices[2])
		bars.Closes = append(bars.Closes, prices[3])
	}
	if !bars.Served() {
		return Bars{}, errors.New("the tape's bars are not served: too few of them, or a price at or under zero")
	}
	return bars, nil
}

// readTapeBar is one bar of a tape: its open time and its open, high, low and close.
func readTapeBar(cells []json.RawMessage) (open int64, prices [4]float64, err error) {
	if len(cells) != len(tapeFields) {
		return 0, prices, fmt.Errorf("%d values, want %d", len(cells), len(tapeFields))
	}
	if err := json.Unmarshal(cells[0], &open); err != nil {
		return 0, prices, fmt.Errorf("open time: %w", err)
	}
	for slot := range prices {
		var text string
		if err := json.Unmarshal(cells[slot+1], &text); err != nil {
			return 0, prices, fmt.Errorf("%s: %w", tapeFields[slot+1], err)
		}
		if prices[slot], err = strconv.ParseFloat(text, 64); err != nil {
			return 0, prices, fmt.Errorf("%s: %w", tapeFields[slot+1], err)
		}
	}
	return open, prices, nil
}

// loadTape is the bars of a tape in testdata, by file name: each 1h bar's five fields
// read as the arrays the gate reads. It fails the test on a tape readTape refuses.
func loadTape(t *testing.T, name string) Bars {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read the tape: %v", err)
	}
	bars, err := readTape(raw)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return bars
}

// cutWindow is the size bars of the tape that end at the bar opening at lastOpen: the
// bars the gate reads when sophos serves its look-back as of that bar. The open time
// is matched exactly, by binary search over the tape's open times, which it keeps
// sorted. The five arrays are cut alike and clipped, so that nothing appended to a
// window reaches the tape. It refuses a window it cannot cut: no bar opens at
// lastOpen, no bar is asked, or fewer than size bars end there.
func cutWindow(bars Bars, lastOpen int64, size int) (Bars, error) {
	end, found := slices.BinarySearch(bars.Opens, lastOpen)
	if !found {
		return Bars{}, fmt.Errorf("no bar opens at %s", time.UnixMilli(lastOpen).UTC().Format(stampLayout))
	}
	from, to := end+1-size, end+1
	if size < 1 || from < 0 {
		return Bars{}, fmt.Errorf("%d bars end at the bar opening at %d, want a window of %d", to, lastOpen, size)
	}
	return Bars{
		Opens:      slices.Clip(bars.Opens[from:to]),
		OpenPrices: slices.Clip(bars.OpenPrices[from:to]),
		Highs:      slices.Clip(bars.Highs[from:to]),
		Lows:       slices.Clip(bars.Lows[from:to]),
		Closes:     slices.Clip(bars.Closes[from:to]),
	}, nil
}

// windowEnding is the size bars of the tape ending at the bar that opens at lastOpen
// (ms), and fails the test on a window cutWindow refuses.
func windowEnding(t *testing.T, bars Bars, lastOpen int64, size int) Bars {
	t.Helper()
	window, err := cutWindow(bars, lastOpen, size)
	if err != nil {
		t.Fatalf("cut the window: %v", err)
	}
	return window
}

// fiveFields is tapeFields as a tape file writes them.
const fiveFields = `["openTime","open","high","low","close"]`

// tapeBar is one bar of a synthetic tape as the tape file writes it, index bars after
// the first: the open time in ms, then the open, high, low and close as decimal strings.
func tapeBar(index int, open, high, low, closed string) string {
	return fmt.Sprintf("[%d,%q,%q,%q,%q]", int64(index)*series.BarMs, open, high, low, closed)
}

// tapeOf is a synthetic tape of that interval and fields (JSON text) holding those bars.
func tapeOf(interval, fields string, bars ...string) []byte {
	return fmt.Appendf(nil, `{"symbol":"TEST","interval":%q,"fields":%s,"bars":[%s]}`, interval, fields, strings.Join(bars, ","))
}

// rampTape is a tape of n bars whose every array holds a number of its own at every
// index, so that a window cut from the wrong place in any one of them shows.
func rampTape(n int) Bars {
	var bars Bars
	for index := range n {
		bars.Opens = append(bars.Opens, int64(index)*series.BarMs)
		bars.OpenPrices = append(bars.OpenPrices, float64(100+index))
		bars.Highs = append(bars.Highs, float64(200+index))
		bars.Lows = append(bars.Lows, float64(50+index))
		bars.Closes = append(bars.Closes, float64(150+index))
	}
	return bars
}

// A tape is read in the order it lists its fields: the open, the high, the low and the
// close of each bar land in their own array, the open times in Opens, oldest first.
func TestTheLoaderReadsTheFiveFieldsInTheOrderTheTapeListsThem(t *testing.T) {
	want := Bars{
		Opens:      []int64{0, series.BarMs, 2 * series.BarMs},
		OpenPrices: []float64{10, 11.5, 12.25},
		Highs:      []float64{12, 13, 14},
		Lows:       []float64{9, 10, 11},
		Closes:     []float64{11, 12.25, 13},
	}
	got, err := readTape(tapeOf("1h", fiveFields,
		tapeBar(0, "10", "12", "9", "11"),
		tapeBar(1, "11.5", "13", "10", "12.25"),
		tapeBar(2, "12.25", "14", "11", "13"),
	))
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("read %+v, %v\nwant %+v", got, err, want)
	}
}

// The loader fails a test on a tape that holds no bar, is not 1h bars, lists other
// fields, holds bars that are not one bar apart, or holds a bar it cannot read, and it
// says which: each refusal names its own cause.
func TestTheLoaderRefusesATapeThatIsNotOneHourBarsOfTheFiveFieldsOneBarApart(t *testing.T) {
	first, second := tapeBar(0, "10", "12", "9", "11"), tapeBar(1, "11", "13", "10", "12")
	for _, tc := range []struct {
		name, cause string
		raw         []byte
	}{
		{"not json", "decode the tape", []byte(`{"interval":`)},
		{"no bar", "no bar", tapeOf("1h", fiveFields)},
		{"another interval", "want 1h bars", tapeOf("4h", fiveFields, first, second)},
		{"the fields of the other tapes", "want 1h bars", tapeOf("1h", `["openTime","high","low","close","volume"]`, first, second)},
		{"one field more", "want 1h bars", tapeOf("1h", `["openTime","open","high","low","close","volume"]`, first, second)},
		{"the fields in another order", "want 1h bars", tapeOf("1h", `["openTime","open","low","high","close"]`, first, second)},
		{"two bars apart", "one bar after", tapeOf("1h", fiveFields, first, tapeBar(2, "11", "13", "10", "12"))},
		{"oldest last", "one bar after", tapeOf("1h", fiveFields, second, first)},
		{"a bar of four values", "4 values", tapeOf("1h", fiveFields, first, `[3600000,"11","13","10"]`)},
		{"an open time that is a string", "open time", tapeOf("1h", fiveFields, first, `["3600000","11","13","10","12"]`)},
		{"a price that is a number", "open:", tapeOf("1h", fiveFields, first, `[3600000,11,"13","10","12"]`)},
		{"a price that is not a number", "eleven", tapeOf("1h", fiveFields, first, `[3600000,"eleven","13","10","12"]`)},
		{"a price of zero", "not served", tapeOf("1h", fiveFields, first, tapeBar(1, "11", "13", "0", "12"))},
	} {
		if _, err := readTape(tc.raw); err == nil || !strings.Contains(err.Error(), tc.cause) {
			t.Errorf("%s: error %v, want the tape refused with %q", tc.name, err, tc.cause)
		}
	}
}

// A window is the size bars up to and including the one opening at the time asked, the
// five arrays cut alike and clipped; the time is matched exactly, and a window that
// cannot be cut is refused.
func TestWindowEndingCutsTheBarsUpToTheOpenAndRefusesWhatItCannotCut(t *testing.T) {
	tape := rampTape(40)
	const end, size = 30, 24
	got, err := cutWindow(tape, tape.Opens[end], size)
	if err != nil {
		t.Fatal(err)
	}
	from, to := end+1-size, end+1
	want := Bars{
		Opens: tape.Opens[from:to], OpenPrices: tape.OpenPrices[from:to],
		Highs: tape.Highs[from:to], Lows: tape.Lows[from:to], Closes: tape.Closes[from:to],
	}
	if !reflect.DeepEqual(got, want) || got.Len() != size {
		t.Errorf("window ending at bar %d of %d bars is %+v, want %+v", end, size, got, want)
	}
	grown := append(got.Opens, -1)
	if grown[len(grown)-1] != -1 || tape.Opens[end+1] != int64(end+1)*series.BarMs {
		t.Errorf("a bar appended to a window reached the tape: %v", tape.Opens[end+1])
	}
	if all, err := cutWindow(tape, tape.Opens[end], end+1); err != nil || all.Len() != end+1 {
		t.Errorf("every bar up to the open is a window of its own size, got %d bars, %v", all.Len(), err)
	}
	for _, tc := range []struct {
		name string
		open int64
		size int
	}{
		{"one bar more than end there", tape.Opens[end], end + 2},
		{"no bar asked", tape.Opens[end], 0},
		{"a negative size", tape.Opens[end], -size},
		{"an open between two bars", tape.Opens[end] + 1, size},
		{"an open before the first bar", tape.Opens[0] - series.BarMs, 1},
		{"an open after the last bar", tape.Opens[len(tape.Opens)-1] + series.BarMs, 1},
	} {
		if window, err := cutWindow(tape, tc.open, tc.size); err == nil {
			t.Errorf("%s: want the window refused, got %d bars", tc.name, window.Len())
		}
	}
}
