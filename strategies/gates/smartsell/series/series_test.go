package series

import (
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// barOpenAt is the open time in ms of the bar index hours after a fixed first bar.
func barOpenAt(index int) int64 {
	return time.Date(2021, time.October, 10, 0, 0, 0, 0, time.UTC).UnixMilli() + int64(index)*BarMs
}

// The series is the block's two arrays as sophos served them, and it is served
// only when they are one list of bars: both empty, only one of them, or of
// different lengths is not, and then it has no last bar and no bar is in it.
func TestSeriesIsServedOnlyAsOneListOfBars(t *testing.T) {
	block := aggragates.SmartSellIndicators{SeriesOpens: []int64{barOpenAt(0), barOpenAt(1), barOpenAt(3)}, SeriesCloses: []float64{10, 11, 12}}
	served := SeriesOf(block)
	if !served.Served() || served.LastBarOpen() != barOpenAt(3) {
		t.Fatalf("three bars are served up to the last open, got %+v", served)
	}
	if index, found := served.IndexOf(barOpenAt(3)); !found || index != 2 {
		t.Errorf("a bar after a gap is found by its open time, got %d, %v", index, found)
	}
	if _, found := served.IndexOf(barOpenAt(2)); found {
		t.Error("a bar the exchange left out is not in the series")
	}
	for name, block := range map[string]aggragates.SmartSellIndicators{
		"neither array":     {},
		"empty arrays":      {SeriesOpens: []int64{}, SeriesCloses: []float64{}},
		"only the opens":    {SeriesOpens: []int64{barOpenAt(0)}},
		"only the closes":   {SeriesCloses: []float64{10}},
		"different lengths": {SeriesOpens: []int64{barOpenAt(0), barOpenAt(1)}, SeriesCloses: []float64{10}},
	} {
		tape := SeriesOf(block)
		if _, found := tape.IndexOf(barOpenAt(0)); tape.Served() || tape.LastBarOpen() != 0 || found {
			t.Errorf("%s: no series is served, got %+v", name, tape)
		}
	}
}

// The wicks are served only as a high and a low for every bar of a served series, and the first
// bar at or after an open is found whatever the open: one before the series gives the first bar,
// one in a gap the bar after it, one past the last bar none.
func TestSeriesServesTheWicksAndFindsTheBarAtOrAfter(t *testing.T) {
	block := aggragates.SmartSellIndicators{
		SeriesOpens:  []int64{barOpenAt(0), barOpenAt(1), barOpenAt(3)},
		SeriesCloses: []float64{10, 11, 12},
		SeriesHighs:  []float64{11, 12, 13},
		SeriesLows:   []float64{9, 10, 11},
	}
	served := SeriesOf(block)
	if !served.WicksServed() || served.Highs[2] != 13 || served.Lows[0] != 9 {
		t.Fatalf("the wicks are served as decoded, got %+v", served)
	}
	for name, bars := range map[string]Series{
		"no wicks":   {Opens: served.Opens, Closes: served.Closes},
		"only highs": {Opens: served.Opens, Closes: served.Closes, Highs: served.Highs},
		"short lows": {Opens: served.Opens, Closes: served.Closes, Highs: served.Highs, Lows: served.Lows[:2]},
		"not served": {Highs: served.Highs, Lows: served.Lows},
	} {
		if bars.WicksServed() {
			t.Errorf("%s: the wicks are not served, got %+v", name, bars)
		}
	}
	for name, tc := range map[string]struct {
		open, want int64
		found      bool
	}{
		"before the series": {barOpenAt(-5), barOpenAt(0), true},
		"on a bar":          {barOpenAt(1), barOpenAt(1), true},
		"in a gap":          {barOpenAt(2), barOpenAt(3), true},
		"after the last":    {barOpenAt(4), 0, false},
	} {
		if got, found := served.BarAtOrAfter(tc.open); got != tc.want || found != tc.found {
			t.Errorf("%s: want %d, %v, got %d, %v", name, tc.want, tc.found, got, found)
		}
	}
	if _, found := (Series{}).BarAtOrAfter(barOpenAt(0)); found {
		t.Error("a series not served holds no bar")
	}
}

// The dashboard's rows are the four arrays as served, in their order, and no rows at all when
// the arrays disagree or none was served; a row's Way is its reads the side's way.
func TestReadsOfMapsTheDashboardRowsOrNone(t *testing.T) {
	block := aggragates.SmartSellIndicators{
		SmcTrendTimeframes: []string{"1h", "4h", "1D"},
		SmcTrendRowDown:    []int64{7, 5, 0},
		SmcTrendRowUp:      []int64{1, 3, 0},
		SmcTrendRowReads:   []int64{9, 9, 0},
	}
	rows := ReadsOf(block)
	if len(rows) != 3 || rows[0] != (TrendRow{Timeframe: "1h", Down: 7, Up: 1, Reads: 9}) || rows[2] != (TrendRow{Timeframe: "1D"}) {
		t.Fatalf("the rows read as served, got %+v", rows)
	}
	if rows[1].Way(aggragates.SideLong) != 5 || rows[1].Way(aggragates.SideShort) != 3 || rows[1].Way("") != 0 {
		t.Errorf("a row's way is its reads the side's way, got %+v", rows[1])
	}
	uneven := block
	uneven.SmcTrendRowUp = []int64{1}
	if ReadsOf(uneven) != nil || ReadsOf(aggragates.SmartSellIndicators{}) != nil {
		t.Error("arrays that disagree, or none, are no rows")
	}
}

// The rows' free fall and free rise are the block's two bool arrays as served, row by row in
// dashboard order, each apart from the other and from the reads.
func TestReadsOfMapsTheFreeFallAndFreeRiseOntoTheRows(t *testing.T) {
	block := aggragates.SmartSellIndicators{
		SmcTrendTimeframes:  []string{"1h", "4h", "1D"},
		SmcTrendRowDown:     []int64{7, 5, 0},
		SmcTrendRowUp:       []int64{1, 3, 0},
		SmcTrendRowReads:    []int64{9, 9, 0},
		SmcTrendRowFreeFall: []bool{false, true, true},
		SmcTrendRowFreeRise: []bool{true, false, true},
	}
	want := []TrendRow{
		{Timeframe: "1h", Down: 7, Up: 1, Reads: 9, FreeRise: true},
		{Timeframe: "4h", Down: 5, Up: 3, Reads: 9, FreeFall: true},
		{Timeframe: "1D", FreeFall: true, FreeRise: true},
	}
	rows := ReadsOf(block)
	if len(rows) != len(want) {
		t.Fatalf("the rows read as served, got %+v", rows)
	}
	for index := range want {
		if rows[index] != want[index] {
			t.Errorf("row %d = %+v, want %+v", index, rows[index], want[index])
		}
	}
}

// A free fall or free rise array that is not one entry per row (an older sophos serves none, a
// broken one the wrong count) leaves every row out of it but still serves the rows, which the four
// count arrays alone decide: the other array is read for what it is.
func TestReadsOfLeavesNoRowFreeWhenItsArrayIsNotOnePerRow(t *testing.T) {
	block := aggragates.SmartSellIndicators{
		SmcTrendTimeframes: []string{"1h", "4h", "1D"},
		SmcTrendRowDown:    []int64{7, 5, 0},
		SmcTrendRowUp:      []int64{1, 3, 0},
		SmcTrendRowReads:   []int64{9, 9, 0},
	}
	counts := []TrendRow{{Timeframe: "1h", Down: 7, Up: 1, Reads: 9}, {Timeframe: "4h", Down: 5, Up: 3, Reads: 9}, {Timeframe: "1D"}}
	cases := map[string]struct {
		fall, rise []bool
		want       []TrendRow
	}{
		"none served, an older sophos": {nil, nil, counts},
		"empty arrays":                 {[]bool{}, []bool{}, counts},
		"too few entries":              {[]bool{true, true}, []bool{true}, counts},
		"too many entries":             {[]bool{true, true, true, true}, []bool{true, true, true, true}, counts},
		"only the free fall one per row": {[]bool{false, true, false}, []bool{true}, []TrendRow{
			{Timeframe: "1h", Down: 7, Up: 1, Reads: 9}, {Timeframe: "4h", Down: 5, Up: 3, Reads: 9, FreeFall: true}, {Timeframe: "1D"},
		}},
		"only the free rise one per row": {[]bool{true, true}, []bool{false, false, true}, []TrendRow{
			{Timeframe: "1h", Down: 7, Up: 1, Reads: 9}, {Timeframe: "4h", Down: 5, Up: 3, Reads: 9}, {Timeframe: "1D", FreeRise: true},
		}},
	}
	for name, c := range cases {
		served := block
		served.SmcTrendRowFreeFall, served.SmcTrendRowFreeRise = c.fall, c.rise
		rows := ReadsOf(served)
		if len(rows) != len(c.want) {
			t.Errorf("%s: the rows are still served, got %+v", name, rows)
			continue
		}
		for index := range c.want {
			if rows[index] != c.want[index] {
				t.Errorf("%s: row %d = %+v, want %+v", name, index, rows[index], c.want[index])
			}
		}
	}
	free := block
	free.SmcTrendRowFreeFall, free.SmcTrendRowFreeRise = []bool{true, true, true}, []bool{true, true, true}
	free.SmcTrendRowUp = []int64{1}
	if ReadsOf(free) != nil {
		t.Error("free fall does not make rows of count arrays that disagree")
	}
}

// A row is in free fall the side's way: its FreeFall for a long ladder, its FreeRise for a short
// one, never for another side.
func TestTrendRowFreeIsTheSidesWay(t *testing.T) {
	cases := []struct {
		name  string
		row   TrendRow
		long  bool
		short bool
	}{
		{"free fall", TrendRow{FreeFall: true}, true, false},
		{"free rise", TrendRow{FreeRise: true}, false, true},
		{"both", TrendRow{FreeFall: true, FreeRise: true}, true, true},
		{"neither", TrendRow{Down: 9, Up: 9, Reads: 9}, false, false},
	}
	for _, c := range cases {
		if got := c.row.Free(aggragates.SideLong); got != c.long {
			t.Errorf("%s: a long ladder's free = %v, want %v", c.name, got, c.long)
		}
		if got := c.row.Free(aggragates.SideShort); got != c.short {
			t.Errorf("%s: a short ladder's free = %v, want %v", c.name, got, c.short)
		}
		if c.row.Free("") || c.row.Free("flat") {
			t.Errorf("%s: another side is never in free fall", c.name)
		}
	}
}
