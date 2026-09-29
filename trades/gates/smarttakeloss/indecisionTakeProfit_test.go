package smarttakeloss

import (
	"math"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// declinePrices are the entry prices of the doubling ladders below, falling.
var declinePrices = []float64{79.64, 76.80, 75.11, 72.00}

// shallowDecline is the doubling ladder at its third depth, whose position
// price's take profit sits over its break even and under the average's take
// profit; deepDecline adds a depth far enough under the others that its
// position price's take profit sits under its break even.
func shallowDecline() aggragates.Trades {
	return doublingLadder(declinePrices[:3]...)
}

func deepDecline() aggragates.Trades {
	return doublingLadder(declinePrices...)
}

// A latched trade at or over break even reads the larger of its input and
// the move against its position price: between the position price's take
// profit and the average's, the move against the position price is past the
// take profit and the move against the average entry price is not, so the
// take profit arms at the lower price; the same ladder unlatched reads its
// input, and an input larger than the position price's move comes back.
func TestTakeProfitPercentageReadsThePositionPriceOnALatchedTrade(t *testing.T) {
	plain := shallowDecline()
	latched := latchedBy(plain)
	step := takeProfitStep(latched)
	fromPosition, fromAverage := takeProfitLines(latched)
	average := ladder.AverageEntryPrice(latched)
	if !rebuildState(latched).indecision || !(average < fromPosition && fromPosition < fromAverage) {
		t.Fatal("fixture drifted: the ladder must be latched, its position price's take profit between break even and the average's")
	}
	price := (fromPosition + fromAverage) / 2
	fromAverageMove, fromPositionMove := moveAgainst(price, average), moveAgainst(price, latched.PositionPrice)
	if fromAverageMove < 0 || fromAverageMove >= step || fromPositionMove < step {
		t.Fatalf("fixture drifted: at %v the average's move %v must sit short of the take profit, the position price's %v past it", price, fromAverageMove, fromPositionMove)
	}
	if got := TakeProfitPercentage(latched, price, fromAverageMove); got != fromPositionMove {
		t.Fatalf("a latched trade must read the move against its position price %v, got %v", fromPositionMove, got)
	}
	if got := TakeProfitPercentage(plain, price, fromAverageMove); got != fromAverageMove {
		t.Fatalf("the same ladder unlatched must read its input %v, got %v", fromAverageMove, got)
	}
	larger := fromPositionMove + step
	if got := TakeProfitPercentage(latched, price, larger); got != larger {
		t.Fatalf("an input larger than the position price's move must come back, got %v, want %v", got, larger)
	}
}

// Under break even nothing moves. On a latched ladder whose position price's
// take profit sits under its break even, one float step under break even the
// move against the average entry price is negative and comes back
// unchanged, past the position price's take profit as that price is; at
// break even and one float step over it the latched trade reads the position
// price's move, past the take profit, and the unlatched one its input — so
// the take profit arms at break even and never under it.
func TestTakeProfitPercentageReadsThePositionPriceFromBreakEvenUp(t *testing.T) {
	plain := deepDecline()
	latched := latchedBy(plain)
	breakEven := ladder.AverageEntryPrice(latched)
	step := takeProfitStep(latched)
	if !rebuildState(latched).indecision || moveAgainst(breakEven, latched.PositionPrice) < step {
		t.Fatal("fixture drifted: the latched ladder's position price's take profit must sit under break even")
	}
	under := math.Nextafter(breakEven, 0)
	fromAverage := moveAgainst(under, breakEven)
	if fromAverage >= 0 || moveAgainst(under, latched.PositionPrice) < step {
		t.Fatalf("fixture drifted: one float step under break even the move must be negative past the position price's take profit, got %v", fromAverage)
	}
	for name, trade := range map[string]aggragates.Trades{"latched": latched, "unlatched": plain} {
		if got := TakeProfitPercentage(trade, under, fromAverage); got != fromAverage {
			t.Errorf("%s: under break even the input must come back, got %v, want %v", name, got, fromAverage)
		}
	}
	for name, price := range map[string]float64{"at break even": breakEven, "over break even": math.Nextafter(breakEven, math.Inf(1))} {
		fromAverage := moveAgainst(price, breakEven)
		want := moveAgainst(price, latched.PositionPrice)
		if got := TakeProfitPercentage(latched, price, fromAverage); got != want || got < step {
			t.Errorf("%s: the latched trade must read its position price %v, past the take profit, got %v", name, want, got)
		}
		if got := TakeProfitPercentage(plain, price, fromAverage); got != fromAverage {
			t.Errorf("%s: unlatched the input must come back, got %v, want %v", name, got, fromAverage)
		}
	}
}

// The latched rule reads trade.PositionPrice — the anchor of the `buy` row's
// own percentage, a re-anchor included — not the newest fill. A trade both
// pending and latched reads the largest of its three moves: the newest
// fill's where the position price was re-anchored over it, the position
// price's where it was re-anchored under it. Every product below is rounded
// on its own (an explicit conversion), so the compiler cannot fuse it into
// the subtraction a move makes and the expected moves are the ones the
// helper computes, bit for bit.
func TestTakeProfitPercentageReadsThePositionPriceNotTheNewestFill(t *testing.T) {
	newest := shallowDecline().PositionPrice
	price := float64(ladder.AverageEntryPrice(shallowDecline()) * 1.01)
	over, under := float64(newest*1.01), float64(newest*0.99)

	reanchored := latchedBy(shallowDecline())
	reanchored.PositionPrice = over
	if got, want := TakeProfitPercentage(reanchored, price, 0), moveAgainst(price, over); got != want || got == moveAgainst(price, newest) {
		t.Fatalf("a re-anchored latched trade reads its position price: got %v, want %v", got, want)
	}

	marker := Row{Message: SlowDeclineMessage("buy", slowDeclineReasons), Price: newest}
	both := latchedBy(withRow(shallowDecline(), marker, testutil.At("15:00:00")))
	if st := rebuildState(both); !st.slowDeclinePending || !st.indecision {
		t.Fatalf("fixture drifted: the trade must be pending and latched, got %+v", st)
	}
	for name, tc := range map[string]struct{ positionPrice, anchor float64 }{
		"re-anchored over the newest fill":  {over, newest},
		"re-anchored under the newest fill": {under, under},
	} {
		trade := both
		trade.PositionPrice = tc.positionPrice
		if got, want := TakeProfitPercentage(trade, price, 0), moveAgainst(price, tc.anchor); got != want {
			t.Errorf("%s: got %v, want the largest move %v", name, got, want)
		}
	}
}

// Every trade the indecision row does not latch reads its input: an inverse
// ladder, a child, a futures trade, a ladder one short of IndecisionArmDepth,
// a strategy without the flag, a row without a price — and a latched trade on
// no price at all, and every latched trade while IndecisionDirection is off.
func TestTakeProfitPercentageLeavesTheUnlatchedTradesAlone(t *testing.T) {
	_, fromAverage := takeProfitLines(shallowDecline())
	price := fromAverage - 0.01
	row := latchedBy(shallowDecline()).Logs
	inverse := testutil.LadderTrade(true, fills(watchedFills, "17:38:00")...)
	inverse.Logs = row
	child := latchedBy(shallowDecline())
	child.ParentID = 7
	futures := latchedBy(shallowDecline())
	futures.Strategy.TradeType = aggragates.Futures
	shallow := doublingLadder(declinePrices[:IndecisionArmDepth-1]...)
	shallow.Logs = row
	off := latchedBy(shallowDecline())
	off.Strategy.Params.SmartTakeLoss = false
	unpriced := shallowDecline()
	unpriced.Logs = []aggragates.TradesLogs{{Message: IndecisionMessage("buy", indecisionReasons)}}
	for name, trade := range map[string]aggragates.Trades{
		"an inverse ladder":               inverse,
		"a child":                         child,
		"a futures trade":                 futures,
		"one short of IndecisionArmDepth": shallow,
		"a strategy without the flag":     off,
		"a row without a price":           unpriced,
	} {
		if got := TakeProfitPercentage(trade, price, breakEvenReading); got != breakEvenReading {
			t.Errorf("%s must read its input, got %v", name, got)
		}
	}
	latched := latchedBy(shallowDecline())
	for _, noPrice := range []float64{0, -price} {
		if got := TakeProfitPercentage(latched, noPrice, breakEvenReading); got != breakEvenReading {
			t.Errorf("a price of %v must read the input, got %v", noPrice, got)
		}
	}
	if got := TakeProfitPercentage(latched, price, breakEvenReading); got == breakEvenReading {
		t.Fatal("control: the latched trade itself must read its position price")
	}
	withIndecisionDirection(t, false)
	if got := TakeProfitPercentage(latched, price, breakEvenReading); got != breakEvenReading {
		t.Fatalf("switched off, the latched trade must read its input, got %v", got)
	}
}
