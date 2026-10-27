package flatmarket

import (
	"encoding/json"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss/series"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// lineTapeFields are the fields of a bar of a testdata tape, in the order the tape lists them; the
// gate reads the open time, the open and the close.
var lineTapeFields = []string{"openTime", "open", "high", "low", "close"}

// loadLineTape is the bars of a 1h tape in testdata, by file name, as the gate reads them.
func loadLineTape(t *testing.T, name string) Bars {
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
	if file.Interval != "1h" || !slices.Equal(file.Fields, lineTapeFields) {
		t.Fatalf("the tape holds %s bars of %q, want 1h bars of %q", file.Interval, file.Fields, lineTapeFields)
	}
	var bars Bars
	for index, cells := range file.Bars {
		var open int64
		var openText, closeText string
		if len(cells) != len(lineTapeFields) || json.Unmarshal(cells[0], &open) != nil || json.Unmarshal(cells[1], &openText) != nil || json.Unmarshal(cells[4], &closeText) != nil {
			t.Fatalf("bar %d of the tape is not an open time and four prices", index)
		}
		opened, err := strconv.ParseFloat(openText, 64)
		if err != nil {
			t.Fatalf("bar %d: open: %v", index, err)
		}
		closed, err := strconv.ParseFloat(closeText, 64)
		if err != nil {
			t.Fatalf("bar %d: close: %v", index, err)
		}
		if index > 0 && open != bars.Opens[index-1]+series.BarMs {
			t.Fatalf("bar %d opens at %d, want one bar after the bar before it", index, open)
		}
		bars.Opens = append(bars.Opens, open)
		bars.OpenPrices = append(bars.OpenPrices, opened)
		bars.Closes = append(bars.Closes, closed)
	}
	if !bars.Served() {
		t.Fatal("the tape's bars are not served")
	}
	return bars
}

// lineWindowEnding is the Window bars of the tape ending at the bar that opens at stamp (UTC, to the minute).
func lineWindowEnding(t *testing.T, bars Bars, stamp string) Bars {
	t.Helper()
	at, err := time.Parse(barLayout, stamp)
	if err != nil {
		t.Fatalf("stamp %q: %v", stamp, err)
	}
	end, found := slices.BinarySearch(bars.Opens, at.UnixMilli())
	if !found || end+1 < Window {
		t.Fatalf("no window of %d bars ends at the bar opening %s", Window, stamp)
	}
	return lineWindow(bars, end)
}

// window is the Window bars of the tape ending at index end.
func lineWindow(bars Bars, end int) Bars {
	return Bars{
		Opens:      slices.Clip(bars.Opens[end+1-Window : end+1]),
		OpenPrices: slices.Clip(bars.OpenPrices[end+1-Window : end+1]),
		Closes:     slices.Clip(bars.Closes[end+1-Window : end+1]),
	}
}

// lineMeasuredAt are the constants the lineReplays below were measured at: the lineReplays run only while
// every one is what it was, and skip, naming the constant, as soon as one is retuned. MaxDeviation
// is not among them: each lineReplay states its bound and reads at the percentage that makes it.
var lineMeasuredAt = []struct {
	name                string
	inForce, measuredAs float64
}{
	{"TolerancePercentage", float64(TolerancePercentage), 10},
	{"Window", float64(Window), 48},
}

func lineNeedsTheMeasuredConstants(t *testing.T) {
	t.Helper()
	for _, constant := range lineMeasuredAt {
		if constant.inForce != constant.measuredAs {
			t.Skipf("%s is %v, the readings were measured at %v", constant.name, constant.inForce, constant.measuredAs)
		}
	}
}

// percentageFor is the ladder percentage that makes the bound under the MaxDeviation in force.
func percentageFor(bound float64) float64 {
	return bound / MaxDeviation
}

// lineMetTrend is a Trend Direction row that meets MinSmcTrend the side's way whatever it is.
func lineMetTrend(side string) series.Trend {
	rows := max(MinSmcTrend, 1)
	if side == aggragates.SideShort {
		return series.Trend{Up: rows, Valid: max(lineDashboardRows, rows)}
	}
	return series.Trend{Down: rows, Valid: max(lineDashboardRows, rows)}
}

// round2 is a percent to two decimals, as the reasons print it.
func round2(pct float64) float64 {
	return math.Round(pct*100) / 100
}

// lineReplay is one window of a tape read for a ladder of a side at a bound, and what it measures.
type lineReplay struct {
	tape, stamp, side     string
	bound                 float64
	holds                 bool
	movePct, above, below float64
	counted, wide         int
	names                 string
}

// BTC/USDT, late May 2026, at the bound of trade 161912's probe: the flat range before 06-01 holds
// as a flat line, the slide of 06-01 → 06-03 holds as a line down with the closes within
// +1.18%/−1.44% of it and one body over the bound, the window across the break of 06-01 12:00
// holds with one close counted, and the slide read for an inverse ladder misses on its way.
// SOL/USDT, May 2022 (trade 162696): the windows the gate triggered on in backtest 268 hold at its
// bound with two and four bars counted — the body of 05-07 21:00 among them, its close inside the
// band — and the second misses at a bound a quarter tighter.
var lineReplays = []lineReplay{
	{"btcusdt-1h-2026-05.json", "2026-06-01 09:00", aggragates.SideLong, 1.65, true, -0.91, 0.49, -0.86, 0, 0, "line flat"},
	{"btcusdt-1h-2026-05.json", "2026-06-03 03:00", aggragates.SideLong, 1.65, true, -9.84, 1.18, -1.44, 1, 1, "1 of 48 bars over the bound (1 on their body), at most 4"},
	{"btcusdt-1h-2026-05.json", "2026-06-01 15:00", aggragates.SideLong, 1.65, true, -2.48, 0.94, -2.04, 1, 0, "1 of 48 bars over the bound (0 on their body), at most 4"},
	{"btcusdt-1h-2026-05.json", "2026-06-03 03:00", aggragates.SideShort, 1.65, false, -9.84, 1.18, -1.44, 1, 1, "against the ladder"},
	{"solusdt-1h-2022-05-line.json", "2022-05-07 14:00", aggragates.SideLong, 2.40, true, -4.35, 1.79, -2.31, 2, 2, "2 of 48 bars over the bound (2 on their body), at most 4"},
	{"solusdt-1h-2022-05-line.json", "2022-05-08 11:00", aggragates.SideLong, 2.40, true, -5.37, 2.56, -2.51, 4, 1, "4 of 48 bars over the bound (1 on their body), at most 4"},
	{"solusdt-1h-2022-05-line.json", "2022-05-08 11:00", aggragates.SideLong, 2.25, false, -5.37, 2.56, -2.51, 6, 1, "6 of 48 bars over the bound 2.25% (1 on their body, the widest -3.01% at 2022-05-07 21:00), at most 4"},
}

func TestTapeReplays(t *testing.T) {
	lineNeedsTheMeasuredConstants(t)
	for _, tc := range lineReplays {
		t.Run(tc.tape+" "+tc.stamp+" "+tc.side+" "+strconv.FormatFloat(tc.bound, 'f', 2, 64), func(t *testing.T) {
			bars := lineWindowEnding(t, loadLineTape(t, tc.tape), tc.stamp)
			percentage, trend := percentageFor(tc.bound), lineMetTrend(tc.side)
			reading := TriggerLine(bars, percentage, tc.side, trend)
			measured := LineReading{Side: tc.side, Trend: trend, Bound: tc.bound}.measured(bars)
			measured = measured.deviated(fit(bars.Closes).read(bars, tc.bound))
			got := []float64{round2(measured.MovePct), round2(measured.MaxAbove), round2(measured.MaxBelow), float64(measured.Counted), float64(measured.Wide)}
			if want := []float64{tc.movePct, tc.above, tc.below, float64(tc.counted), float64(tc.wide)}; !slices.Equal(got, want) {
				t.Fatalf("the window measures %v (move, above, below, counted, wide), want %v", got, want)
			}
			if reading.Holds != tc.holds {
				t.Fatalf("holds = %v, want %v: %+v; misses %q", reading.Holds, tc.holds, reading, LineBreaks(bars, percentage, tc.side, trend))
			}
			texts := reading.Reasons
			if !tc.holds {
				texts = LineBreaks(bars, percentage, tc.side, trend)
			}
			if !slices.ContainsFunc(texts, func(text string) bool { return strings.Contains(text, tc.names) }) {
				t.Fatalf("the texts %q must name %q", texts, tc.names)
			}
		})
	}
}

// Every hourly window of the BTC slide, read for a long ladder at the probe's bound, holds: the
// counted bars never pass the tolerance, so the gate reads the shape the probe of 161912 described.
func TestBtcMay2026HoldsAcrossTheSlide(t *testing.T) {
	lineNeedsTheMeasuredConstants(t)
	tape := loadLineTape(t, "btcusdt-1h-2026-05.json")
	percentage, trend := percentageFor(1.65), lineMetTrend(aggragates.SideLong)
	var misses []string
	for end := Window - 1; end < len(tape.Opens); end++ {
		stamp := time.UnixMilli(tape.Opens[end]).UTC().Format(barLayout)
		if stamp < "2026-06-01 17:00" || stamp > "2026-06-03 09:00" {
			continue
		}
		if !TriggerLine(lineWindow(tape, end), percentage, aggragates.SideLong, trend).Holds {
			misses = append(misses, stamp[5:13])
		}
	}
	if len(misses) != 0 {
		t.Fatalf("the slide holds on every window, got misses at %v", misses)
	}
}
