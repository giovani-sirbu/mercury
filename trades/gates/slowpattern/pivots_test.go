package slowpattern

import (
	"reflect"
	"slices"
	"testing"
)

// A swing is the highest — or lowest — close of the bars each side, and a bar
// equal to its neighbours counts: two equal highs side by side are both swing
// highs, a flat stretch is a swing high and a swing low at once, the high
// first.
func TestSwingPivotsTiesCount(t *testing.T) {
	for name, tc := range map[string]struct {
		closes []float64
		width  int
		want   []pivot
	}{
		"a peak and a trough": {
			closes: []float64{1, 3, 2, 4, 2, 1},
			width:  1,
			want:   []pivot{{bar: 1, price: 3, high: true}, {bar: 2, price: 2}, {bar: 3, price: 4, high: true}},
		},
		"two equal highs side by side": {
			closes: []float64{1, 3, 3, 1},
			width:  1,
			want:   []pivot{{bar: 1, price: 3, high: true}, {bar: 2, price: 3, high: true}},
		},
		"a flat stretch is both": {
			closes: []float64{5, 5, 5},
			width:  1,
			want:   []pivot{{bar: 1, price: 5, high: true}, {bar: 1, price: 5}},
		},
		"a peak with three bars each side": {
			closes: []float64{1, 2, 3, 5, 3, 2, 1},
			width:  3,
			want:   []pivot{{bar: 3, price: 5, high: true}},
		},
		"a neighbour higher by a hair is no high": {
			closes: []float64{1, 2, 3, 5, 3, 2, 5.0000001},
			width:  3,
			want:   nil,
		},
		"a neighbour tied is still a high": {
			closes: []float64{1, 2, 5, 5, 3, 2, 1},
			width:  3,
			want:   []pivot{{bar: 3, price: 5, high: true}},
		},
		"too short for one swing": {
			closes: []float64{1, 2, 1},
			width:  3,
			want:   nil,
		},
	} {
		if got := swingPivots(tc.closes, tc.width); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: got %+v, want %+v", name, got, tc.want)
		}
	}
}

// Appending bars never changes a pivot found before: the pivots of a series
// cut at any bar are a prefix of the pivots of the series cut one bar later,
// at every width, on a series full of ties.
func TestSwingPivotsAreCausal(t *testing.T) {
	closes := make([]float64, 0, 200)
	value := uint32(7)
	for len(closes) < 200 {
		value = value*1664525 + 1013904223
		closes = append(closes, float64(90+value>>28))
	}
	for _, width := range []int{1, 2, 3} {
		for cut := 1; cut < len(closes); cut++ {
			earlier, later := swingPivots(closes[:cut], width), swingPivots(closes[:cut+1], width)
			if len(later) < len(earlier) || !slices.Equal(later[:len(earlier)], earlier) {
				t.Fatalf("width %d: appending bar %d changed the pivots found before it:\n%+v\nthen\n%+v", width, cut, earlier, later)
			}
		}
	}
}

// A pivot is known only once width bars closed after it: the last bar that can
// be one sits width bars before the end.
func TestSwingPivotsNeedBarsAfter(t *testing.T) {
	closes := []float64{1, 2, 3, 5, 3, 2, 1}
	if got := swingPivots(closes[:6], 3); len(got) != 0 {
		t.Fatalf("a peak with two bars after it is no swing at width 3, got %+v", got)
	}
	if got := swingPivots(closes, 3); len(got) != 1 || got[0].bar != 3 {
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

// The legs of a window run from the FIRST of its equal highest swing highs, and
// a down leg counts as a lower high only when it starts strictly under the down
// leg before it: two highs of 100 give one down leg more than starting at the
// second, and no lower high between themselves.
func TestLegsStartAtTheFirstOfTheEqualHighestHighsAndLowerHighsAreStrict(t *testing.T) {
	series := staircase(4, 94, 100, 92, 100, 92, 96, 88)
	reading, readable := Read(series, barOpenAt(0), barOpenAt(24), 1, 4)
	if !readable {
		t.Fatalf("the window must be read, got %+v", reading)
	}
	if reading.DownLegs != 3 || reading.LowerHighs != 1 {
		t.Errorf("want 3 down legs from the first 100 and 1 lower high, the 96 under the second 100, got %d and %d", reading.DownLegs, reading.LowerHighs)
	}
	fall := -move(100, 92) - move(100, 92) - move(96, 88)
	rise := move(92, 100) + move(92, 96)
	assertNear(t, "weight", reading.Weight, (fall-rise)/(fall+rise), 1e-9)
}
