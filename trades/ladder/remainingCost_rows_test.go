package ladder

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// What a ladder still has to spend, on a grid that changes BOTH its
// multiplier and its percentage per depth and is configured for more depths
// than it has rows.
//
// The remaining tail is the budget the grid was planned with, so every
// remaining entry moves two ways at once: the quantity up by its row's
// multiplier and the price down by that row's percentage. A grid whose rows
// all carry the same pair of numbers answers the same sum whichever row each
// entry was priced on, so it cannot show which row was read. These fixtures
// give every row its own pair, and put the base row's pair well away from the
// last row's, so the readings a reader could pick apart are different sums
// rather than the same one by coincidence.

// rowsEpsilon is float noise on sums of a few entries priced in the tens.
const rowsEpsilon = 1e-9

// rowsHistoryPrice is what the fixture's fills were placed at. It is not the
// position price: the entries a ladder has left are costed down the grid from
// the position price, and keeping the two apart is what lets a test move one
// without moving the other.
const rowsHistoryPrice = 100

// rowsStep is one settings row as this file cares about it: how much the next
// entry grows, and how far down the grid it is planned to be bought.
type rowsStep struct {
	multiplier float64
	percentage float64
}

// rowsGrid is the grid every test below walks. Each row carries its own pair,
// and the base row's pair is nothing like the last row's so a fallback to one
// can never be mistaken for a fallback to the other.
//
// Every percentage lands on a binary fraction — the price factors are 0.5,
// 0.75, 0.25 and 0.875 — and every multiplier is a whole number, so each
// intermediate price, quantity and product below is exact in float64 and the
// hand-computed sums are exact rather than nearly so.
func rowsGrid() []rowsStep {
	return []rowsStep{
		{multiplier: 2, percentage: 50},
		{multiplier: 4, percentage: 25},
		{multiplier: 8, percentage: 75},
		{multiplier: 3, percentage: 12.5},
	}
}

// rowsLadder is a long whose filled entries follow the ladder's own rule:
// entry n multiplies the entry before it by the multiplier of the row that
// entry reads — row n-1, the same row selection Buy and the funds gate use —
// so the quantity the remaining depths are priced forward from is the one the
// grid would really have produced.
//
// Every row allows the same number of entries, so the ceiling is `depths`
// whichever row the next fill reads.
func rowsLadder(filled int, price, depths float64, steps ...rowsStep) aggragates.Trades {
	rows := make([]aggragates.StrategySettings, 0, len(steps))
	for _, step := range steps {
		rows = append(rows, aggragates.StrategySettings{
			Percentage: step.percentage,
			Multiplier: step.multiplier,
			Depths:     depths,
		})
	}

	trade := aggragates.Trades{
		ID:            7412,
		Symbol:        "LINK/USDT",
		PositionPrice: price,
		StrategyPair: aggragates.StrategiesPairs{
			TradeFilters:     aggragates.TradeFilters{MinNotional: 5},
			StrategySettings: rows,
		},
	}

	quantity := 1.0
	for entry := 1; entry <= filled; entry++ {
		if entry > 1 {
			quantity *= rows[SettingsIndexOrBase(rows, entry-1)].Multiplier
		}
		trade.History = append(trade.History, aggragates.TradesHistory{
			Type:     "BUY",
			Quantity: quantity,
			Price:    rowsHistoryPrice - float64(entry),
			OrderId:  int64(entry),
		})
	}

	return trade
}

// rowsInverse is the same ladder on the other side: an inverse trade enters
// by selling base, so its entries are SELL rows.
func rowsInverse(trade aggragates.Trades) aggragates.Trades {
	trade.Inverse = true
	for index := range trade.History {
		trade.History[index].Type = "SELL"
	}
	return trade
}

// Each remaining entry is priced on the row that entry reads — both halves of
// that row — and never on one row for the whole tail. The rows below are
// deliberately out of order, so a reader that held the last filled row, or
// the first one, for the rest of the ladder answers a different sum rather
// than the same one.
//
// One entry filled at quantity 1 on a grid of four depths, position at 16:
//   - the second entry reads the second row, 1x4 = 4 at 16x0.75 = 12, so 48;
//   - the third reads the third, 4x8 = 32 at 12x0.25 = 3, so 96;
//   - the fourth reads the fourth, 32x3 = 96 at 3x0.875 = 2.625, so 252.
//
// The tail is 396. Held at the second row throughout it would be 624, and at
// the base row throughout 48.
func TestRemainingCostPricesEachRemainingDepthOnItsOwnRow(t *testing.T) {
	trade := rowsLadder(1, 16, 4, rowsGrid()...)

	asset, cost := RemainingCost(trade)
	if asset != "USDT" {
		t.Errorf("asset = %q, want the quote side of the pair", asset)
	}
	testutil.AssertFloatEqual(t, cost, 396, rowsEpsilon, "the tail priced row by row")
}

