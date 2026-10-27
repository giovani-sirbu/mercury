package freefallprevention

import (
	"reflect"
	"slices"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// barMs is the span of one bar of Timeframe in ms; the fixture bars open that far apart.
const barMs = int64(60 * 60 * 1000)

// barsOf is a series served: one bar per mask, opening an hour apart from a fixed start.
func barsOf(fall, rise []int64) Series {
	opens := make([]int64, len(fall))
	for index := range opens {
		opens[index] = 1_640_995_200_000 + int64(index)*barMs
	}
	return Series{Opens: opens, FreeFall: fall, FreeRise: rise}
}

// run is count bars with the same mask.
func run(count int, mask int64) []int64 {
	masks := make([]int64, count)
	for index := range masks {
		masks[index] = mask
	}
	return masks
}

// blockOf is the block sophos serves the series in.
func blockOf(bars Series) aggragates.SmartTakeLossIndicators {
	return aggragates.SmartTakeLossIndicators{FreeFallOpens: bars.Opens, FreeFallMasks: bars.FreeFall, FreeRiseMasks: bars.FreeRise}
}

// The series is the block's three arrays as decoded, each under its own name and none copied: the
// open times, the free fall masks and the free rise masks are told apart, and a block that served
// none hands a series with none.
func TestSeriesOfIsTheBlocksArraysAsDecoded(t *testing.T) {
	block := aggragates.SmartTakeLossIndicators{
		FreeFallOpens: []int64{10, 20}, FreeFallMasks: []int64{1, 2}, FreeRiseMasks: []int64{4, 5},
	}
	got := SeriesOf(block)
	if !slices.Equal(got.Opens, []int64{10, 20}) || !slices.Equal(got.FreeFall, []int64{1, 2}) || !slices.Equal(got.FreeRise, []int64{4, 5}) {
		t.Fatalf("each array under its own name, got %+v", got)
	}
	if &got.Opens[0] != &block.FreeFallOpens[0] || &got.FreeFall[0] != &block.FreeFallMasks[0] || &got.FreeRise[0] != &block.FreeRiseMasks[0] {
		t.Fatal("the series is the block's arrays as decoded, not a copy")
	}
	if none := SeriesOf(aggragates.SmartTakeLossIndicators{}); none.Served() || none.Opens != nil || none.FreeFall != nil || none.FreeRise != nil {
		t.Fatalf("a block that served none hands a series not served, got %+v", none)
	}
}

// A series is served with at least one bar and the three arrays of one length: no bar, a missing
// array and arrays of unequal lengths cannot be read, in whichever array the length differs.
func TestASeriesIsServedWithAtLeastOneBarAndThreeArraysOfOneLength(t *testing.T) {
	for _, c := range []struct {
		name   string
		bars   Series
		served bool
	}{
		{"one bar", Series{Opens: []int64{1}, FreeFall: []int64{0}, FreeRise: []int64{0}}, true},
		{"three bars", barsOf(run(3, 1), run(3, 0)), true},
		{"nothing", Series{}, false},
		{"empty arrays", Series{Opens: []int64{}, FreeFall: []int64{}, FreeRise: []int64{}}, false},
		{"masks with no bar opens", Series{FreeFall: []int64{1}, FreeRise: []int64{1}}, false},
		{"bar opens alone", Series{Opens: []int64{1}}, false},
		{"the free fall short", Series{Opens: []int64{1, 2}, FreeFall: []int64{1}, FreeRise: []int64{1, 2}}, false},
		{"the free rise short", Series{Opens: []int64{1, 2}, FreeFall: []int64{1, 2}, FreeRise: []int64{1}}, false},
		{"the bar opens short", Series{Opens: []int64{1}, FreeFall: []int64{1, 2}, FreeRise: []int64{1, 2}}, false},
		{"the free rise missing", Series{Opens: []int64{1}, FreeFall: []int64{1}}, false},
	} {
		if got := c.bars.Served(); got != c.served {
			t.Errorf("%s: served %v, want %v", c.name, got, c.served)
		}
	}
}

// Tail is the newest n bars of all three arrays alike, not copied; all of them when the series has
// n or fewer or n asks nothing, and the series itself when it is not served.
func TestTailIsTheNewestBarsOfAllThreeArrays(t *testing.T) {
	bars := Series{
		Opens:    []int64{1, 2, 3, 4, 5},
		FreeFall: []int64{11, 12, 13, 14, 15},
		FreeRise: []int64{21, 22, 23, 24, 25},
	}
	got := bars.Tail(2)
	if !slices.Equal(got.Opens, []int64{4, 5}) || !slices.Equal(got.FreeFall, []int64{14, 15}) || !slices.Equal(got.FreeRise, []int64{24, 25}) {
		t.Fatalf("the newest two bars of each array, got %+v", got)
	}
	if &got.Opens[0] != &bars.Opens[3] || &got.FreeFall[0] != &bars.FreeFall[3] || &got.FreeRise[0] != &bars.FreeRise[3] {
		t.Fatal("the tail is a view of the series, not a copy")
	}
	if one := bars.Tail(1); !slices.Equal(one.Opens, []int64{5}) || !slices.Equal(one.FreeFall, []int64{15}) || !slices.Equal(one.FreeRise, []int64{25}) {
		t.Fatalf("the newest bar alone, got %+v", one)
	}
	for _, n := range []int{5, 6, 1000, 0, -1} {
		if got := bars.Tail(n); !reflect.DeepEqual(got, bars) {
			t.Errorf("Tail(%d) is all five bars (n at or over the count, or asking nothing), got %+v", n, got)
		}
	}

	unserved := Series{Opens: []int64{1, 2, 3}, FreeFall: []int64{1, 2}, FreeRise: []int64{1, 2, 3}}
	if got := unserved.Tail(1); !reflect.DeepEqual(got, unserved) {
		t.Errorf("a series not served is handed back whole, got %+v", got)
	}
	if got := (Series{}).Tail(240); got.Served() || got.Opens != nil {
		t.Errorf("no series stays no series, got %+v", got)
	}
}

// A ladder reads the masks of its own way: the free fall masks for a long ladder
// (aggragates.SideLong), the free rise masks for an inverse one (aggragates.SideShort), none for a
// side the gate does not read.
func TestMasksAreTheLaddersOwnWay(t *testing.T) {
	bars := barsOf([]int64{1, 2}, []int64{4, 8})
	if got := bars.Masks(aggragates.SideLong); !slices.Equal(got, []int64{1, 2}) {
		t.Errorf("a long ladder reads the free fall masks, got %v", got)
	}
	if got := bars.Masks(aggragates.SideShort); !slices.Equal(got, []int64{4, 8}) {
		t.Errorf("an inverse ladder reads the free rise masks, got %v", got)
	}
	for _, side := range []string{"", "flat", "LONG"} {
		if got := bars.Masks(side); got != nil {
			t.Errorf("side %q reads no masks, got %v", side, got)
		}
	}
}
