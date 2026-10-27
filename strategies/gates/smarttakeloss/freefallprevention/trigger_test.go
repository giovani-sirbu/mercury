package freefallprevention

import (
	"reflect"
	"slices"
	"strconv"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// The row bits of the dashboard (1h, 4h, 1D), the masks' bit r for dashboard row r.
const (
	row1h int64 = 1 << iota
	row4h
	row1D
)

// withFreeFall runs the test with MinFreeFallTimeframes as asked and FreeFallTimeframes as listed
// (none lists none, which counts every row) and puts the package's values back after; package
// state, so never in parallel.
func withFreeFall(t *testing.T, asked int, timeframes ...string) {
	t.Helper()
	previousAsked, previousTimeframes := MinFreeFallTimeframes, FreeFallTimeframes
	MinFreeFallTimeframes, FreeFallTimeframes = asked, timeframes
	t.Cleanup(func() { MinFreeFallTimeframes, FreeFallTimeframes = previousAsked, previousTimeframes })
}

// needsTheShippedWindow skips a test that pins whole bars and wording on a window or a share other
// than the 240 bars of 1h and the 80% the spec ships.
func needsTheShippedWindow(t *testing.T) {
	t.Helper()
	if Window != 240 || MinShare != 0.8 || Timeframe != "1h" {
		t.Skipf("the test pins 192 and 193 of 240 1h bars over 80%%, the constants read %d bars of %s over %v", Window, Timeframe, MinShare)
	}
}

// rowsServed is the block with the dashboard's three rows served (1h, 4h, 1D) and each row's free
// fall and free rise as given, one entry per row; no series.
func rowsServed(fall, rise []bool) aggragates.SmartTakeLossIndicators {
	return aggragates.SmartTakeLossIndicators{
		SmcTrendTimeframes:  []string{"1h", "4h", "1D"},
		SmcTrendRowDown:     []int64{0, 0, 0},
		SmcTrendRowUp:       []int64{0, 0, 0},
		SmcTrendRowReads:    []int64{9, 9, 9},
		SmcTrendRowFreeFall: fall,
		SmcTrendRowFreeRise: rise,
	}
}

// onlyThe1hFalling is the dashboard on which only the 1h row is in free fall.
func onlyThe1hFalling() aggragates.SmartTakeLossIndicators {
	return rowsServed([]bool{true, false, false}, []bool{false, false, false})
}

// withSeries is the block with the series sophos serves in it.
func withSeries(block aggragates.SmartTakeLossIndicators, bars Series) aggragates.SmartTakeLossIndicators {
	block.FreeFallOpens, block.FreeFallMasks, block.FreeRiseMasks = bars.Opens, bars.FreeFall, bars.FreeRise
	return block
}

// The last closed bar alone holds: the counted row in free fall there holds the reading with no
// series served, naming the row, and nothing counts toward a share. A dashboard whose counted row
// is not in free fall reads nothing at all.
func TestTheLastClosedBarAloneHolds(t *testing.T) {
	got := Trigger(onlyThe1hFalling(), aggragates.SideLong)
	want := Reading{Holds: true, LastBar: true, Reasons: []string{"1h in free fall on the last closed bar"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("reading = %+v, want %+v", got, want)
	}

	for name, block := range map[string]aggragates.SmartTakeLossIndicators{
		"no row in free fall":      rowsServed([]bool{false, false, false}, []bool{false, false, false}),
		"the 4h and the 1D alone":  rowsServed([]bool{false, true, true}, nil),
		"nothing served":           {},
		"a short free fall served": rowsServed([]bool{true}, nil),
	} {
		if got := Trigger(block, aggragates.SideLong); !reflect.DeepEqual(got, Reading{}) {
			t.Errorf("%s: reading = %+v, want the zero reading", name, got)
		}
	}
}

// The share holds only when strictly more than MinShare of the bars served: 192 of 240 is exactly
// 80% and does not hold, 193 does, and the reading says how many of how many either way; a miss
// carries no reasons. The last bar is not free here, so the share holds alone.
func TestTheShareHoldsOnlyStrictlyOverMinShare(t *testing.T) {
	needsTheShippedWindow(t)
	for _, c := range []struct {
		name  string
		free  int
		holds bool
	}{
		{"none", 0, false},
		{"191 of 240", 191, false},
		{"192 of 240, exactly the share", 192, false},
		{"193 of 240, one bar over", 193, true},
		{"239 of 240", 239, true},
		{"every bar", 240, true},
	} {
		bars := barsOf(slices.Concat(run(Window-c.free, 0), run(c.free, row1h)), run(Window, 0))
		got := Trigger(withSeries(rowsServed(nil, nil), bars), aggragates.SideLong)
		if got.Holds != c.holds || got.LastBar || got.Count != c.free || got.Of != Window {
			t.Errorf("%s: reading = %+v, want holds %v on %d of %d with the last bar not free", c.name, got, c.holds, c.free, Window)
		}
		if c.holds {
			wantReason := []string{"1h in free fall on " + strconv.Itoa(c.free) + " of 240 1h bars, over 80%"}
			if !reflect.DeepEqual(got.Reasons, wantReason) {
				t.Errorf("%s: reasons = %q, want %q", c.name, got.Reasons, wantReason)
			}
		} else if got.Reasons != nil {
			t.Errorf("%s: a miss carries no reasons, got %q", c.name, got.Reasons)
		}
	}
}

// The share reads the newest Window bars and no others: a series longer than the window is cut to
// its tail, so older bars in free fall add nothing and older bars that were not dilute nothing; a
// series shorter than the window is read whole, as a share of the bars served.
func TestTheShareReadsOnlyTheNewestWindowBars(t *testing.T) {
	needsTheShippedWindow(t)
	const older = 60
	norise := run(Window+older, 0)

	// The 60 older bars all in free fall cannot lift a newest window at 192 of 240 over the share.
	lifted := barsOf(slices.Concat(run(older, row1h), run(48, 0), run(192, row1h)), norise)
	got := Trigger(withSeries(rowsServed(nil, nil), lifted), aggragates.SideLong)
	if got.Holds || got.Count != 192 || got.Of != Window {
		t.Errorf("older bars in free fall must not count: reading = %+v, want 192 of %d, not holding", got, Window)
	}

	// The 60 older bars all calm cannot drag a newest window all in free fall under the share.
	dragged := barsOf(slices.Concat(run(older, 0), run(Window, row1h)), norise)
	got = Trigger(withSeries(rowsServed(nil, nil), dragged), aggragates.SideLong)
	if !got.Holds || got.Count != Window || got.Of != Window {
		t.Errorf("older bars calm must not count: reading = %+v, want %d of %d holding", got, Window, Window)
	}

	// The window's oldest bar is the bar Window back: one bar older is out of it.
	edge := barsOf(slices.Concat(run(1, row1h), run(Window-193, 0), run(193, row1h)), run(Window+1, 0))
	got = Trigger(withSeries(rowsServed(nil, nil), edge), aggragates.SideLong)
	if !got.Holds || got.Count != 193 || got.Of != Window {
		t.Errorf("a bar one older than the window is out of it: reading = %+v, want 193 of %d holding", got, Window)
	}

	// Fewer bars than the window are read as a share of the bars served.
	short := barsOf(run(10, row1h), run(10, 0))
	got = Trigger(withSeries(rowsServed(nil, nil), short), aggragates.SideLong)
	if !got.Holds || got.Count != 10 || got.Of != 10 {
		t.Errorf("a short series is read whole: reading = %+v, want 10 of 10 holding", got)
	}
	short = barsOf(slices.Concat(run(2, 0), run(8, row1h)), run(10, 0))
	if got = Trigger(withSeries(rowsServed(nil, nil), short), aggragates.SideLong); got.Holds || got.Count != 8 || got.Of != 10 {
		t.Errorf("8 of 10 is exactly the share: reading = %+v, want 8 of 10, not holding", got)
	}
}

// With as many rows asked as counted the share counts a bar only when every counted row is in free
// fall on it: a bar with one counted bit missing does not count, a bar with more bits than counted
// does, and rows each over the share on bars that never coincide read as the few bars on which both
// did.
func TestTheShareCountsABarOnlyWhenEveryCountedRowIsFreeOnIt(t *testing.T) {
	needsTheShippedWindow(t)
	withFreeFall(t, 2, "1h", "4h")
	rise := run(Window, 0)

	both := slices.Concat(run(40, row1h), run(200, row1h|row4h))
	got := Trigger(withSeries(rowsServed(nil, nil), barsOf(both, rise)), aggragates.SideLong)
	if !got.Holds || got.Count != 200 || got.Of != Window {
		t.Errorf("the 1h alone on 40 bars: reading = %+v, want 200 of %d holding", got, Window)
	}
	wantReasons := []string{"1h and 4h in free fall on 200 of 240 1h bars, over 80%"}
	if !reflect.DeepEqual(got.Reasons, wantReasons) {
		t.Errorf("reasons = %q, want %q", got.Reasons, wantReasons)
	}

	missing := slices.Concat(run(60, row1h), run(180, row1h|row4h))
	got = Trigger(withSeries(rowsServed(nil, nil), barsOf(missing, rise)), aggragates.SideLong)
	if got.Holds || got.Count != 180 {
		t.Errorf("180 of 240 with the 4h missing on 60: reading = %+v, want 180, not holding", got)
	}

	// The 1h is free on bars 0..199, the 4h on bars 40..239: each row alone over the share (200 of
	// 240), together on bars 40..199 only (160 of 240).
	apart := slices.Concat(run(40, row1h), run(160, row1h|row4h), run(40, row4h))
	got = Trigger(withSeries(rowsServed(nil, nil), barsOf(apart, rise)), aggragates.SideLong)
	if got.Holds || got.Count != 160 {
		t.Errorf("rows free on different bars: reading = %+v, want 160 of %d, not holding", got, Window)
	}

	extra := run(Window, row1h|row4h|row1D)
	got = Trigger(withSeries(rowsServed(nil, nil), barsOf(extra, rise)), aggragates.SideLong)
	if !got.Holds || got.Count != Window {
		t.Errorf("a row beyond the asked ones does no harm: reading = %+v, want %d of %d holding", got, Window, Window)
	}

	withFreeFall(t, 1, "1d")
	got = Trigger(withSeries(rowsServed(nil, nil), barsOf(slices.Concat(run(Window, row1h|row4h)), rise)), aggragates.SideLong)
	if got.Holds || got.Count != 0 {
		t.Errorf("only the 1D counted and free on no bar: reading = %+v, want 0 of %d", got, Window)
	}
}

// The last bar asks every counted row at once when as many are asked as counted: with the 1h and
// the 4h counted and two asked, either alone misses, a row not listed adds nothing; only both hold.
func TestTheLastBarAsksEveryCountedRowAtOnce(t *testing.T) {
	withFreeFall(t, 2, "1h", "4h")
	for _, c := range []struct {
		name  string
		fall  []bool
		holds bool
	}{
		{"the 1h alone", []bool{true, false, false}, false},
		{"the 4h alone", []bool{false, true, false}, false},
		{"the 1D alone, not listed", []bool{false, false, true}, false},
		{"the 1h and the 1D, the 1D not listed", []bool{true, false, true}, false},
		{"the 1h and the 4h", []bool{true, true, false}, true},
		{"every row", []bool{true, true, true}, true},
	} {
		got := Trigger(rowsServed(c.fall, nil), aggragates.SideLong)
		if got.Holds != c.holds || got.LastBar != c.holds {
			t.Errorf("%s: reading = %+v, want holds %v", c.name, got, c.holds)
		}
	}
}

// An inverse ladder reads the mirror: the free rise of the rows on the last bar and the free rise
// masks of the series, worded free rise; a long ladder reads the free fall alone, and neither reads
// the other's.
func TestAnInverseLadderReadsTheFreeRise(t *testing.T) {
	needsTheShippedWindow(t)
	rising := rowsServed([]bool{false, false, false}, []bool{true, false, false})
	got := Trigger(rising, aggragates.SideShort)
	want := Reading{Holds: true, LastBar: true, Reasons: []string{"1h in free rise on the last closed bar"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("an inverse ladder on a free rise: reading = %+v, want %+v", got, want)
	}
	if got := Trigger(rising, aggragates.SideLong); !reflect.DeepEqual(got, Reading{}) {
		t.Errorf("a long ladder reads no free fall on a free rise: reading = %+v", got)
	}
	if got := Trigger(onlyThe1hFalling(), aggragates.SideShort); !reflect.DeepEqual(got, Reading{}) {
		t.Errorf("an inverse ladder reads no free rise on a free fall: reading = %+v", got)
	}

	bars := barsOf(run(Window, 0), run(Window, row1h))
	block := withSeries(rowsServed(nil, nil), bars)
	got = Trigger(block, aggragates.SideShort)
	if !got.Holds || got.LastBar || got.Count != Window || got.Of != Window {
		t.Errorf("an inverse ladder on the free rise masks: reading = %+v, want %d of %d holding", got, Window, Window)
	}
	if want := []string{"1h in free rise on 240 of 240 1h bars, over 80%"}; !reflect.DeepEqual(got.Reasons, want) {
		t.Errorf("reasons = %q, want %q", got.Reasons, want)
	}
	if got := Trigger(block, aggragates.SideLong); got.Holds || got.Count != 0 || got.Of != Window {
		t.Errorf("a long ladder reads the free fall masks, all calm: reading = %+v", got)
	}
	for _, side := range []string{"", "flat"} {
		if got := Trigger(withSeries(rising, bars), side); !reflect.DeepEqual(got, Reading{}) {
			t.Errorf("side %q is not read: reading = %+v, want the zero reading", side, got)
		}
	}
}

// With no row asked (MinFreeFallTimeframes at or below zero) the gate reads nothing, whatever the
// dashboard, the series and the list of timeframes serve or name, empty or not.
func TestNoRowAskedReadsNothing(t *testing.T) {
	block := withSeries(rowsServed([]bool{true, true, true}, []bool{true, true, true}), barsOf(run(Window, row1h|row4h|row1D), run(Window, row1h|row4h|row1D)))
	for _, c := range []struct {
		asked      int
		timeframes []string
	}{
		{0, []string{"1h"}},
		{0, nil},
		{0, []string{"1h", "4h", "1d"}},
		{-1, []string{"1h", "4h", "1d"}},
		{-1, nil},
		{-3, []string{"1h"}},
	} {
		withFreeFall(t, c.asked, c.timeframes...)
		for _, side := range []string{aggragates.SideLong, aggragates.SideShort} {
			if got := Trigger(block, side); !reflect.DeepEqual(got, Reading{}) {
				t.Errorf("asked %d of %q, side %s: reading = %+v, want the zero reading", c.asked, c.timeframes, side, got)
			}
		}
	}
}

// A series not served counts nothing and fails the share closed, 0 of 0, while the last bar still
// reads from the dashboard: bars with no opens, masks of unequal lengths and an empty series are
// all unserved, and a series served while the dashboard is not still reads by share.
func TestASeriesNotServedReadsNothingButTheLastBar(t *testing.T) {
	needsTheShippedWindow(t)
	full := run(Window, row1h)
	for name, bars := range map[string]Series{
		"none":                        {},
		"empty arrays":                {Opens: []int64{}, FreeFall: []int64{}, FreeRise: []int64{}},
		"masks with no opens":         {FreeFall: full, FreeRise: run(Window, 0)},
		"the free fall one short":     {Opens: barsOf(full, full).Opens, FreeFall: full[1:], FreeRise: run(Window, 0)},
		"the free rise one short":     {Opens: barsOf(full, full).Opens, FreeFall: full, FreeRise: run(Window-1, 0)},
		"the opens one short":         {Opens: barsOf(full, full).Opens[1:], FreeFall: full, FreeRise: run(Window, 0)},
		"every array a different one": {Opens: barsOf(full, full).Opens, FreeFall: full[2:], FreeRise: run(Window-1, 0)},
	} {
		got := Trigger(withSeries(rowsServed(nil, nil), bars), aggragates.SideLong)
		if !reflect.DeepEqual(got, Reading{}) {
			t.Errorf("%s: the share reads nothing: reading = %+v, want the zero reading", name, got)
		}
		got = Trigger(withSeries(onlyThe1hFalling(), bars), aggragates.SideLong)
		want := Reading{Holds: true, LastBar: true, Reasons: []string{"1h in free fall on the last closed bar"}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: the last bar still reads: reading = %+v, want %+v", name, got, want)
		}
	}

	// With no dashboard served the list decides: an empty one counts every bit, so the series reads
	// by share alone; a list naming timeframes has no row to match, so nothing counts.
	noDashboard := withSeries(aggragates.SmartTakeLossIndicators{}, barsOf(full, run(Window, 0)))
	withFreeFall(t, 1)
	got := Trigger(noDashboard, aggragates.SideLong)
	if !got.Holds || got.LastBar || got.Count != Window || got.Of != Window {
		t.Errorf("an empty list, a series served with no dashboard reads by share alone: reading = %+v", got)
	}
	withFreeFall(t, 1, "1h")
	got = Trigger(noDashboard, aggragates.SideLong)
	if got.Holds || got.LastBar || got.Count != 0 || got.Of != Window || got.Reasons != nil {
		t.Errorf("a listed timeframe, a series served with no dashboard counts nothing: reading = %+v, want 0 of %d not holding", got, Window)
	}
}

// A listed row the dashboard did not serve cannot be free on the last bar: no rows served, fewer
// rows than the two asked, and a free fall array that is not one per row all leave the rows not in
// free fall.
func TestAListedRowTheDashboardDidNotServeFailsTheLastBar(t *testing.T) {
	withFreeFall(t, 2, "1h", "4h")
	short := aggragates.SmartTakeLossIndicators{
		SmcTrendTimeframes: []string{"1h"}, SmcTrendRowDown: []int64{0}, SmcTrendRowUp: []int64{0}, SmcTrendRowReads: []int64{9},
		SmcTrendRowFreeFall: []bool{true},
	}
	for name, block := range map[string]aggragates.SmartTakeLossIndicators{
		"no rows":                     {},
		"one row of the two listed":   short,
		"free fall for two of three":  rowsServed([]bool{true, true}, nil),
		"free fall served for no row": rowsServed(nil, nil),
		"rows of unequal lengths":     {SmcTrendTimeframes: []string{"1h", "4h"}, SmcTrendRowDown: []int64{0}, SmcTrendRowUp: []int64{0, 0}, SmcTrendRowReads: []int64{9, 9}, SmcTrendRowFreeFall: []bool{true, true}},
	} {
		if got := Trigger(block, aggragates.SideLong); got.Holds || got.LastBar {
			t.Errorf("%s: reading = %+v, want the last bar not free", name, got)
		}
	}
}

// The reasons name the counted rows by their timeframes as the dashboard serves them, "1h", "1h and
// 4h", "1h, 4h and 1D", and a row served without its timeframe as "row N" counting from one, which
// only an empty list counts; the last bar's reason comes before the share's when both hold, and
// each is worded for the ladder's way.
func TestReasonsNameTheRows(t *testing.T) {
	needsTheShippedWindow(t)
	all := barsOf(run(Window, row1h|row4h|row1D), run(Window, row1h|row4h|row1D))
	unlabelled := aggragates.SmartTakeLossIndicators{SmcTrendTimeframes: []string{""}, SmcTrendRowDown: []int64{0}, SmcTrendRowUp: []int64{0}, SmcTrendRowReads: []int64{9}}
	twoLabelled := aggragates.SmartTakeLossIndicators{SmcTrendTimeframes: []string{"1h", "4h", ""}, SmcTrendRowDown: []int64{0, 0, 0}, SmcTrendRowUp: []int64{0, 0, 0}, SmcTrendRowReads: []int64{9, 9, 9}}
	for _, c := range []struct {
		name       string
		asked      int
		timeframes []string
		block      aggragates.SmartTakeLossIndicators
		want       string
	}{
		{"the 1h", 1, []string{"1h"}, rowsServed(nil, nil), "1h"},
		{"the 4h", 1, []string{"4h"}, rowsServed(nil, nil), "4h"},
		{"the 1D", 1, []string{"1D"}, rowsServed(nil, nil), "1D"},
		{"the 1D, listed in lower case, is named as served", 1, []string{"1d"}, rowsServed(nil, nil), "1D"},
		{"the 1h and the 4h", 2, []string{"1h", "4h"}, rowsServed(nil, nil), "1h and 4h"},
		{"the 1h and the 1D", 2, []string{"1h", "1D"}, rowsServed(nil, nil), "1h and 1D"},
		{"every row", 3, []string{"1h", "4h", "1D"}, rowsServed(nil, nil), "1h, 4h and 1D"},
		{"every row, an empty list", 3, nil, rowsServed(nil, nil), "1h, 4h and 1D"},
		{"no rows served, an empty list", 1, nil, aggragates.SmartTakeLossIndicators{}, "at least 1 of the rows"},
		{"no rows served, three asked, an empty list", 3, nil, aggragates.SmartTakeLossIndicators{}, "at least 3 of the rows"},
		{"a row served without its timeframe, an empty list", 1, nil, unlabelled, "row 1"},
		{"two rows and one without its timeframe, an empty list", 3, nil, twoLabelled, "1h, 4h and row 3"},
	} {
		withFreeFall(t, c.asked, c.timeframes...)
		got := Trigger(withSeries(c.block, all), aggragates.SideLong)
		want := []string{c.want + " in free fall on 240 of 240 1h bars, over 80%"}
		if !reflect.DeepEqual(got.Reasons, want) {
			t.Errorf("%s: reasons = %q, want %q", c.name, got.Reasons, want)
		}
	}

	withFreeFall(t, 2, "1h", "4h")
	both := Trigger(withSeries(rowsServed([]bool{true, true, false}, nil), all), aggragates.SideLong)
	want := []string{"1h and 4h in free fall on the last closed bar", "1h and 4h in free fall on 240 of 240 1h bars, over 80%"}
	if !both.Holds || !both.LastBar || !reflect.DeepEqual(both.Reasons, want) {
		t.Errorf("both holding: reading = %+v, want the last bar's reason first and the share's second", both)
	}
	inverse := Trigger(withSeries(rowsServed(nil, []bool{true, true, false}), all), aggragates.SideShort)
	want = []string{"1h and 4h in free rise on the last closed bar", "1h and 4h in free rise on 240 of 240 1h bars, over 80%"}
	if !reflect.DeepEqual(inverse.Reasons, want) {
		t.Errorf("an inverse ladder words them free rise: reasons = %q, want %q", inverse.Reasons, want)
	}
}

// Window is WindowTimeframe in bars of Timeframe, and MinShare a bound strictly between nothing
// and every bar: a 1h bar spans 60 minutes.
func TestWindowIsTheSpanInBarsOfTheTimeframe(t *testing.T) {
	if Window <= 0 {
		t.Fatalf("a window of %d bars reads nothing", Window)
	}
	if got := int(WindowTimeframe.Minutes()) / Window; got != 60 && Timeframe == "1h" {
		t.Errorf("a bar of 1h spans 60 minutes, the window's %d bars span %d minutes each", Window, got)
	}
	if MinShare <= 0 || MinShare >= 1 {
		t.Errorf("MinShare %v leaves no share to read: strictly over a bound between 0 and 1", MinShare)
	}
}
