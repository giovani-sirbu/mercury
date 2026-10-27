package flatmarket

import (
	"math"
	"slices"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// assertUnread fails unless bars that are not served leave a reading nothing to hold
// on, on each side: a miss with no text whose breaks name the bars alone.
func assertUnread(t *testing.T, name string, bars Bars) {
	t.Helper()
	for _, sc := range rangeSideCases {
		if reading := TriggerRange(bars, fixturePercentage, sc.side, sc.met); reading.Holds || reading.Reasons != nil {
			t.Errorf("%s, %s: want a miss with no text, got %+v", name, sc.name, reading)
		}
		if got := RangeBreaks(bars, fixturePercentage, sc.side, sc.met); !slices.Equal(got, []string{"bars not served"}) {
			t.Errorf("%s, %s: breaks %q, want the bars alone", name, sc.name, got)
		}
	}
}

// BarsOf hands the five arrays of the block over one to one, every array holding
// numbers of its own so that a swapped key shows, and copies none of them.
func TestBarsOfReadsTheFiveArraysOneToOneAndAllocatesNothing(t *testing.T) {
	block := aggragates.SmartTakeLossIndicators{
		FlatMarketBarOpens:   []int64{10, 20},
		FlatMarketOpenPrices: []float64{1, 2},
		FlatMarketHighs:      []float64{3, 4},
		FlatMarketLows:       []float64{5, 6},
		FlatMarketCloses:     []float64{7, 8},
	}
	bars := BarsOf(block)
	if &bars.Opens[0] != &block.FlatMarketBarOpens[0] || &bars.OpenPrices[0] != &block.FlatMarketOpenPrices[0] ||
		&bars.Highs[0] != &block.FlatMarketHighs[0] || &bars.Lows[0] != &block.FlatMarketLows[0] ||
		&bars.Closes[0] != &block.FlatMarketCloses[0] {
		t.Errorf("each array must be the block's own, got %+v", bars)
	}
	if allocs := testing.AllocsPerRun(100, func() { BarsOf(block) }); allocs != 0 {
		t.Errorf("BarsOf allocated %v times, want none", allocs)
	}
	if empty := BarsOf(aggragates.SmartTakeLossIndicators{}); empty.Served() || empty.Len() != 0 {
		t.Errorf("a block with no bars serves none, got %+v", empty)
	}
}

// Bars are served only with the five arrays of one length, at least the fewest bars
// the rule asks and every price above zero and finite. Len is the length the arrays
// agree on and LastBarOpen the open time of the newest bar served: both read nothing
// of bars that are not.
func TestServedNeedsFiveArraysOfOneLengthEnoughBarsAndPricesAboveZero(t *testing.T) {
	good := holding(aggragates.SideLong)
	if !good.Served() || good.Len() != rangeFixtureBars || good.LastBarOpen() != good.Opens[rangeFixtureBars-1] {
		t.Fatalf("a window of bars is served, got served %v, len %d, last %d", good.Served(), good.Len(), good.LastBarOpen())
	}
	cut := func(count int) Bars {
		return Bars{good.Opens[:count], good.OpenPrices[:count], good.Highs[:count], good.Lows[:count], good.Closes[:count]}
	}
	if !cut(MinBars).Served() || cut(MinBars-1).Served() || cut(MinBars-1).LastBarOpen() != 0 {
		t.Errorf("the fewest bars served is %d (MinBars)", MinBars)
	}
	n := rangeFixtureBars - 1
	for name, bars := range map[string]Bars{
		"opens":       {good.Opens[:n], good.OpenPrices, good.Highs, good.Lows, good.Closes},
		"open prices": {good.Opens, good.OpenPrices[:n], good.Highs, good.Lows, good.Closes},
		"highs":       {good.Opens, good.OpenPrices, good.Highs[:n], good.Lows, good.Closes},
		"lows":        {good.Opens, good.OpenPrices, good.Highs, good.Lows[:n], good.Closes},
		"closes":      {good.Opens, good.OpenPrices, good.Highs, good.Lows, good.Closes[:n]},
	} {
		if bars.Served() || bars.Len() != 0 || bars.LastBarOpen() != 0 {
			t.Errorf(
				"arrays that disagree on the %s are not served: served %v, len %d, last %d",
				name, bars.Served(), bars.Len(), bars.LastBarOpen(),
			)
		}
		assertUnread(t, "arrays that disagree on the "+name, bars)
	}
	for which, name := range []string{"open price", "high", "low", "close"} {
		for _, index := range []int{0, rangeFixtureBars / 2, rangeFixtureBars - 1} {
			for _, price := range []float64{0, -1, math.NaN(), math.Inf(1)} {
				bars := holding(aggragates.SideLong)
				[][]float64{bars.OpenPrices, bars.Highs, bars.Lows, bars.Closes}[which][index] = price
				if bars.Served() || bars.LastBarOpen() != 0 || bars.Len() != rangeFixtureBars {
					t.Errorf("a %s of %v at bar %d must leave the bars unserved but their length", name, price, index)
				}
				assertUnread(t, "a "+name+" that is not above zero", bars)
			}
		}
	}
}

// A body is the distance from the open to the close, up or down alike, in percent of
// the open. Bodies just under and just over the limit, in both directions, tell a
// body read against the open from one read against the close, and a distance from a
// signed difference. The body at the limit is a quarter of the open, a size at which
// the division is exact, and is not under it.
func TestABodyIsReadAgainstTheOpenUpOrDown(t *testing.T) {
	limit := BodyShare * fixturePercentage
	under, over := limit*(1-limit/200), limit*(1+limit/200)
	bars := Bars{
		OpenPrices: []float64{basePrice, basePrice, basePrice, basePrice},
		Closes: []float64{
			basePrice * (1 + under/100), basePrice * (1 - under/100),
			basePrice * (1 + over/100), basePrice * (1 - over/100),
		},
	}
	if got := bars.quietBodies(limit); got != 2 {
		t.Errorf("only the two bars just under the limit are quiet, got %d", got)
	}
	if got := bars.quietBodies(math.Inf(1)); got != len(bars.Closes) {
		t.Errorf("every body is under an unbounded limit, got %d", got)
	}
	atLimit := Bars{OpenPrices: []float64{basePrice}, Closes: []float64{1.25 * basePrice}}
	if atLimit.quietBodies(25) != 0 || atLimit.quietBodies(math.Nextafter(25, math.Inf(1))) != 1 {
		t.Errorf("a body is quiet under the limit, not at it")
	}
}

// The move is read from the oldest and the newest bar alone, and from one field of
// each: nothing the side does not read changes it, whatever the bars between hold.
func TestTheMoveIsReadFromTheOldestAndNewestBarsAlone(t *testing.T) {
	for _, tc := range []struct {
		name                                                  string
		side                                                  string
		oldestHigh, oldestLow, newestHigh, newestLow, wantPct float64
	}{
		{"a fall", aggragates.SideLong, 100, 90, 120, 96, 4},
		{"a long ladder's market going the other way", aggragates.SideLong, 100, 90, 120, 104, -4},
		{"a rise", aggragates.SideShort, 120, 80, 84, 70, 5},
		{"an inverse ladder's market going the other way", aggragates.SideShort, 120, 100, 96, 70, -4},
	} {
		bars := Bars{
			Opens:      []int64{1, 2, 3},
			OpenPrices: []float64{basePrice, 1, basePrice},
			Highs:      []float64{tc.oldestHigh, 1e9, tc.newestHigh},
			Lows:       []float64{tc.oldestLow, 1e-9, tc.newestLow},
			Closes:     []float64{1e9, 1e-9, 1e9},
		}
		assertNear(t, tc.name, bars.movePct(tc.side), tc.wantPct, 1e-9)
	}
	if got := holding(aggragates.SideLong).movePct(""); !math.IsNaN(got) || moveBounded(got) {
		t.Errorf("a side other than the two has no move and never holds, got %v", got)
	}
}

// assertNear fails unless got is within tolerance of want.
func assertNear(t *testing.T, name string, got, want, tolerance float64) {
	t.Helper()
	if math.Abs(got-want) > tolerance {
		t.Errorf("%s = %v, want %v within %v", name, got, want, tolerance)
	}
}

// moveOf is the move of the bars on the side, written out against the rule, so that a
// reason names the number the measurement made and not the one the window was asked,
// which a rounding step could tell apart.
func moveOf(bars Bars, side string) float64 {
	last := len(bars.Opens) - 1
	if side == aggragates.SideShort {
		return (bars.Highs[last] - bars.Lows[0]) / bars.Lows[0] * 100
	}
	return (bars.Highs[0] - bars.Lows[last]) / bars.Highs[0] * 100
}

// The quiet bodies must be MORE than the share: where the share lands on a whole
// number of bars, that many bars miss and one more holds. The move must be UNDER its
// bound, which itself is not (the stray's bound is the other way, see stray_test.go).
func TestTheQuietShareAndTheMoveBoundAreExcluded(t *testing.T) {
	needsAMove(t)
	just := math.Nextafter(MaxMovePct, math.Inf(-1))
	if !moveBounded(just) || moveBounded(MaxMovePct) || moveBounded(math.NaN()) {
		t.Errorf("a move holds under MaxMovePct only: the bound and a move that is no number miss")
	}
	of, found := exactBars()
	whole := int(math.Round(QuietShare * float64(of)))
	if !found || whole < 0 {
		t.Skip("QuietShare lands on no whole number of bars, from none up, for a window up to two hundred bars")
	}
	for _, sc := range rangeSideCases {
		for _, tc := range []struct {
			quiet int
			holds bool
		}{{whole, false}, {whole + 1, true}} {
			bars := window{of: of, quiet: tc.quiet, movePct: holdMove, side: sc.side}.build(fixturePercentage)
			if tc.quiet <= of && TriggerRange(bars, fixturePercentage, sc.side, sc.met).Holds != tc.holds {
				t.Errorf("%s: %d quiet bars of %d must hold %v", sc.name, tc.quiet, of, tc.holds)
			}
		}
	}
}
