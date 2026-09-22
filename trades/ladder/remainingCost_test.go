package ladder

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// costEpsilon is float noise on sums of a few entries priced in the hundreds.
const costEpsilon = 1e-9

// rowDiscount is the percentage step every fixture row puts between one depth
// and the next: the ladder plans each entry one step further down than the
// one before it, and the remaining-depths walk has to move with it.
const rowDiscount = 2.5

// costLadderTrade is a ladder whose fills really do grow by the multiplier of
// the row each of them read, with a position price to cost them at. The
// remaining depths are priced by walking that same growth forward, so a
// fixture whose fills followed a different one would be priced against a
// ladder that never existed.
func costLadderTrade(filled int, price float64, rows ...aggragates.StrategySettings) aggragates.Trades {
	trade := depthTrade(filled, rows...)
	trade.PositionPrice = price
	trade.StrategyPair.TradeFilters = aggragates.TradeFilters{MinNotional: 5}

	// Entry n reads row n-1, the row-selection contract the funds gate sizes
	// the order it places with: the first entry is the initial bid and reads
	// no row, and each one after it grows by the row ITS own depth carries.
	quantity := 1.0
	for index := range trade.History {
		trade.History[index].Quantity = quantity
		quantity *= rows[SettingsIndexOrBase(rows, index+1)].Multiplier
	}

	return trade
}

// rowPerDepthRows configures one row per depth, each with its own multiplier
// and all of them allowing the same number of entries.
func rowPerDepthRows(multipliers ...float64) []aggragates.StrategySettings {
	rows := make([]aggragates.StrategySettings, 0, len(multipliers))
	for _, multiplier := range multipliers {
		rows = append(rows, aggragates.StrategySettings{Percentage: rowDiscount, Multiplier: multiplier, Depths: float64(len(multipliers))})
	}
	return rows
}

// The walk is the ladder's own PLANNED arithmetic: each remaining entry
// multiplies the previous quantity by the multiplier of the ROW that entry
// reads AND steps the price one further percentage down, because that is the
// level the grid was laid out to buy it at. A grid that changes its
// multiplier per depth is what makes the row selection visible — reading one
// row for the whole tail would answer a different number.
func TestRemainingCostWalksEachDepthOnItsOwnRow(t *testing.T) {
	const price = 10
	const step = 1 - rowDiscount/100

	rows := rowPerDepthRows(2, 3, 4, 5)
	trade := costLadderTrade(2, price, rows...)

	// Two entries filled, the second grown by the second row, so the tail is
	// the third entry on the third row and the fourth on the fourth — each
	// one step further down the grid than the one before.
	want := 12*(price*step) + 60*(price*step*step)

	asset, cost := RemainingCost(trade)
	if asset != "USDT" {
		t.Errorf("asset = %q, want the quote side of the pair", asset)
	}
	testutil.AssertFloatEqual(t, cost, want, costEpsilon, "remaining cost of the tail")
}

// The closed form of that walk on a one-row grid, which is the budget
// CalculateInitialBid lays the ladder out against: every remaining entry
// costs the previous one times the multiplier times the price step, so the
// tail is the last filled entry's cost times the sum of that ratio's powers.
// Pricing the tail at today's price instead — the defect this replaced —
// would drop the step and leave a reserve the grid can never spend.
func TestRemainingCostIsTheLaddersRemainingPlannedBudget(t *testing.T) {
	const price = 10
	const multiplier = 2
	const ceiling = 6

	filled := 2
	trade := costLadderTrade(filled, price, aggragates.StrategySettings{
		Percentage: rowDiscount,
		Multiplier: multiplier,
		Depths:     ceiling,
	})

	ratio := multiplier * (1 - rowDiscount/100)
	term := 1.0
	want := 0.0
	for remaining := 0; remaining < ceiling-filled; remaining++ {
		term *= ratio
		want += term
	}
	want *= plannedQuantityAtDepth(trade, filled) * price

	_, cost := RemainingCost(trade)
	testutil.AssertFloatEqual(t, cost, want, costEpsilon, "remaining planned budget")

	// And it is strictly under what the same tail would come to at today's
	// price, which is what made one ladder's remainder outgrow the wallet.
	atTodaysPrice := 0.0
	quantity := plannedQuantityAtDepth(trade, filled)
	for remaining := 0; remaining < ceiling-filled; remaining++ {
		quantity *= multiplier
		atTodaysPrice += quantity * price
	}
	if cost >= atTodaysPrice {
		t.Fatalf("planned budget %f, want less than the tail at today's price %f", cost, atTodaysPrice)
	}
}

