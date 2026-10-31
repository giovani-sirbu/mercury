package dynamicparams_test

import (
	"testing"

	"github.com/giovani-sirbu/mercury/strategies/gates/dynamicparams"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// pinFreeFall sets the free fall knobs for the test and restores the ones it found when it ends, so no
// test reads the shipped values.
func pinFreeFall(t *testing.T, minimum int, timeframes ...string) {
	t.Helper()

	wasMinimum, wasTimeframes := dynamicparams.MinFreeFallTimeframes, dynamicparams.FreeFallTimeframes
	t.Cleanup(func() {
		dynamicparams.MinFreeFallTimeframes, dynamicparams.FreeFallTimeframes = wasMinimum, wasTimeframes
	})

	dynamicparams.MinFreeFallTimeframes, dynamicparams.FreeFallTimeframes = minimum, timeframes
}

// inFreeFall is the block with these row labels in free fall, comma-joined the way the decode writes them.
func inFreeFall(reads aggragates.DynamicParamsIndicators, labels string) aggragates.DynamicParamsIndicators {
	reads.FreeFall = labels
	return reads
}

// At or below zero the trigger is off, however many rows read in free fall and whatever the list names.
func TestFreeFallIsOffAtOrBelowZero(t *testing.T) {
	for _, minimum := range []int{0, -1} {
		for _, timeframes := range [][]string{nil, {"1D", "W"}} {
			pinFreeFall(t, minimum, timeframes...)

			if dynamicparams.FreeFall(aggragates.DynamicParamsIndicators{FreeFall: "1D,W,M"}) {
				t.Errorf("minimum %d, list %v: FreeFall = true, want the trigger off", minimum, timeframes)
			}
		}
	}
}

// A row counts when the list names its timeframe, or when the list names none; the trigger fires at
// the minimum exactly, not a row short of it.
func TestFreeFallCountsTheRowsTheListNames(t *testing.T) {
	cases := []struct {
		name       string
		minimum    int
		timeframes []string
		freeFall   string
		want       bool
	}{
		{"an empty list counts any row", 2, nil, "4h,1h", true},
		{"an empty list counts one row short of the minimum", 2, nil, "4h", false},
		{"a list counts the rows it names", 2, []string{"1D", "W"}, "1D,W", true},
		{"a list does not count a row it does not name", 2, []string{"1D", "W"}, "1D,M", false},
		{"a list counts its rows beside one it does not name", 2, []string{"1D", "W"}, "M,1D,W", true},
		{"a list counts none of the rows it does not name", 1, []string{"1D", "W"}, "M,4h", false},
		{"a listed timeframe the payload does not serve never counts", 3, []string{"1D", "W", "M"}, "1D,W", false},
		{"a lower case row counts for an upper case entry", 1, []string{"1D"}, "1d", true},
		{"an upper case row counts for a lower case entry", 1, []string{"w"}, "W", true},
		{"a duplicate entry does not count a row twice", 2, []string{"1D", "1D"}, "1D", false},
		{"a duplicate entry in another case does not count a row twice", 2, []string{"1D", "1d", "W"}, "1D", false},
		{"a duplicate entry still counts the other listed rows", 2, []string{"1D", "1D", "W"}, "1D,W", true},
		{"exactly the minimum", 3, []string{"1D", "W", "M"}, "1D,W,M", true},
		{"one row short of the minimum", 3, []string{"1D", "W", "M"}, "1D,W", false},
		{"above the minimum", 2, []string{"1D", "W", "M"}, "1D,W,M", true},
		{"a minimum of one", 1, []string{"1D"}, "1D", true},
		{"no row in free fall", 1, nil, "", false},
		{"no row in free fall under a list", 1, []string{"1D", "W"}, "", false},
		{"a stray comma around a row", 1, []string{"W"}, ",W,", true},
		{"stray commas between rows", 2, []string{"1D", "W"}, ",1D,,W,", true},
		{"stray commas are not rows under an empty list", 1, nil, ",,", false},
		{"stray commas are not rows beside the minimum", 2, nil, ",W,", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pinFreeFall(t, c.minimum, c.timeframes...)

			reads := aggragates.DynamicParamsIndicators{FreeFall: c.freeFall}
			if got := dynamicparams.FreeFall(reads); got != c.want {
				t.Errorf("minimum %d, list %v, free fall %q: FreeFall = %v, want %v", c.minimum, c.timeframes, c.freeFall, got, c.want)
			}
		})
	}
}

// The count reads the free fall rows alone: the two reads and whether the chart row was read do not
// enter it.
func TestFreeFallIgnoresTheTwoReads(t *testing.T) {
	pinFreeFall(t, 2, "1D", "W")

	for name, reads := range map[string]aggragates.DynamicParamsIndicators{
		"both bearish": bothBearish,
		"mixed":        mixedRead,
		"not bearish":  notBearish,
		"not read":     notRead,
		"the zero":     {},
	} {
		if !dynamicparams.FreeFall(inFreeFall(reads, "1D,W")) {
			t.Errorf("%s: FreeFall = false with both rows in free fall, want true", name)
		}
		if dynamicparams.FreeFall(inFreeFall(reads, "1D")) {
			t.Errorf("%s: FreeFall = true with one row in free fall, want false", name)
		}
		if dynamicparams.FreeFall(inFreeFall(reads, "")) {
			t.Errorf("%s: FreeFall = true with no row in free fall, want false", name)
		}
	}
}
