package smarttakeloss

import (
	"fmt"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// solBlock is the block sophos serves on the tape of SOL 45211 (28 Dec 2021):
// over its window of closed daily bars, 181.20 is the price with at most
// MaxBarsLeft bars under it and 179.75 the price with none at all. No line:
// the band is the only target. The high levels sit far above the ladder, so
// the same block never activates an inverse trade.
func solBlock() aggragates.SmartTakeLossIndicators {
	return aggragates.SmartTakeLossIndicators{
		HasVerdict:       true,
		LowestLow:        179.75,
		LowWithBarsLeft:  181.20,
		HighestHigh:      195.10,
		HighWithBarsLeft: 193.40,
		UpperBB:          193.83,
		LowerBB:          176.40,
	}
}

// risingBlock is the same shape on the synthetic inverse ladder's scale
// (fills 100 to 110): 106 is the price with at most MaxBarsLeft bars OVER it
// and 108.50 the price with none. Its low levels sit far under that ladder,
// so it never activates a long.
func risingBlock() aggragates.SmartTakeLossIndicators {
	return aggragates.SmartTakeLossIndicators{
		HasVerdict:       true,
		LowestLow:        90,
		LowWithBarsLeft:  92,
		HighestHigh:      108.50,
		HighWithBarsLeft: 106,
		UpperBB:          112,
		LowerBB:          96,
	}
}

// fills is the w3s ladder n deep: the first n−1 fills a quarter hour apart
// from 08:00, the newest stamped at lastClock.
func fills(n int, lastClock string) []testutil.LadderFill {
	clocks := make([]string, n)
	for index := range clocks {
		clocks[index] = fmt.Sprintf("%02d:%02d:00", 8+index/4, (index%4)*15)
	}
	clocks[n-1] = lastClock
	return testutil.W3sFills(clocks...)
}

// risingFills is an inverse ladder n deep, SELLs from 100 up by 2, stamped
// like fills.
func risingFills(n int, lastClock string) []testutil.LadderFill {
	stamped := fills(n, lastClock)
	for index := range stamped {
		stamped[index].Price = 100 + 2*float64(index)
	}
	return stamped
}

// The activation reads the price against the level where the window still
// allows MaxBarsLeft bars under it. Touching it is enough; a cent above is
// one bar too many.
func TestActivatesAtTheLevelWithBarsStillToTheLeft(t *testing.T) {
	trade := testutil.LadderTrade(false, fills(5, "17:38:00")...)
	st := rebuildState(trade)
	block := solBlock()

	cases := []struct {
		name  string
		price float64
		want  bool
	}{
		{"at the level", block.LowWithBarsLeft, true},
		{"under it", block.LowWithBarsLeft - 0.01, true},
		{"under everything the window holds", block.LowestLow - 5, true},
		{"a cent over the level", block.LowWithBarsLeft + 0.01, false},
		{"far over it", 190, false},
		{"no price", 0, false},
	}
	for _, c := range cases {
		if got := activates(trade, st, c.price, block); got != c.want {
			t.Errorf("%s (%.2f): activates = %v, want %v", c.name, c.price, got, c.want)
		}
	}
}

// The reading is the tick's, not a fill's: a ladder whose funds ran out hours
// ago activates on the tick the price arrives. This is the whole point of
// reading it per tick — the trades this rule exists for have stopped filling.
func TestActivatesOnAnyTickNotOnlyAtAFill(t *testing.T) {
	stale := testutil.LadderTrade(false, fills(5, "08:00:00")...)
	if !activates(stale, rebuildState(stale), 180, solBlock()) {
		t.Fatal("a ladder that stopped filling still activates when the price arrives")
	}

	// The stamp is the exit's business (MinAgeAfterLastFill), never the
	// activation's: the activation refuses nothing on its tick.
	unstamped := testutil.LadderTrade(false, fills(5, "17:38:00")...)
	unstamped.History[4].CreatedAt = time.Time{}
	if !activates(unstamped, rebuildState(unstamped), 180, solBlock()) {
		t.Fatal("a fill without a stamp does not stop the activation")
	}
}

// A trade with no fill to anchor the permitted depth to, a block without a
// verdict and a block without the level never activate: the cooldown's
// fail-open posture.
func TestActivatesNeverWithoutAFillAVerdictOrALevel(t *testing.T) {
	trade := testutil.LadderTrade(false, fills(5, "17:38:00")...)
	st := rebuildState(trade)

	if activates(trade, st, 180, aggragates.SmartTakeLossIndicators{}) {
		t.Fatal("the zero block must not activate")
	}
	noVerdict := solBlock()
	noVerdict.HasVerdict = false
	if activates(trade, st, 180, noVerdict) {
		t.Fatal("a block without a verdict must not activate")
	}
	noLevel := solBlock()
	noLevel.LowWithBarsLeft = 0
	if activates(trade, st, 180, noLevel) {
		t.Fatal("a block without the level must not activate")
	}
	if activates(trade, state{}, 180, solBlock()) {
		t.Fatal("a ladder with no fill has nothing to count the permitted depth from")
	}
}

func TestActivatesInverseOnTheHighLevel(t *testing.T) {
	inverse := testutil.LadderTrade(true, risingFills(5, "17:38:00")...)
	st := rebuildState(inverse)
	block := risingBlock()

	if !activates(inverse, st, block.HighWithBarsLeft, block) {
		t.Fatal("an inverse ladder activates at the level with bars still over it")
	}
	if activates(inverse, st, block.HighWithBarsLeft-0.01, block) {
		t.Fatal("a cent under the level is one bar too many for an inverse ladder")
	}
	if activates(inverse, st, 179.90, solBlock()) {
		t.Fatal("the long's low levels are not the inverse ladder's signal")
	}

	long := testutil.LadderTrade(false, fills(5, "17:38:00")...)
	if activates(long, rebuildState(long), 107, block) {
		t.Fatal("the inverse ladder's high levels are not the long's signal")
	}
}

// The second reading of the same window: no bar to the left AT ALL, which is
// what arms the tolerance exit.
func TestNoBarsLeftAtTheWindowsOwnExtreme(t *testing.T) {
	long := testutil.LadderTrade(false, fills(5, "17:38:00")...)
	block := solBlock()

	if !noBarsLeft(long, block.LowestLow, block) {
		t.Fatal("the window's lowest low has no bar under it")
	}
	if !noBarsLeft(long, block.LowestLow-1, block) {
		t.Fatal("a price under the window has no bar under it either")
	}
	if noBarsLeft(long, block.LowestLow+0.01, block) {
		t.Fatal("a cent over the lowest low still has a bar to the left")
	}
	// Between the two levels is exactly where the activation stands and the
	// tolerance exit does not.
	if !activates(long, rebuildState(long), 180.50, block) || noBarsLeft(long, 180.50, block) {
		t.Fatal("between the levels the trade is active and the tolerance is not armed")
	}

	inverse := testutil.LadderTrade(true, risingFills(5, "17:38:00")...)
	rising := risingBlock()
	if !noBarsLeft(inverse, rising.HighestHigh, rising) || noBarsLeft(inverse, rising.HighestHigh-0.01, rising) {
		t.Fatal("an inverse ladder mirrors the reading on the window's highest high")
	}

	if noBarsLeft(long, block.LowestLow, aggragates.SmartTakeLossIndicators{}) {
		t.Fatal("the zero block arms nothing")
	}
	noVerdict := solBlock()
	noVerdict.HasVerdict = false
	if noBarsLeft(long, block.LowestLow, noVerdict) {
		t.Fatal("a block without a verdict arms nothing")
	}
}
