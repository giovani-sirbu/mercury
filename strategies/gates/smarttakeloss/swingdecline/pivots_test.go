package swingdecline

import (
	"reflect"
	"slices"
	"testing"
)

// A swing high is the highest high of the bars each side and a swing low the
// lowest low, each by its own wick, and a bar equal to its neighbours counts:
// two equal highs side by side are both swing highs, a flat stretch is a swing
// high and a swing low at once, the high first, and a bar with a tall wick over
// a low one is both.
func TestSwingPivotsTiesCount(t *testing.T) {
	for name, tc := range map[string]struct {
		highs, lows []float64
		width       int
		want        []pivot
	}{
		"a peak and a trough": {
			highs: []float64{1, 3, 2, 4, 2, 1}, lows: []float64{1, 3, 2, 4, 2, 1},
			width: 1,
			want:  []pivot{{bar: 1, price: 3, high: true}, {bar: 2, price: 2}, {bar: 3, price: 4, high: true}},
		},
		"the wicks, not one price": {
			highs: []float64{1, 3, 2, 4, 2, 1}, lows: []float64{0.5, 2, 1, 3, 1, 0.5},
			width: 1,
			want:  []pivot{{bar: 1, price: 3, high: true}, {bar: 2, price: 1}, {bar: 3, price: 4, high: true}},
		},
		"a tall wick over a low one is both": {
			highs: []float64{1, 1, 5, 1, 1}, lows: []float64{1, 1, 0.5, 1, 1},
			width: 2,
			want:  []pivot{{bar: 2, price: 5, high: true}, {bar: 2, price: 0.5}},
		},
		"two equal highs side by side": {
			highs: []float64{1, 3, 3, 1}, lows: []float64{1, 3, 3, 1},
			width: 1,
			want:  []pivot{{bar: 1, price: 3, high: true}, {bar: 2, price: 3, high: true}},
		},
		"a flat stretch is both": {
			highs: []float64{5, 5, 5}, lows: []float64{5, 5, 5},
			width: 1,
			want:  []pivot{{bar: 1, price: 5, high: true}, {bar: 1, price: 5}},
		},
		"a peak with three bars each side": {
			highs: []float64{1, 2, 3, 5, 3, 2, 1}, lows: []float64{1, 2, 3, 5, 3, 2, 1},
			width: 3,
			want:  []pivot{{bar: 3, price: 5, high: true}},
		},
		"a neighbour higher by a hair is no high": {
			highs: []float64{1, 2, 3, 5, 3, 2, 5.0000001}, lows: []float64{1, 2, 3, 5, 3, 2, 5.0000001},
			width: 3,
			want:  nil,
		},
		"a neighbour tied is still a high": {
			highs: []float64{1, 2, 5, 5, 3, 2, 1}, lows: []float64{1, 2, 5, 5, 3, 2, 1},
			width: 3,
			want:  []pivot{{bar: 3, price: 5, high: true}},
		},
		"too short for one swing": {
			highs: []float64{1, 2, 1}, lows: []float64{1, 2, 1},
			width: 3,
			want:  nil,
		},
	} {
		if got := swingPivots(tc.highs, tc.lows, tc.width); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: got %+v, want %+v", name, got, tc.want)
		}
	}
}

// Appending bars never changes a pivot found before: the pivots of a series
// cut at any bar are a prefix of the pivots of the series cut one bar later,
// at every width, on a series full of ties.
func TestSwingPivotsAreCausal(t *testing.T) {
	highs := make([]float64, 0, 200)
	value := uint32(7)
	for len(highs) < 200 {
		value = value*1664525 + 1013904223
		highs = append(highs, float64(90+value>>28))
	}
	lows := make([]float64, len(highs))
	for index, high := range highs {
		lows[index] = high - 1
	}
	for _, width := range []int{1, 2, 3} {
		for cutAt := 1; cutAt < len(highs); cutAt++ {
			earlier, later := swingPivots(highs[:cutAt], lows[:cutAt], width), swingPivots(highs[:cutAt+1], lows[:cutAt+1], width)
			if len(later) < len(earlier) || !slices.Equal(later[:len(earlier)], earlier) {
				t.Fatalf("width %d: appending bar %d changed the pivots found before it:\n%+v\nthen\n%+v", width, cutAt, earlier, later)
			}
		}
	}
}