// An inverse ladder spends the BASE side of its pair and its entries ARE base
// quantities, so nothing is multiplied by the price: costing them in quote
// would reserve a wallet the ladder never touches.
func TestRemainingCostCountsAnInverseLadderInBase(t *testing.T) {
	const price = 10

	rows := rowPerDepthRows(2, 3, 4, 5)
	trade := costLadderTrade(2, price, rows...)
	trade.Inverse = true
	for index := range trade.History {
		trade.History[index].Type = "SELL"
	}

	asset, cost := RemainingCost(trade)
	if asset != "LINK" {
		t.Errorf("asset = %q, want the base side of the pair", asset)
	}
	testutil.AssertFloatEqual(t, cost, 12+60, costEpsilon, "remaining cost of an inverse tail")
}

// An entry settles in parts and its history row's quantity grows with them,
// so mid-settlement the deepest row carries LESS than the row above it. The
// reserve must not move: what the ladder has left to spend is decided by the
// plan, not by how much of the current depth has executed so far. Reading the
// executed quantity as the base made the ladder look one depth shallower and
// let siblings spend exactly what it still needed.
func TestRemainingCostPricesASettlingDepthAtItsPlannedSize(t *testing.T) {
	const price = 10

	rows := rowPerDepthRows(2, 3, 4, 5)
	complete := costLadderTrade(3, price, rows...)

	settling := costLadderTrade(3, price, rows...)
	last := len(settling.History) - 1
	settling.History[last].Quantity = settling.History[last-1].Quantity / 2
	if settling.History[last].Quantity >= settling.History[last-1].Quantity {
		t.Fatal("the fixture must leave the deepest row carrying less than the one above it")
	}

	_, want := RemainingCost(complete)
	_, got := RemainingCost(settling)

	testutil.AssertFloatEqual(t, got, want, costEpsilon, "reserve while the deepest entry is still settling")
}

// Nothing to price means nothing reserved. Each of these ladders is in the
// view for its own reasons and none of them can name an amount the wallet
// would have to be kept for.
func TestRemainingCostIsZeroWhenThereIsNothingToPrice(t *testing.T) {
	rows := rowPerDepthRows(2, 3, 4, 5)

	noFills := costLadderTrade(0, 10, rows...)
	noSettings := costLadderTrade(2, 10, rows...)
	noSettings.StrategyPair.StrategySettings = nil
	full := costLadderTrade(len(rows), 10, rows...)
	noPrice := costLadderTrade(2, 0, rows...)

	for name, trade := range map[string]aggragates.Trades{
		"no fills to multiply forward": noFills,
		"no settings row":              noSettings,
		"already at its ceiling":       full,
		"no position price":            noPrice,
	} {
		if _, cost := RemainingCost(trade); cost != 0 {
			t.Errorf("%s: cost = %f, want nothing reserved", name, cost)
		}
	}
}

// The asset is read off the symbol whatever else the trade is missing: a view
// entry with no asset matches no other, which is what keeps a pair nobody can
// parse out of every wallet instead of in all of them.
func TestRemainingCostNamesTheSpendingAsset(t *testing.T) {
	rows := rowPerDepthRows(2, 3, 4, 5)

	unparsable := costLadderTrade(2, 10, rows...)
	unparsable.Symbol = "LINKUSDT"

	if asset, _ := RemainingCost(unparsable); asset != "" {
		t.Errorf("asset = %q, want no asset at all for a symbol that cannot be split", asset)
	}

	empty := costLadderTrade(0, 10, rows...)
	if asset, _ := RemainingCost(empty); asset != "USDT" {
		t.Errorf("asset = %q, want the spending asset even with nothing to price", asset)
	}
}

// The cost is read off the trade, never written into it. The wallet view is
// built over trades the engines hold by reference and read under a shared
// lock, and the helper that answers "what did the last entry cost" sorts the
// slice it is handed — so this one hands it a copy. Reordering an engine's
// own history rows from a read would be a race on every gated tick.
func TestRemainingCostLeavesTheHistoryOrderAlone(t *testing.T) {
	rows := rowPerDepthRows(2, 3, 4, 5)
	trade := costLadderTrade(2, 10, rows...)

	// A close between the two entries: the row the sort would move first.
	trade.History = append(trade.History, aggragates.TradesHistory{
		Type:     "SELL",
		Quantity: 99,
		Price:    120,
		OrderId:  90,
	})

	before := make([]int64, 0, len(trade.History))
	for _, row := range trade.History {
		before = append(before, row.OrderId)
	}

	RemainingCost(trade)

	for index, row := range trade.History {
		if row.OrderId != before[index] {
			t.Fatalf("history row %d moved: order %d, want order %d", index, row.OrderId, before[index])
		}
	}
}
