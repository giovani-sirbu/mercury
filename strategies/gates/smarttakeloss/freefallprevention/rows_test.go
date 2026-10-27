package freefallprevention

import (
	"fmt"
	"reflect"
	"slices"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// rowsLabelled is the block with the dashboard serving the given timeframes as its rows, each in
// free fall and free rise as flagged, one entry per row; no series.
func rowsLabelled(timeframes []string, fall, rise []bool) aggragates.SmartTakeLossIndicators {
	count := len(timeframes)
	reads := make([]int64, count)
	for index := range reads {
		reads[index] = 9
	}
	return aggragates.SmartTakeLossIndicators{
		SmcTrendTimeframes:  timeframes,
		SmcTrendRowDown:     make([]int64, count),
		SmcTrendRowUp:       make([]int64, count),
		SmcTrendRowReads:    reads,
		SmcTrendRowFreeFall: fall,
		SmcTrendRowFreeRise: rise,
	}
}

// lastBarReason and shareReason are the reasons of the 240 bars of 1h over 80% the tests pin, for
// the rows as named and the way (free fall or free rise) as worded.
func lastBarReason(rows, way string) string {
	return fmt.Sprintf("%s in %s on the last closed bar", rows, way)
}

func shareReason(rows, way string, count int) string {
	return fmt.Sprintf("%s in %s on %d of 240 1h bars, over 80%%", rows, way, count)
}

// firstFree is count flags set out of three rows, from the first row on.
func firstFree(count int) []bool {
	flags := make([]bool, 3)
	for index := range count {
		flags[index] = true
	}
	return flags
}

// The last bar holds when at least MinFreeFallTimeframes of the counted rows are free on it, for
// any N of M: with three rows counted, asking one, two or three holds exactly from that many free,
// whichever rows they are.
func TestAtLeastNOfTheCountedRowsFreeHoldTheLastBar(t *testing.T) {
	for asked := 1; asked <= 3; asked++ {
		withFreeFall(t, asked, "1h", "4h", "1d")
		for free := 0; free <= 3; free++ {
			got := Trigger(rowsServed(firstFree(free), nil), aggragates.SideLong)
			if got.Holds != (free >= asked) || got.LastBar != (free >= asked) {
				t.Errorf("%d of 3 asked, %d free: reading = %+v, want holds %v", asked, free, got, free >= asked)
			}
			if got.Count != 0 || got.Of != 0 {
				t.Errorf("%d of 3 asked, %d free: no series served, reading = %+v", asked, free, got)
			}
		}
	}

	withFreeFall(t, 2, "1h", "4h", "1d")
	for _, c := range []struct {
		name string
		fall []bool
		rows string
	}{
		{"none", []bool{false, false, false}, ""},
		{"the 1h alone", []bool{true, false, false}, ""},
		{"the 4h alone", []bool{false, true, false}, ""},
		{"the 1D alone", []bool{false, false, true}, ""},
		{"the 1h and the 4h", []bool{true, true, false}, "1h and 4h"},
		{"the 1h and the 1D", []bool{true, false, true}, "1h and 1D"},
		{"the 4h and the 1D", []bool{false, true, true}, "4h and 1D"},
		{"all three", []bool{true, true, true}, "1h, 4h and 1D"},
	} {
		got := Trigger(rowsServed(c.fall, nil), aggragates.SideLong)
		want := Reading{}
		if c.rows != "" {
			want = Reading{Holds: true, LastBar: true, Reasons: []string{lastBarReason(c.rows, "free fall")}}
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("2 of 3 asked, %s free: reading = %+v, want %+v", c.name, got, want)
		}
	}
}

// The last bar's reason names the counted rows that are free on it and no others: a row free but not
// listed is left out, a counted row not free is left out, and no "at least" prefix is worded.
func TestTheLastBarReasonNamesExactlyTheFreeCountedRows(t *testing.T) {
	withFreeFall(t, 1, "1h", "4h", "1d")
	got := Trigger(rowsServed([]bool{true, false, true}, nil), aggragates.SideLong)
	if want := []string{lastBarReason("1h and 1D", "free fall")}; !reflect.DeepEqual(got.Reasons, want) {
		t.Errorf("3 counted, 1h and 1D free: reasons = %q, want %q", got.Reasons, want)
	}

	withFreeFall(t, 1, "1h", "4h")
	got = Trigger(rowsServed([]bool{false, true, true}, nil), aggragates.SideLong)
	if want := []string{lastBarReason("4h", "free fall")}; !reflect.DeepEqual(got.Reasons, want) {
		t.Errorf("1h and 4h counted, the 4h and the 1D free: reasons = %q, want %q", got.Reasons, want)
	}
}

// An empty list counts every row the dashboard serves, whatever its timeframe, and a list naming
// timeframes counts only those.
func TestAnEmptyListCountsEveryRow(t *testing.T) {
	odd := []string{"15m", "1h", "weekly"}
	withFreeFall(t, 2)
	got := Trigger(rowsLabelled(odd, []bool{true, false, true}, nil), aggragates.SideLong)
	want := Reading{Holds: true, LastBar: true, Reasons: []string{lastBarReason("15m and weekly", "free fall")}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("an empty list counts the 15m and the weekly: reading = %+v, want %+v", got, want)
	}
	if got := Trigger(rowsLabelled(odd, []bool{false, true, false}, nil), aggragates.SideLong); !reflect.DeepEqual(got, Reading{}) {
		t.Errorf("one row of three free, two asked: reading = %+v, want the zero reading", got)
	}

	withFreeFall(t, 1, "1h")
	if got := Trigger(rowsLabelled(odd, []bool{true, false, true}, nil), aggragates.SideLong); !reflect.DeepEqual(got, Reading{}) {
		t.Errorf("a list naming the 1h counts neither the 15m nor the weekly: reading = %+v, want the zero reading", got)
	}
}

// A listed timeframe matches a row whatever the case of either, on the last bar and on the share
// alike, and the reason names the row as the dashboard serves it.
func TestAListedTimeframeMatchesARowWhateverItsCase(t *testing.T) {
	needsTheShippedWindow(t)
	for _, c := range []struct{ listed, served string }{
		{"1D", "1D"},
		{"1d", "1D"},
		{"1D", "1d"},
		{"1H", "1h"},
		{"1h", "1H"},
		{"4H", "4h"},
	} {
		withFreeFall(t, 1, c.listed)
		got := Trigger(rowsLabelled([]string{c.served}, []bool{true}, nil), aggragates.SideLong)
		want := Reading{Holds: true, LastBar: true, Reasons: []string{lastBarReason(c.served, "free fall")}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("listed %q, served %q: reading = %+v, want %+v", c.listed, c.served, got, want)
		}

		bars := barsOf(run(Window, row1h), run(Window, 0))
		got = Trigger(withSeries(rowsLabelled([]string{c.served}, nil, nil), bars), aggragates.SideLong)
		want = Reading{Holds: true, Count: Window, Of: Window, Reasons: []string{shareReason(c.served, "free fall", Window)}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("listed %q, served %q, the share: reading = %+v, want %+v", c.listed, c.served, got, want)
		}
	}

	withFreeFall(t, 1, "1d")
	if got := Trigger(rowsLabelled([]string{"1h"}, []bool{true}, nil), aggragates.SideLong); !reflect.DeepEqual(got, Reading{}) {
		t.Errorf("a listed 1d does not match the 1h row: reading = %+v, want the zero reading", got)
	}
}

// A row counts once, however many entries name it: with the 1h listed three times, in either case,
// and two rows asked, the 1h free alone does not hold, and the reason names the row once.
func TestADuplicateEntryCountsARowOnce(t *testing.T) {
	needsTheShippedWindow(t)
	allBars := barsOf(run(Window, row1h|row4h|row1D), run(Window, 0))

	withFreeFall(t, 2, "1h", "1h", "1H")
	got := Trigger(withSeries(rowsServed([]bool{true, true, true}, nil), allBars), aggragates.SideLong)
	if got.Holds || got.LastBar || got.Count != 0 || got.Of != Window {
		t.Errorf("the 1h listed three times counts one row: reading = %+v, want 0 of %d, not holding", got, Window)
	}

	withFreeFall(t, 2, "1h", "1h", "4h", "4H")
	got = Trigger(withSeries(rowsServed([]bool{true, true, false}, nil), barsOf(run(Window, row1h|row4h), run(Window, 0))), aggragates.SideLong)
	want := []string{lastBarReason("1h and 4h", "free fall"), shareReason("1h and 4h", "free fall", Window)}
	if !got.Holds || !got.LastBar || got.Count != Window || !reflect.DeepEqual(got.Reasons, want) {
		t.Errorf("two rows listed twice each: reading = %+v, want both holding and reasons %q", got, want)
	}
	if got := Trigger(rowsServed([]bool{true, false, false}, nil), aggragates.SideLong); !reflect.DeepEqual(got, Reading{}) {
		t.Errorf("one of two rows free, however often listed: reading = %+v, want the zero reading", got)
	}
}

// A listed timeframe the dashboard does not serve never counts: it adds no row to the last bar, no
// bit to the share and no name to a reason.
func TestAListedTimeframeThatIsNotServedNeverCounts(t *testing.T) {
	needsTheShippedWindow(t)
	every := barsOf(run(Window, row1h|row4h|row1D), run(Window, 0))

	withFreeFall(t, 2, "1h", "15m")
	got := Trigger(withSeries(rowsServed([]bool{true, true, true}, nil), every), aggragates.SideLong)
	if got.Holds || got.LastBar || got.Count != 0 || got.Of != Window {
		t.Errorf("the 1h counted and the 15m not served, two asked: reading = %+v, want 0 of %d, not holding", got, Window)
	}

	withFreeFall(t, 1, "15m")
	got = Trigger(withSeries(rowsServed([]bool{true, true, true}, nil), every), aggragates.SideLong)
	if got.Holds || got.LastBar || got.Count != 0 || got.Of != Window || got.Reasons != nil {
		t.Errorf("only the 15m listed: reading = %+v, want 0 of %d, not holding", got, Window)
	}

	withFreeFall(t, 1, "15m", "4h")
	got = Trigger(withSeries(rowsServed([]bool{true, true, true}, nil), barsOf(run(Window, row4h), run(Window, 0))), aggragates.SideLong)
	want := []string{lastBarReason("4h", "free fall"), shareReason("4h", "free fall", Window)}
	if !got.Holds || !got.LastBar || got.Count != Window || !reflect.DeepEqual(got.Reasons, want) {
		t.Errorf("the 4h counted beside a 15m not served: reading = %+v, want both holding and reasons %q", got, want)
	}
	got = Trigger(withSeries(rowsServed([]bool{true, false, true}, nil), barsOf(run(Window, row1h|row1D), run(Window, 0))), aggragates.SideLong)
	if got.Holds || got.LastBar || got.Count != 0 {
		t.Errorf("the rows not listed free: reading = %+v, want 0 of %d, not holding", got, Window)
	}
}

// The share counts a bar when at least MinFreeFallTimeframes of the counted bits are set on it, not
// all of them: with two of three asked, a bar with any two bits counts, a bar with one does not, and
// a bit of a row not counted adds nothing.
func TestTheShareCountsABarWithMinOfTheCountedRowsFreeOnIt(t *testing.T) {
	needsTheShippedWindow(t)
	withFreeFall(t, 2, "1h", "4h", "1d")
	rise := run(Window, 0)

	for _, mask := range []int64{row1h | row4h, row1h | row1D, row4h | row1D, row1h | row4h | row1D} {
		got := Trigger(withSeries(rowsServed(nil, nil), barsOf(run(Window, mask), rise)), aggragates.SideLong)
		if !got.Holds || got.LastBar || got.Count != Window || got.Of != Window {
			t.Errorf("every bar with mask %03b: reading = %+v, want %d of %d holding", mask, got, Window, Window)
		}
	}
	for _, mask := range []int64{0, row1h, row4h, row1D} {
		got := Trigger(withSeries(rowsServed(nil, nil), barsOf(run(Window, mask), rise)), aggragates.SideLong)
		if got.Holds || got.Count != 0 || got.Of != Window {
			t.Errorf("every bar with mask %03b: reading = %+v, want 0 of %d, not holding", mask, got, Window)
		}
	}

	// 192 of 240 is exactly the share and 193 is over it, the other bars holding one bit.
	for _, c := range []struct {
		free  int
		holds bool
	}{{192, false}, {193, true}} {
		bars := barsOf(slices.Concat(run(Window-c.free, row1h), run(c.free, row4h|row1D)), rise)
		got := Trigger(withSeries(rowsServed(nil, nil), bars), aggragates.SideLong)
		if got.Holds != c.holds || got.Count != c.free || got.Of != Window {
			t.Errorf("%d bars with two bits of %d: reading = %+v, want holds %v", c.free, Window, got, c.holds)
		}
	}

	withFreeFall(t, 2, "1h", "4h")
	got := Trigger(withSeries(rowsServed(nil, nil), barsOf(run(Window, row1h|row1D), rise)), aggragates.SideLong)
	if got.Holds || got.Count != 0 {
		t.Errorf("the 1D bit is not counted, the 1h alone is one: reading = %+v, want 0 of %d", got, Window)
	}
	got = Trigger(withSeries(rowsServed(nil, nil), barsOf(run(Window, row1h|row4h|row1D), rise)), aggragates.SideLong)
	if !got.Holds || got.Count != Window {
		t.Errorf("the 1h and the 4h counted with the 1D beside them: reading = %+v, want %d of %d holding", got, Window, Window)
	}
}

// More rows asked than counted can never hold, on the last bar or on the share, whatever is free.
func TestMoreRowsAskedThanCountedNeverHolds(t *testing.T) {
	needsTheShippedWindow(t)
	withFreeFall(t, 4, "1h", "4h", "1d")
	block := withSeries(rowsServed([]bool{true, true, true}, []bool{true, true, true}), barsOf(run(Window, row1h|row4h|row1D), run(Window, row1h|row4h|row1D)))
	for _, side := range []string{aggragates.SideLong, aggragates.SideShort} {
		got := Trigger(block, side)
		if got.Holds || got.LastBar || got.Count != 0 || got.Reasons != nil {
			t.Errorf("side %s, four asked of three counted: reading = %+v, want not holding on 0 bars", side, got)
		}
	}
}

// The share's reason says "at least N of " before the rows only when N is fewer than the counted
// rows served; when N equals their number it is the plain list of rows, as it was before. A listed
// timeframe not served is not a counted row.
func TestTheShareReasonSaysAtLeastOnlyWhenMinIsFewerThanTheCountedRows(t *testing.T) {
	needsTheShippedWindow(t)
	all := barsOf(run(Window, row1h|row4h|row1D), run(Window, 0))
	twoServed := rowsLabelled([]string{"1h", "4h"}, nil, nil)
	for _, c := range []struct {
		name       string
		asked      int
		timeframes []string
		block      aggragates.SmartTakeLossIndicators
		want       string
	}{
		{"1 of 3", 1, []string{"1h", "4h", "1d"}, rowsServed(nil, nil), "at least 1 of 1h, 4h and 1D"},
		{"2 of 3", 2, []string{"1h", "4h", "1d"}, rowsServed(nil, nil), "at least 2 of 1h, 4h and 1D"},
		{"3 of 3", 3, []string{"1h", "4h", "1d"}, rowsServed(nil, nil), "1h, 4h and 1D"},
		{"2 of 3, an empty list", 2, nil, rowsServed(nil, nil), "at least 2 of 1h, 4h and 1D"},
		{"3 of 3, an empty list", 3, nil, rowsServed(nil, nil), "1h, 4h and 1D"},
		{"1 of 2, a third listed and not served", 1, []string{"1h", "4h", "15m"}, rowsServed(nil, nil), "at least 1 of 1h and 4h"},
		{"2 of 2, a third listed and not served", 2, []string{"1h", "4h", "15m"}, rowsServed(nil, nil), "1h and 4h"},
		{"1 of 1", 1, []string{"4h"}, rowsServed(nil, nil), "4h"},
		{"3 asked of the 2 rows served, an empty list", 3, nil, twoServed, "1h and 4h"},
	} {
		withFreeFall(t, c.asked, c.timeframes...)
		got := Trigger(withSeries(c.block, all), aggragates.SideLong)
		want := []string{shareReason(c.want, "free fall", Window)}
		if !got.Holds || got.LastBar || !reflect.DeepEqual(got.Reasons, want) {
			t.Errorf("%s: reading = %+v, want the share alone holding with %q", c.name, got, want)
		}
	}

	// The last bar's reason carries no prefix beside a share's that does.
	withFreeFall(t, 2, "1h", "4h", "1d")
	got := Trigger(withSeries(rowsServed([]bool{true, true, false}, nil), all), aggragates.SideLong)
	want := []string{lastBarReason("1h and 4h", "free fall"), shareReason("at least 2 of 1h, 4h and 1D", "free fall", Window)}
	if !reflect.DeepEqual(got.Reasons, want) {
		t.Errorf("both holding: reasons = %q, want %q", got.Reasons, want)
	}
}

// With an empty list every bit of a bar counts, with no label served too: the share holds with no
// counted row served and says "at least N of the rows"; a bit beyond the rows served counts as well.
// With a list naming timeframes and no label served, nothing counts.
func TestWithNoLabelsServedOnlyAnEmptyListReadsTheShare(t *testing.T) {
	needsTheShippedWindow(t)
	bits := run(Window, row1h|row4h)
	block := withSeries(aggragates.SmartTakeLossIndicators{}, barsOf(bits, run(Window, 0)))

	withFreeFall(t, 2)
	got := Trigger(block, aggragates.SideLong)
	want := Reading{Holds: true, Count: Window, Of: Window, Reasons: []string{shareReason("at least 2 of the rows", "free fall", Window)}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("an empty list, no row served: reading = %+v, want %+v", got, want)
	}

	withFreeFall(t, 3)
	if got := Trigger(block, aggragates.SideLong); got.Holds || got.Count != 0 || got.Of != Window {
		t.Errorf("an empty list, three asked of two bits: reading = %+v, want 0 of %d, not holding", got, Window)
	}

	withFreeFall(t, 2, "1h", "4h", "1d")
	got = Trigger(block, aggragates.SideLong)
	if got.Holds || got.LastBar || got.Count != 0 || got.Of != Window || got.Reasons != nil {
		t.Errorf("a list naming timeframes, no row served: reading = %+v, want 0 of %d, not holding", got, Window)
	}

	// An empty list counts a bit beyond the three rows served.
	withFreeFall(t, 2)
	beyond := withSeries(rowsServed(nil, nil), barsOf(run(Window, 1<<5|1<<6), run(Window, 0)))
	got = Trigger(beyond, aggragates.SideLong)
	want = Reading{Holds: true, Count: Window, Of: Window, Reasons: []string{shareReason("at least 2 of 1h, 4h and 1D", "free fall", Window)}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("an empty list, bits beyond the rows served: reading = %+v, want %+v", got, want)
	}

	// With a list, the same bits are none of the counted rows.
	withFreeFall(t, 1, "1h", "4h", "1d")
	if got := Trigger(beyond, aggragates.SideLong); got.Holds || got.Count != 0 {
		t.Errorf("a list, bits beyond the rows served: reading = %+v, want 0 of %d, not holding", got, Window)
	}
}

// A row served without its timeframe is named "row N" from one by its place on the dashboard, and
// an empty list counts it: a list naming timeframes counts the rows it names and not this one.
func TestARowWithoutItsTimeframeIsNamedByItsPlaceAndCountedByAnEmptyListOnly(t *testing.T) {
	blank := rowsLabelled([]string{"", "1h", ""}, []bool{true, true, true}, nil)

	withFreeFall(t, 3)
	got := Trigger(blank, aggragates.SideLong)
	want := Reading{Holds: true, LastBar: true, Reasons: []string{lastBarReason("row 1, 1h and row 3", "free fall")}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("an empty list: reading = %+v, want %+v", got, want)
	}

	withFreeFall(t, 1, "1h", "4h", "1d")
	got = Trigger(blank, aggragates.SideLong)
	want = Reading{Holds: true, LastBar: true, Reasons: []string{lastBarReason("1h", "free fall")}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("a list: the unlabelled rows are not counted: reading = %+v, want %+v", got, want)
	}
}

// An inverse ladder reads the free rise of the counted rows, N of M, on the last bar and on the
// share, worded free rise, and never the free fall; a long ladder reads the free fall alone.
func TestAnInverseLadderReadsTheFreeRiseOfNOfTheCountedRows(t *testing.T) {
	needsTheShippedWindow(t)
	withFreeFall(t, 2, "1h", "4h", "1d")
	rising := rowsServed([]bool{false, false, true}, []bool{true, true, false})

	got := Trigger(rising, aggragates.SideShort)
	want := Reading{Holds: true, LastBar: true, Reasons: []string{lastBarReason("1h and 4h", "free rise")}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("an inverse ladder, two rows in free rise: reading = %+v, want %+v", got, want)
	}
	if got := Trigger(rising, aggragates.SideLong); !reflect.DeepEqual(got, Reading{}) {
		t.Errorf("a long ladder reads one row in free fall of the two asked: reading = %+v, want the zero reading", got)
	}
	if got := Trigger(rowsServed(nil, []bool{true, false, false}), aggragates.SideShort); !reflect.DeepEqual(got, Reading{}) {
		t.Errorf("an inverse ladder, one row in free rise of the two asked: reading = %+v, want the zero reading", got)
	}

	bars := barsOf(run(Window, 0), run(Window, row4h|row1D))
	block := withSeries(rowsServed(nil, nil), bars)
	got = Trigger(block, aggragates.SideShort)
	want = Reading{Holds: true, Count: Window, Of: Window, Reasons: []string{shareReason("at least 2 of 1h, 4h and 1D", "free rise", Window)}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("an inverse ladder on the free rise masks: reading = %+v, want %+v", got, want)
	}
	if got := Trigger(block, aggragates.SideLong); got.Holds || got.Count != 0 || got.Of != Window {
		t.Errorf("a long ladder reads the free fall masks, all calm: reading = %+v", got)
	}
}