// The PERCENTAGE is read from the entry's own row too, not only the
// multiplier. A grid whose rows share one percentage cannot show this — every
// step of its tail moves the price by the same factor whichever row was
// read — so it is pinned here, where each row steps differently: the same
// quantities walked down one shared percentage come to a different sum.
func TestRemainingCostStepsThePriceByTheRowTheEntryReads(t *testing.T) {
	const price = 16

	grid := rowsGrid()
	_, cost := RemainingCost(rowsLadder(1, price, 4, grid...))

	// The same multipliers, every step taken at the BASE row's percentage.
	shared := 0.0
	quantity := 1.0
	stepped := float64(price)
	for _, step := range grid[1:] {
		quantity *= step.multiplier
		stepped *= 1 - grid[0].percentage/100
		shared += quantity * stepped
	}

	testutil.AssertFloatEqual(t, shared, 352, rowsEpsilon, "the tail stepped by one shared percentage")
	if cost == shared {
		t.Fatalf("tail = %v: this grid must price each step by its own row's percentage", cost)
	}
}

// Past the last configured row every further entry falls back to the BASE
// row, never to the deepest one — the row-selection contract every other
// ladder read keeps, and here it decides the price step as well as the
// quantity. Four rows on a grid configured for six depths, four of them
// filled (1, 4, 32, 96), position at 16: the fifth and sixth entries both
// read the base row, so 96x2 = 192 at 16x0.5 = 8 and then 192x2 = 384 at
// 8x0.5 = 4, and the tail is 1536 + 1536 = 3072.
//
// Held at the LAST row instead it would be 14616, and at today's price on the
// base row 9216 — three sums no coincidence folds together.
func TestRemainingCostFallsBackToTheBaseRowPastTheLastConfiguredRow(t *testing.T) {
	grid := rowsGrid()
	trade := rowsLadder(len(grid), 16, 6, grid...)

	if got := ConfiguredDepths(trade); got != 6 {
		t.Fatalf("ceiling = %d, want the six depths every row of this grid allows", got)
	}

	_, cost := RemainingCost(trade)
	testutil.AssertFloatEqual(t, cost, 3072, rowsEpsilon, "the tail past the last configured row")
}

// A deepest row still settling is priced at the size the grid PLANNED for
// that depth, not at the part of it the exchange has handed over so far. The
// engine raises the quantity on that entry's own row as the parts land, so
// for a while the deepest row carries less than the row above it — and a
// reserve rebuilt from the largest executed quantity would read the tail off
// the wrong link of the chain.
//
// On this grid that is not merely a halving: the rows carry different
// multipliers, so starting one entry back walks the remaining depths through
// a different sequence of rows entirely. The plan has no such window — it is
// the FIRST entry taken through the multiplier of every row the entries after
// it read, which is fixed the moment the ladder is laid out.
func TestRemainingCostPricesASettlingLastRowAtItsPlannedSize(t *testing.T) {
	grid := rowsGrid()
	full := rowsLadder(len(grid), 16, 6, grid...)

	_, want := RemainingCost(full)
	testutil.AssertFloatEqual(t, want, 3072, rowsEpsilon, "the tail of a fully filled ladder")

	last := len(full.History) - 1
	planned := full.History[last].Quantity
	above := full.History[last-1].Quantity

	for _, executed := range []float64{above / 2, full.History[0].Quantity / 8} {
		if executed >= above {
			t.Fatalf("the fixture must leave the deepest row under the %v above it, got %v", above, executed)
		}

		settling := full
		settling.History = append([]aggragates.TradesHistory(nil), full.History...)
		settling.History[last].Quantity = executed
		settling.History[last].Status = "PARTIALLY_FILLED"

		if got := CountFilledEntries(settling); got != len(grid) {
			t.Fatalf("executed %v: depth = %d, want the settling entry still counted as one", executed, got)
		}

		_, cost := RemainingCost(settling)
		testutil.AssertFloatEqual(t, cost, want, rowsEpsilon, "the tail of a ladder settling its deepest entry")

		// The reading the fix replaced, for the record: the largest quantity
		// that has executed is the row ABOVE the settling one, and a tail
		// walked from there is a different, smaller reserve.
		if planned <= above {
			t.Fatalf("the fixture must plan the deepest entry above the one before it, got %v after %v", planned, above)
		}
	}
}