// A pivot is known only once width bars closed after it: the last bar that can
// be one sits width bars before the end.
func TestSwingPivotsNeedBarsAfter(t *testing.T) {
	wicks := []float64{1, 2, 3, 5, 3, 2, 1}
	if got := swingPivots(wicks[:6], wicks[:6], 3); len(got) != 0 {
		t.Fatalf("a peak with two bars after it is no swing at width 3, got %+v", got)
	}
	if got := swingPivots(wicks, wicks, 3); len(got) != 1 || got[0].bar != 3 {
		t.Fatalf("the same peak with three bars after it is one, got %+v", got)
	}
}

// Consecutive pivots of one kind fold into the more extreme of them, the later
// one on a tie, and a high and a low in turn are kept as they are.
func TestAlternateKeepsTheMoreExtreme(t *testing.T) {
	high := func(bar int, price float64) pivot { return pivot{bar: bar, price: price, high: true} }
	low := func(bar int, price float64) pivot { return pivot{bar: bar, price: price} }
	for name, tc := range map[string]struct {
		pivots []pivot
		want   []pivot
	}{
		"nothing":                       {nil, []pivot{}},
		"already alternating":           {[]pivot{high(1, 9), low(2, 5), high(3, 8)}, []pivot{high(1, 9), low(2, 5), high(3, 8)}},
		"the higher of two highs":       {[]pivot{high(1, 10), high(2, 12), low(3, 5)}, []pivot{high(2, 12), low(3, 5)}},
		"the lower of two highs stays":  {[]pivot{high(1, 12), high(2, 10), low(3, 5)}, []pivot{high(1, 12), low(3, 5)}},
		"the lower of two lows":         {[]pivot{high(1, 9), low(2, 5), low(3, 4), high(4, 8)}, []pivot{high(1, 9), low(3, 4), high(4, 8)}},
		"the later one on a high tie":   {[]pivot{high(1, 10), high(4, 10), low(5, 5)}, []pivot{high(4, 10), low(5, 5)}},
		"the later one on a low tie":    {[]pivot{high(1, 9), low(2, 4), low(5, 4), high(6, 8)}, []pivot{high(1, 9), low(5, 4), high(6, 8)}},
		"a run of three keeps the best": {[]pivot{high(1, 9), high(2, 11), high(3, 10), low(4, 5)}, []pivot{high(2, 11), low(4, 5)}},
	} {
		if got := alternate(tc.pivots); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: got %+v, want %+v", name, got, tc.want)
		}
	}
}

// The legs of a window run from the FIRST of its equal highest swing highs: two
// highs of 100 give one down leg more than starting at the second, the window's
// high on the first's bar, and the bounce back up to the second is over
// MaxBouncePct, so it is named and not counted.
func TestLegsStartAtTheFirstOfTheEqualHighestHighs(t *testing.T) {
	tape := staircase(legBars, 92, 100, 92, 100, 92, 94, 86)
	reading, readable := Read(tape, barOpenAt(0), barOpenAt(6*legBars), 1, 4)
	if !readable {
		t.Fatalf("the window must be read, got %+v", reading)
	}
	if reading.HighAt != barOpenAt(legBars) || reading.High != 100 {
		t.Errorf("want the high on the first 100's bar, got %v at %d", reading.High, reading.HighAt)
	}
	if reading.DownLegs != 3 || reading.UpLegs != 1 || reading.Bounces != 1 {
		t.Errorf("want 3 down legs from the first 100, 1 bounce under the bound and 1 over it, got %+v", reading)
	}
}
