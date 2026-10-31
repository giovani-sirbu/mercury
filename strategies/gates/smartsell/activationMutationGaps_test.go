package smartsell

import (
	"fmt"
	"math"
	"slices"
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// The tests the independent mutation pass on the trailingBuy activation rule (unit T4) found
// missing: each closes a mutant of the production rule that every other test of the package
// let through. They are written to the rule, one theme to a test, from the package's own
// helpers.

// withInverseFill is the inverse trade with one more entry order filled at price: the sell
// that opens a deeper depth of an inverse ladder, one lot of exactUnits like the fills of
// flatMarketLadder, the position at the fill, the way the engines leave a trade after a
// fill. An inverse ladder fills its deeper depths at HIGHER prices.
func withInverseFill(trade aggragates.Trades, price float64, at time.Time) aggragates.Trades {
	trade.History = append(slices.Clone(trade.History), aggragates.TradesHistory{
		Type: "SELL", Quantity: exactUnits, Price: price, OrderId: int64(len(trade.History) + 1), CreatedAt: at,
	})
	trade.PositionPrice = price
	return trade
}

// A new depth resets the flat market episode of an INVERSE ladder as it resets a long
// ladder's, though an inverse ladder fills its deeper depths at HIGHER prices: the episode
// is closed when the newest fill is no longer the one it opened at, whichever way the price
// moved. Whatever the episode held, its sale held or its trailingBuy activated, the reset row
// at the new fill closes it on that tick and the reading, still holding, opens another
// episode on the same tick with its own triggered row, a natural one: a ladder resting in
// buy with its order resting writes nothing more at a print that books a gain, one resting
// in trailingBuy is sold there. A reading that has gone leaves the reset row alone, and the
// tick after, at the same depth, writes no second reset.
func TestFlatMarketResetsTheEpisodeOfAnInverseLadderAtADepthFilledHigher(t *testing.T) {
	flatMarketAlone(t)
	start := flatMarketLadder(true)
	ai, fill := flatMarketMarket(start), start.PositionPrice
	const newest = 104.0
	if newest <= fill {
		t.Fatalf("fixture drifted: the new depth of an inverse ladder fills over the last one, got %v over %v", newest, fill)
	}
	gone := withBlock(flatMarketBlockOf(basePercentage(start), aggragates.SideShort, flatMarketBars, flatMarketHoldMove))
	gain := ladder.AverageEntryPrice(start) / gainFactor

	open := withRows(start, triggeredBy(start, ai))
	held := withRows(open, FlatMarketSaleHeldRow(fill, -0.9))
	activated := committedBy(open, FlatMarketActivatedRow(fill, -0.25))
	for name, episode := range map[string]aggragates.Trades{"open": open, "held": held, "activated": activated} {
		if st := rebuildState(episode); !st.flatMarketActive || st.flatMarketFrom != fill {
			t.Fatalf("%s: fixture drifted: the episode must be open at the last fill, got %+v", name, st)
		}
		deeper := withInverseFill(episode, newest, testutil.At("14:00:00"))
		reset, reopened := FlatMarketResetRow(newest), triggeredBy(deeper, ai)

		got := Apply(withRestingOrder(deeper), "buy", gain, ai)
		if episode.PositionType == "trailingBuy" {
			assertSoldWithAt(t, got, reasonFlatMarket, deeper, gain, reset, reopened)
		} else {
			assertWaits(t, got, "buy", reset, reopened)
		}
		written, _ := engineStep(withRestingOrder(deeper), "buy", gain, testutil.At("14:05:00"), ai)
		if st := rebuildState(written); !st.flatMarketActive || st.flatMarketActivated || st.flatMarketSaleHeld || st.flatMarketFrom != newest {
			t.Fatalf("%s: the new episode is open at the new fill with nothing held or activated, got %+v", name, st)
		}
		assertUntouched(t, Apply(restingIn(written, "buy"), "buy", gain, ai), "buy")

		assertWaits(t, Apply(withRestingOrder(deeper), "buy", gain, gone), "buy", reset)
	}
}

// fractionalLadder is a spot ladder whose entries are a fraction of a unit each, filled an
// hour apart at the prices given, resting in buy: a long ladder of it pays little quote, an
// inverse one sells little base, and the used amount the loss is a share of (the quote the
// entries paid, the base they sold) is under one. The pair's lot size is three decimals, so
// that the close the loss simulates (quantities.SimulatedClose floors it to the lot size) is
// not rounded away to nothing.
func fractionalLadder(inverse bool, quantity float64, prices ...float64) aggragates.Trades {
	trade := testutil.LadderTrade(inverse)
	trade.StrategyPair.TradeFilters = aggragates.TradeFilters{LotSize: 3, PriceFilter: 2}
	entry := "BUY"
	if inverse {
		entry = "SELL"
	}
	for index, price := range prices {
		trade.History = append(trade.History, aggragates.TradesHistory{
			Type: entry, Quantity: quantity, Price: price, OrderId: int64(index + 1),
			CreatedAt: testutil.At(fmt.Sprintf("%02d:00:00", 9+index)),
		})
		trade.PositionPrice = price
	}
	return trade
}

// The profit of a sale is a share of the amount the trade has used, whatever its size: a
// ladder whose entries paid less than a whole unit of quote (a long one) or sold less than
// a whole unit of base (an inverse one) is measured like any other. A gain and a loss within
// the trigger reach it, a loss over it falls short, each profit the share the entries' totals
// state; only a trade that used nothing has nothing to measure.
func TestSaleProfitMeasuresALadderThatUsedLessThanAWholeUnit(t *testing.T) {
	long, inverse := fractionalLadder(false, 0.1, 2.00, 1.90, 1.80), fractionalLadder(true, 0.2, 100, 101, 102)
	if used := ladder.EntryNotional(long); used <= 0 || used >= 1 {
		t.Fatalf("fixture drifted: the long ladder must have paid under one unit of quote, got %v", used)
	}
	if used := ladder.EntryQuantity(inverse); used <= 0 || used >= 1 {
		t.Fatalf("fixture drifted: the inverse ladder must have sold under one unit of base, got %v", used)
	}

	const triggerPct = -2.0
	for _, c := range []struct {
		name     string
		trade    aggragates.Trades
		price    float64
		profit   float64
		standing triggerStanding
	}{
		{"long, at a gain", long, 1.95, -handLossPct(long, 1.95), reached},
		{"long, a loss within the trigger", long, 1.89, -handLossPct(long, 1.89), reached},
		{"long, a loss over the trigger", long, 1.80, -handLossPct(long, 1.80), short},
		{"inverse, at a gain", inverse, 99, -inverseLossPctOf(inverse, 99), reached},
		{"inverse, a loss within the trigger", inverse, 102, -inverseLossPctOf(inverse, 102), reached},
		{"inverse, a loss over the trigger", inverse, 105, -inverseLossPctOf(inverse, 105), short},
		{"a trade that used nothing", testutil.LadderTrade(false), 1.80, 0, nothingUsed},
	} {
		profit, standing := saleProfit(c.trade, c.price, triggerPct)
		if standing != c.standing || math.Abs(profit-c.profit) > 1e-9 {
			t.Errorf("%s: saleProfit at %v = %v (%v), want %v (%v)", c.name, c.price, profit, standing, c.profit, c.standing)
		}
	}
	if longAverage, inverseAverage := ladder.AverageEntryPrice(long), ladder.AverageEntryPrice(inverse); longAverage >= 1.95 || inverseAverage <= 99 {
		t.Fatalf("fixture drifted: the gains must sit past the averages %v and %v", longAverage, inverseAverage)
	}
}

// The flat market gate acts on an inverse ladder that sold less than a whole unit of base
// as on any other: at a print whose loss is within the cap it activates the trailingBuy, with
// the loss in the activation row, and at the first print over the cap it waits with its held
// row.
func TestTheFlatMarketGateActsOnAnInverseLadderThatSoldLessThanAWholeUnit(t *testing.T) {
	flatMarketAlone(t)
	if allowedLoss() <= 0 || allowedLoss() >= 100 {
		t.Skip("SaleTriggerPercentage leaves no price to read the inverse ladder at")
	}
	inverse := fractionalLadder(true, 0.2, 100, 101, 102)
	if used := ladder.EntryQuantity(inverse); used <= 0 || used >= 1 {
		t.Fatalf("fixture drifted: the inverse ladder must have sold under one unit of base, got %v", used)
	}
	ai, fill := flatMarketMarket(inverse), inverse.PositionPrice
	within := math.Floor(ladder.AverageEntryPrice(inverse)/(1-allowedLoss()/100)*100) / 100
	over := within + 0.01
	assertActivated(t, Apply(inverse, "", within, ai), triggeredBy(inverse, ai), FlatMarketActivatedRow(fill, -inverseLossPctOf(inverse, within)))
	assertWaits(t, Apply(inverse, "", over, ai), "", triggeredBy(inverse, ai), FlatMarketSaleHeldRow(fill, -inverseLossPctOf(inverse, over)))
}

// cheapLadder is a long spot ladder on a pair priced well under one, the price of a cheap
// coin: four whole-unit fills at cents of a unit, the pair's price filter at five decimals
// so that the cap names a price on its grid, the position at the newest fill, resting in
// buy. The quote it paid is large (thousands of units of the pair), only the price is small.
func cheapLadder() aggragates.Trades {
	trade := testutil.LadderTrade(false)
	trade.StrategyPair.TradeFilters = aggragates.TradeFilters{LotSize: 0, PriceFilter: 5}
	for index, price := range []float64{0.30, 0.29, 0.28, 0.27} {
		trade.History = append(trade.History, aggragates.TradesHistory{
			Type: "BUY", Quantity: 1000, Price: price, OrderId: int64(index + 1),
			CreatedAt: testutil.At(fmt.Sprintf("%02d:00:00", 9+index)),
		})
		trade.PositionPrice = price
	}
	return trade
}

// The overlay is not only for pairs priced over one: a pair priced under one (a coin of a few
// cents) is acted on like any other. The flat market gate activates the trailingBuy of its
// ladder at a print within the cap, waits with its held row at the first price over it, and
// sells the ladder resting in trailingBuy at the print within the cap, the prices being those
// of the pair's own grid.
func TestAPairPricedUnderOneIsActedOnLikeAnyOther(t *testing.T) {
	flatMarketAlone(t)
	if allowedLoss() <= 0 || allowedLoss() >= 100 {
		t.Skip("SaleTriggerPercentage leaves no price to read the ladder at")
	}
	trade := cheapLadder()
	ai, fill := flatMarketMarket(trade), trade.PositionPrice
	used, units, average := ladder.EntryNotional(trade), ladder.EntryQuantity(trade), ladder.AverageEntryPrice(trade)
	lossAt := func(price float64) float64 { return (used - units*price) / used * 100 }
	// over is the first price on the pair's grid, walking down from the break even, whose loss is over the
	// cap by a margin; within is two steps above it, so that neither sits on the edge of the cap.
	over := average
	for lossAt(over) <= allowedLoss()+1e-9 {
		over = math.Round((over-1e-5)*1e5) / 1e5
	}
	within := math.Round((over+2e-5)*1e5) / 1e5
	if within >= 1 || lossAt(within) <= 0 || lossAt(within) >= allowedLoss()-1e-9 {
		t.Fatalf("fixture drifted: %v must be a price under one losing something within the cap and %v one over it", within, over)
	}

	assertActivated(t, Apply(trade, "", within, ai), triggeredBy(trade, ai), FlatMarketActivatedRow(fill, -lossAt(within)))
	assertWaits(t, Apply(trade, "", over, ai), "", triggeredBy(trade, ai), FlatMarketSaleHeldRow(fill, -lossAt(over)))
	assertSoldWith(t, Apply(restingIn(trade, "trailingBuy"), "", within, ai), reasonFlatMarket, triggeredBy(trade, ai))
}

// The early take profit still latches a ladder it alone watches: with the quiet decline,
// the swing decline, capital protection and the flat market gate all off, no other gate
// has a reason to tick the ladder, and the sophos reading of the indecision still lands on
// its row, beside the proposal it leaves untouched.
func TestTheEarlyTakeProfitDirectionStillLatchesALadderNoOtherGateWatches(t *testing.T) {
	withQuietDecline(t, false)
	withSwingDecline(t, false)
	withCapitalProtectionExit(t, false)
	withFlatMarket(t, false)
	withEarlyTakeProfit(t, true)
	trade := indecisionLadder()
	if st := rebuildState(trade); !st.earlyTakeProfitWatched || st.slowDeclineWatched || st.capitalProtectionWatched || st.flatMarketWatched {
		t.Fatalf("fixture drifted: the early take profit alone must watch the ladder, got %+v", st)
	}
	got := Apply(trade, "", underTheBand, indecisionReading())
	assertRow(t, got, LatchedRow(trade.PositionPrice, indecisionFinding(indecisionReasons)))
	assertNoSale(t, got, "")
}

// The flat market switch is a switch: off, the gate opens no episode and writes no row on a
// ladder another gate watches either, whatever the market it is served. The indecision
// direction watches this ladder and the market holds for the flat market reading, so with
// the switch on the tick writes the latch and the triggered row, and with it off the latch
// alone.
func TestTheFlatMarketSwitchOffKeepsTheGateSilentOnALadderAnotherGateWatches(t *testing.T) {
	withQuietDecline(t, false)
	withSwingDecline(t, false)
	withCapitalProtectionExit(t, false)
	withEarlyTakeProfit(t, true)
	trade := indecisionLadder()
	if name := noFlatMarketMarket(basePercentage(trade)); name != "" {
		t.Skipf("%s leaves no market that holds for the fixture ladder to be read under", name)
	}
	block := flatMarketBlock(basePercentage(trade), aggragates.EntrySide(trade, aggragates.AIIndicators{}))
	block.QuietDeclineFillFrom, block.QuietDeclineBreakReasons, block.Indecision = fillWindowFrom.UnixMilli(), indecisionReasons, true
	ai, gain := withBlock(block), gainPrint(trade)
	latch := LatchedRow(trade.PositionPrice, indecisionFinding(indecisionReasons))

	withFlatMarket(t, true)
	if !flatMarketReading(trade, block, ai).Holds || !rebuildState(trade).earlyTakeProfitWatched {
		t.Fatalf("fixture drifted: the early take profit must watch the ladder and the market hold for the flat market reading")
	}
	assertWaits(t, Apply(withRestingOrder(trade), "", gain, ai), "", latch, triggeredBy(trade, ai))

	withFlatMarket(t, false)
	assertWaits(t, Apply(withRestingOrder(trade), "", gain, ai), "", latch)
}