// The plan starts at the ladder's first real ENTRY, and the rows that are not
// one are stepped over: a close on the other side of the book, a row carrying
// no quantity, and above all the bookkeeping row an impasse child's profit
// transfer leaves on its parent.
//
// That last one is the dangerous row. It is written on the entry side at a
// sentinel price, and what it carries is the child's profit — a QUOTE amount
// standing in a base quantity's field, which on most pairs is orders of
// magnitude larger than any entry the ladder ever placed. Read as the first
// entry it would not shift the reserve a little: the whole plan is that
// quantity taken through every multiplier, so the ladder would keep a wallet
// it could never spend and hold every sibling of the exchange indefinitely.
func TestRemainingCostPlansFromTheFirstRealEntry(t *testing.T) {
	grid := rowsGrid()
	clean := rowsLadder(len(grid), 16, 6, grid...)

	_, want := RemainingCost(clean)
	testutil.AssertFloatEqual(t, want, 3072, rowsEpsilon, "the tail of a ladder with nothing but entries")

	// The same ladder as the database hands it back once an impasse child has
	// closed onto it and a partial close has been booked.
	noisy := clean
	noisy.History = append([]aggragates.TradesHistory{
		{Type: "BUY", Quantity: 500, Price: AccountingPriceCeiling / 10, OrderId: 97},
		{Type: "BUY", Quantity: 0, Price: 15, OrderId: 98},
		{Type: "SELL", Quantity: 7, Price: 20, OrderId: 99},
	}, clean.History...)

	if got := CountFilledEntries(noisy); got != len(grid) {
		t.Fatalf("depth = %d, want the %d real entries alone", got, len(grid))
	}

	_, cost := RemainingCost(noisy)
	testutil.AssertFloatEqual(t, cost, want, rowsEpsilon, "the tail of a ladder carrying bookkeeping rows")
}

// An inverse ladder's entries ARE base quantities, so its remainder is
// counted in base and the position price takes no part in it at all — neither
// as a level nor as a step, since the percentage shapes only the quote
// proceeds on that side. The same ladder answers the same amount at any
// price, including none. Taking an inverse tail to the price would reserve a
// quote wallet the ladder never spends from, in a currency it never holds.
func TestRemainingCostOfAnInverseLadderIsBaseAndIgnoresThePrice(t *testing.T) {
	// The same three entries as the row-by-row case, counted in base: 4, 32
	// and 96.
	const want = 132

	for _, price := range []float64{16, 0.0625, 3175.5, 0} {
		trade := rowsInverse(rowsLadder(1, price, 4, rowsGrid()...))

		asset, cost := RemainingCost(trade)
		if asset != "LINK" {
			t.Errorf("position at %v: asset = %q, want the base side of the pair", price, asset)
		}
		testutil.AssertFloatEqual(t, cost, want, rowsEpsilon, "an inverse tail against the position price")
	}
}

// A long ladder's remainder is those planned quantities taken to planned
// prices, and every one of those prices is the position price times a factor
// the grid fixes — so the whole tail scales with the position price exactly.
// The wallet a long ladder is kept for is worth more when its position sits
// higher, and the plan behind it never moves.
func TestRemainingCostOfALongLadderScalesWithThePositionPrice(t *testing.T) {
	_, unit := RemainingCost(rowsLadder(1, 1, 4, rowsGrid()...))
	if unit <= 0 {
		t.Fatalf("the fixture must have a tail to price, got %v", unit)
	}

	for _, price := range []float64{0.0625, 1, 16, 3175.5} {
		_, cost := RemainingCost(rowsLadder(1, price, 4, rowsGrid()...))

		want := unit * price
		testutil.AssertFloatEqual(t, cost, want, rowsEpsilon+want*rowsEpsilon, "a long tail at the position price")
	}

	// And a long ladder with no position price names no amount: there is
	// nothing to step down from, and a ladder that cannot name an amount
	// reserves nothing rather than reserving zero by accident.
	if _, cost := RemainingCost(rowsLadder(1, 0, 4, rowsGrid()...)); cost != 0 {
		t.Fatalf("cost = %v, want nothing reserved without a position price", cost)
	}
}
