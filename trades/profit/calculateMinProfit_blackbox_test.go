package profit_test

import (
	"testing"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
	"github.com/giovani-sirbu/mercury/trades/profit"
)

// entryFill is one fill on a trade's history, its side written the way the
// exchange writes it ("BUY" or "SELL") and carrying the exchange order it
// belongs to: two fills of one order are the partial fills of a single entry.
func entryFill(side string, orderID int64, quantity, price float64) aggragates.TradesHistory {
	return aggragates.TradesHistory{Type: side, OrderId: orderID, Quantity: quantity, Price: price}
}

// entryFills is the history of a ladder that has filled the given number of
// entries on the given side, each its own order of one base at 1000.
func entryFills(side string, filled int) []aggragates.TradesHistory {
	history := make([]aggragates.TradesHistory, 0, filled)
	for index := 0; index < filled; index++ {
		history = append(history, entryFill(side, int64(index+1), 1, 1000))
	}

	return history
}

// ladderTrade is a trade on the given settings rows with the given history.
func ladderTrade(rows []aggragates.StrategySettings, history ...aggragates.TradesHistory) aggragates.Trades {
	return aggragates.Trades{
		StrategyPair: aggragates.StrategiesPairs{StrategySettings: rows},
		History:      history,
	}
}

func TestCalculateMinProfit(t *testing.T) {
	single := []aggragates.StrategySettings{{Percentage: 2.2, Depths: 6}}

	// A row per depth: the same depths on every row and a percentage that rises
	// as the ladder goes deeper.
	perDepth := []aggragates.StrategySettings{
		{Percentage: 2, Depths: 8},
		{Percentage: 2.25, Depths: 8},
		{Percentage: 3, Depths: 8},
	}

	inverse := ladderTrade(single,
		entryFill("SELL", 1, 4, 50),
		entryFill("SELL", 2, 6, 40),
		entryFill("BUY", 3, 3, 30),
	)
	inverse.Inverse = true
	inverse.PositionPrice = 123

	inversePerDepth := ladderTrade(perDepth, entryFills("SELL", 2)...)
	inversePerDepth.Inverse = true

	tests := []struct {
		name  string
		trade aggragates.Trades
		want  float64
	}{
		{
			// 6 depths at 2.2: the whole position must clear 2.2 / 6 = 0.3666...%
			// over its average entry price, 36.666... on the 10000 its entry cost.
			"SixDepthsAtTwoPointTwoPercent",
			ladderTrade(single, entryFill("BUY", 1, 10, 1000)),
			10000 * 2.2 / 6 / 100,
		},
		{
			// Every fill adds what it cost, whatever it filled at: 5000 + 2700 + 1600.
			"SeveralFills",
			ladderTrade(single,
				entryFill("BUY", 1, 5, 1000),
				entryFill("BUY", 2, 3, 900),
				entryFill("BUY", 3, 2, 800),
			),
			9300 * 2.2 / 6 / 100,
		},
		{
			// Two partial fills of one order are one entry: the ladder holds its
			// second depth, whose row is 2.25 over 8, not its third. The cost still
			// adds every fill: 6000 + 1800 + 1800.
			"TwoPartialFillsOfOneOrder",
			ladderTrade(perDepth,
				entryFill("BUY", 1, 6, 1000),
				entryFill("BUY", 2, 2, 900),
				entryFill("BUY", 2, 2, 900),
			),
			9600 * 2.25 / 8 / 100,
		},
		{
			// The exits a ladder has booked are not what its entries cost.
			"ExitsAreNotEntries",
			ladderTrade(single,
				entryFill("BUY", 1, 10, 1000),
				entryFill("SELL", 2, 4, 1100),
			),
			10000 * 2.2 / 6 / 100,
		},
		{
			// The profit of an inverse trade is counted in base, so the floor is the
			// share of the base its entries sold (4 + 6) and neither the prices they
			// sold at, nor the buy-back, nor the position price are read.
			"Inverse",
			inverse,
			10 * 2.2 / 6 / 100,
		},
		{
			"InverseReadsTheHeldDepthRowToo",
			inversePerDepth,
			2 * 2.25 / 8 / 100,
		},
		{
			"ZeroPercentage",
			ladderTrade([]aggragates.StrategySettings{{Percentage: 0, Depths: 6}}, entryFill("BUY", 1, 10, 1000)),
			0,
		},
		{
			"FirstDepthReadsTheBaseRow",
			ladderTrade(perDepth, entryFills("BUY", 1)...),
			1000 * 2.0 / 8 / 100,
		},
		{
			"SecondDepthReadsTheSecondRow",
			ladderTrade(perDepth, entryFills("BUY", 2)...),
			2000 * 2.25 / 8 / 100,
		},
		{
			"ThirdDepthReadsTheThirdRow",
			ladderTrade(perDepth, entryFills("BUY", 3)...),
			3000 * 3.0 / 8 / 100,
		},
		{
			// A depth no row was configured for falls back to the base row, never to
			// the last one.
			"ADepthPastTheRowsFallsBackToTheBaseRow",
			ladderTrade(perDepth, entryFills("BUY", 4)...),
			4000 * 2.0 / 8 / 100,
		},
		{
			// The depths are the held row's own, not the first row's.
			"TheHeldRowsOwnDepths",
			ladderTrade(
				[]aggragates.StrategySettings{{Percentage: 2.4, Depths: 8}, {Percentage: 3, Depths: 6}},
				entryFills("BUY", 2)...,
			),
			2000 * 3.0 / 6 / 100,
		},
		{
			// A fraction of an entry cannot be placed: 6.9 depths are 6.
			"FractionalDepthsAreFloored",
			ladderTrade([]aggragates.StrategySettings{{Percentage: 2.2, Depths: 6.9}}, entryFill("BUY", 1, 10, 1000)),
			10000 * 2.2 / 6 / 100,
		},
		{
			// A row configured without depths divides by one, not by zero.
			"NoDepthsDividesByOne",
			ladderTrade([]aggragates.StrategySettings{{Percentage: 2, Depths: 0}}, entryFill("BUY", 1, 10, 1000)),
			10000 * 2.0 / 1 / 100,
		},
		{
			// Less than one depth floors to none, which counts as one.
			"LessThanOneDepthDividesByOne",
			ladderTrade([]aggragates.StrategySettings{{Percentage: 2, Depths: 0.5}}, entryFill("BUY", 1, 10, 1000)),
			10000 * 2.0 / 1 / 100,
		},
		{
			"NoRows",
			ladderTrade(nil, entryFill("BUY", 1, 10, 1000)),
			0,
		},
		{
			"NoFills",
			ladderTrade(single),
			0,
		},
		{
			"OnlyExits",
			ladderTrade(single, entryFill("SELL", 1, 4, 1100)),
			0,
		},
		{
			// A child-profit transfer onto an impasse parent is bookkeeping at the
			// sentinel price: it is neither a depth nor a cost, so the ladder still
			// holds its first depth, on the base row, over its one real fill.
			"AnAccountingRowIsNotAnEntry",
			ladderTrade(perDepth,
				entryFill("BUY", 30, 12.5, ladder.AccountingPriceCeiling),
				entryFill("BUY", 1, 1, 1000),
			),
			1000 * 2.0 / 8 / 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := profit.CalculateMinProfit(tt.trade)
			testutil.AssertFloatEqual(t, got, tt.want, 1e-10, "CalculateMinProfit")
		})
	}
}

// The floor is asked for in the user's own terms: a ladder of 6 depths at a
// percentage of 2.2 must close at least 0.3666...% over its average entry price.
// The close price that nets exactly the floor on the whole position is that far
// over the average, whichever prices the entries filled at.
func TestCalculateMinProfitIsThePercentageOverTheAverageEntryPrice(t *testing.T) {
	trade := ladderTrade(
		[]aggragates.StrategySettings{{Percentage: 2.2, Depths: 6}},
		entryFill("BUY", 1, 4, 1000),
		entryFill("BUY", 2, 4, 900),
		entryFill("BUY", 3, 2, 800),
	)

	average := ladder.AverageEntryPrice(trade)
	closePrice := average + profit.CalculateMinProfit(trade)/ladder.EntryQuantity(trade)

	overAverage := (closePrice/average - 1) * 100
	testutil.AssertFloatEqual(t, overAverage, 2.2/6, 1e-9, "percentage over the average entry price")
}
