package ladder

import (
	"testing"

	"github.com/giovani-sirbu/mercury/helpers"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// The add: the last filled quantity times the multiplier of the row the NEXT
// entry reads, at the position price — the same arithmetic the funds gate
// sizes that entry with. A row early or late is a different order, and the
// gate would then hold an entry for an amount the engine never spends.
func TestNextEntryCostMultipliesTheLastFillByTheNextRow(t *testing.T) {
	const price = 10

	rows := rowPerDepthRows(2, 3, 4, 5)
	trade := costLadderTrade(2, price, rows...)

	// Two entries filled, the second grown by the second row; the third reads
	// the third row.
	asset, cost := NextEntryCost(trade, 0)
	if asset != "USDT" {
		t.Errorf("asset = %q, want the quote side of the pair", asset)
	}
	testutil.AssertFloatEqual(t, cost, 3*4*price, costEpsilon, "cost of the next depth")
}

// A long add is fitted to the pair's lot size the way the funds gate fits the
// amount it is about to spend, and fitted DOWNWARD: the reserve weighs the
// entry at what the engine actually commits, never a hair above it. The two
// readings have to move together — an unmirrored fit is a difference nobody
// notices until the fit changes.
func TestNextEntryCostFitsALongAddToTheLotSize(t *testing.T) {
	const price = 10.07
	const lotSize = 1

	rows := rowPerDepthRows(2, 3, 4, 5)
	trade := costLadderTrade(2, price, rows...)
	trade.StrategyPair.TradeFilters.LotSize = lotSize

	unfitted := 3 * 4 * price
	_, cost := NextEntryCost(trade, 0)

	testutil.AssertFloatEqual(t, cost, helpers.ToFixed(unfitted, lotSize), costEpsilon, "cost of a fitted next depth")
	if cost >= unfitted {
		t.Fatalf("cost = %f, want the lot size to fit it below the raw %f", cost, unfitted)
	}
}

// An inverse add is a base quantity and is not taken to the price: the wallet
// it comes out of is the base one, and the funds gate fits nothing on that
// side either.
func TestNextEntryCostCountsAnInverseAddInBase(t *testing.T) {
	rows := rowPerDepthRows(2, 3, 4, 5)
	trade := costLadderTrade(2, 10, rows...)
	trade.Inverse = true
	for index := range trade.History {
		trade.History[index].Type = "SELL"
	}

	asset, cost := NextEntryCost(trade, 0)
	if asset != "LINK" {
		t.Errorf("asset = %q, want the base side of the pair", asset)
	}
	testutil.AssertFloatEqual(t, cost, 3*4, costEpsilon, "cost of an inverse next depth")
}

// A first entry has no fill to multiply, so what it would spend is the
// ladder's initial bid off the wallet it is being sized against — the same
// number the buy action commits. The budget only takes part here.
func TestNextEntryCostPricesAFirstEntryAsTheInitialBid(t *testing.T) {
	rows := rowPerDepthRows(2, 3, 4, 5)
	trade := costLadderTrade(0, 10, rows...)

	const budget = 10000

	want, bidErr := CalculateInitialBid(budget, trade, 0)
	if bidErr != nil {
		t.Fatalf("the fixture must size a first bid from this budget: %v", bidErr)
	}
	if want <= trade.StrategyPair.TradeFilters.MinNotional {
		t.Fatalf("initial bid %f must clear the pair minimum for this test to mean anything", want)
	}

	_, cost := NextEntryCost(trade, budget)
	testutil.AssertFloatEqual(t, cost, want, costEpsilon, "cost of a first entry")

	// A bigger wallet sizes a bigger first entry: the budget is what the
	// ladder is planned against.
	_, richer := NextEntryCost(trade, budget*2)
	if richer <= cost {
		t.Fatalf("a first entry off a bigger wallet cost %f, want more than %f", richer, cost)
	}
}

// The estimate is sized directly instead of through CalculateInitialBid's
// downward search, so it has to answer the same number wherever that search
// settles on the full-depth bid — which is every wallet the gate can hold an
// entry on. A pair configured off the half-depth grid the search steps down
// is the case that would drift if the alignment were dropped.
func TestNextEntryCostMatchesCalculateInitialBidOnAnOrdinaryWallet(t *testing.T) {
	const budget = 10000

	for _, depths := range []float64{4, 4.5, 4.7, 8} {
		rows := []aggragates.StrategySettings{{Percentage: rowDiscount, Multiplier: 2, Depths: depths}}

		for _, inverse := range []bool{false, true} {
			trade := costLadderTrade(0, 10, rows...)
			trade.Inverse = inverse

			want, bidErr := CalculateInitialBid(budget, trade, 0)
			if bidErr != nil {
				t.Fatalf("depths %v, inverse %v: the fixture must size a bid from this wallet: %v", depths, inverse, bidErr)
			}

			_, cost := NextEntryCost(trade, budget)
			testutil.AssertFloatEqual(t, cost, want, costEpsilon, "first entry sized directly")
		}
	}
}

// Every first entry is raised to the pair's minimum before it is placed, so
// that minimum is the least one can cost — including when the ladder cannot
// reach it from the wallet at all and the sizing refuses.
func TestNextEntryCostFloorsAFirstEntryAtThePairMinimum(t *testing.T) {
	// A wallet too small for the ladder to reach the pair's minimum from.
	const starved = 0.1

	rows := rowPerDepthRows(2, 3, 4, 5)
	trade := costLadderTrade(0, 10, rows...)

	if _, bidErr := CalculateInitialBid(starved, trade, 0); bidErr == nil {
		t.Fatal("the fixture must refuse to size a ladder from a wallet this small")
	}

	_, cost := NextEntryCost(trade, starved)
	testutil.AssertFloatEqual(t, cost, trade.StrategyPair.TradeFilters.MinNotional, costEpsilon, "cost of a refused first entry")

	// An inverse first entry is bid in base, so its floor is the minimum
	// notional converted at the position price.
	inverse := costLadderTrade(0, 10, rows...)
	inverse.Inverse = true

	if _, bidErr := CalculateInitialBid(starved, inverse, 0); bidErr == nil {
		t.Fatal("the inverse fixture must refuse the same wallet")
	}

	_, inverseCost := NextEntryCost(inverse, starved)
	testutil.AssertFloatEqual(t, inverseCost, inverse.StrategyPair.TradeFilters.MinNotional/inverse.PositionPrice, costEpsilon, "cost of a refused inverse first entry")
}

// A trade nothing can be priced from costs nothing: the gate then weighs the
// reserve alone rather than inventing an amount for an entry it cannot size.
func TestNextEntryCostIsZeroWithoutSettingsOrPrice(t *testing.T) {
	rows := rowPerDepthRows(2, 3, 4, 5)

	noSettings := costLadderTrade(2, 10, rows...)
	noSettings.StrategyPair.StrategySettings = nil

	noPrice := costLadderTrade(2, 0, rows...)

	for name, trade := range map[string]aggragates.Trades{
		"no settings row":   noSettings,
		"no position price": noPrice,
	} {
		if _, cost := NextEntryCost(trade, 1000); cost != 0 {
			t.Errorf("%s: cost = %f, want nothing priced", name, cost)
		}
	}
}
