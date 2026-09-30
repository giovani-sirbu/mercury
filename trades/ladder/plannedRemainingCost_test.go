package ladder

import (
	"fmt"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/dynamicparams"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// The planned cost is the wallet gate's tie-break between two ladders at the
// same depth, so what matters about it is when it may move and when it must
// not. It is the remaining-depths walk RemainingCost takes, started at the
// price of the ladder's last fill instead of its position price, which is why
// it is still between fills and moves on one.
//
// Every fixture here is the rowsGrid ladder, whose prices and quantities are
// exact in float64, so the hand-computed sums are exact rather than nearly so.

// One entry filled at quantity 1, its fill one below the history price, on a
// grid of four depths: the second entry is 4 at three quarters of the fill,
// the third 32 at a quarter of that, the fourth 96 at seven eighths of that.
const plannedFromTheFirstFill = 297 + 594 + 1559.25

// Two entries filled: the third entry is 32 at a quarter of the second fill,
// the fourth 96 at seven eighths of that.
const plannedFromTheSecondFill = 784 + 2058

// The walk is RemainingCost's own, anchored at the last fill: the same ladder
// with its position moved onto that fill answers the same sum, and with the
// position anywhere else the two readings part.
func TestPlannedRemainingCostWalksTheTailFromTheLastFill(t *testing.T) {
	trade := rowsLadder(1, 16, 4, rowsGrid()...)

	planned := DepthOf(trade).PlannedRemainingCost
	testutil.AssertFloatEqual(t, planned, plannedFromTheFirstFill, rowsEpsilon, "the tail planned from the last fill")

	onTheFill := trade
	onTheFill.PositionPrice = trade.History[len(trade.History)-1].Price
	_, fromTheFill := RemainingCost(onTheFill)
	testutil.AssertFloatEqual(t, planned, fromTheFill, rowsEpsilon, "RemainingCost with the position on the last fill")

	if _, fromThePosition := RemainingCost(trade); fromThePosition == planned {
		t.Fatalf("remaining cost = planned cost = %v: the fixture must keep its position off its last fill", planned)
	}
}

// Between two fills the position moves — it re-arms lower, it trails — and
// the reserve moves with it, but the key two level ladders are ranked on must
// not: a key read off the position would hand the front back and forth
// between them on every re-arm while neither buys anything.
func TestPlannedRemainingCostStandsStillWhileOnlyThePositionMoves(t *testing.T) {
	still := DepthOf(rowsLadder(1, 16, 4, rowsGrid()...))

	for _, price := range []float64{0.0625, 3175.5, rowsHistoryPrice - 1, 1e6} {
		moved := DepthOf(rowsLadder(1, price, 4, rowsGrid()...))

		if moved.PlannedRemainingCost != still.PlannedRemainingCost {
			t.Errorf("position at %v: planned cost = %v, want %v whatever the position", price, moved.PlannedRemainingCost, still.PlannedRemainingCost)
		}
		if moved.RemainingCost == still.RemainingCost {
			t.Errorf("position at %v: the reserve must move with the position, stayed at %v", price, moved.RemainingCost)
		}
	}

	// Nor while the last entry settles: its row grows part by part at the
	// price the entry was placed at, and the plan is sized from the first
	// entry, so neither half of the walk moves.
	complete := rowsLadder(2, 16, 4, rowsGrid()...)
	settling := complete
	settling.History = append([]aggragates.TradesHistory(nil), complete.History...)
	last := len(settling.History) - 1
	settling.History[last].Quantity /= 8
	settling.History[last].Status = "PARTIALLY_FILLED"

	if got, want := DepthOf(settling).PlannedRemainingCost, DepthOf(complete).PlannedRemainingCost; got != want {
		t.Fatalf("planned cost while the last entry settles = %v, want %v", got, want)
	}
}

// A fill is the one thing that moves the key: the ladder is a depth further
// down, what it has left starts one entry later, and the anchor is the new
// fill's price. Same position both times, so nothing else moved.
func TestPlannedRemainingCostMovesOnAFill(t *testing.T) {
	before := DepthOf(rowsLadder(1, 16, 4, rowsGrid()...))
	after := DepthOf(rowsLadder(2, 16, 4, rowsGrid()...))

	testutil.AssertFloatEqual(t, before.PlannedRemainingCost, plannedFromTheFirstFill, rowsEpsilon, "planned cost before the fill")
	testutil.AssertFloatEqual(t, after.PlannedRemainingCost, plannedFromTheSecondFill, rowsEpsilon, "planned cost after the fill")
}

// The anchor is the last ENTRY, read from the end of the history, so the rows
// that land after the entries must be stepped over: the bookkeeping row an
// impasse child's profit transfer writes onto its parent at a sentinel price,
// an entry-side row with no quantity, and a close on the other side of the
// book. Anchored on the sentinel, the ladder would plan its tail at a price
// of nothing and rank as all but free to finish — first in line on every tie.
func TestPlannedRemainingCostSkipsTheRowsThatAreNotEntries(t *testing.T) {
	clean := rowsLadder(2, 16, 4, rowsGrid()...)

	noisy := clean
	noisy.History = append(append([]aggragates.TradesHistory(nil), clean.History...),
		aggragates.TradesHistory{Type: "BUY", Quantity: 0, Price: 15, OrderId: 98},
		aggragates.TradesHistory{Type: "SELL", Quantity: 7, Price: 20, OrderId: 99},
		aggragates.TradesHistory{Type: "BUY", Quantity: 500, Price: AccountingPriceCeiling / 10, OrderId: 97},
	)

	if got := CountFilledEntries(noisy); got != 2 {
		t.Fatalf("depth = %d, want the two real entries alone", got)
	}

	got := DepthOf(noisy).PlannedRemainingCost
	testutil.AssertFloatEqual(t, got, plannedFromTheSecondFill, rowsEpsilon, "planned cost past the rows that are not entries")
}

// An inverse ladder's remaining depths are bare base quantities and its walk
// takes no price, so the planned reading is its remaining cost exactly —
// wherever its position sits, none included, and whatever its last fill was
// priced at.
func TestPlannedRemainingCostOfAnInverseLadderIsItsRemainingCost(t *testing.T) {
	for _, price := range []float64{16, 0.0625, 3175.5, 0} {
		trade := rowsInverse(rowsLadder(1, price, 4, rowsGrid()...))
		trade.History[len(trade.History)-1].Price = 3.75

		view := DepthOf(trade)
		if view.RemainingCost <= 0 {
			t.Fatalf("position at %v: the inverse fixture must have a tail, got %v", price, view.RemainingCost)
		}
		if view.PlannedRemainingCost != view.RemainingCost {
			t.Errorf("position at %v: planned cost = %v, want the remaining cost %v", price, view.PlannedRemainingCost, view.RemainingCost)
		}
	}
}

// The two costs name an amount on exactly the same ladders: a ladder the view
// says reserves nothing ranks as one with nothing left to pay for, and one
// that reserves something is ranked on a real plan. Swept over every depth of
// the grid and past it, long and inverse, with and without a position price,
// and on the rows that carry no settings or no ceiling at all.
func TestPlannedRemainingCostIsZeroExactlyWhereRemainingCostIs(t *testing.T) {
	ladders := map[string]aggragates.Trades{}

	for filled := 0; filled <= 5; filled++ {
		for _, price := range []float64{0, 16} {
			long := rowsLadder(filled, price, 4, rowsGrid()...)
			ladders[testName("long", filled, price)] = long
			ladders[testName("inverse", filled, price)] = rowsInverse(long)
		}
	}

	noSettings := rowsLadder(2, 16, 4, rowsGrid()...)
	noSettings.StrategyPair.StrategySettings = nil
	ladders["no settings row"] = noSettings
	ladders["no ceiling configured"] = rowsLadder(2, 16, 0, rowsGrid()...)

	for name, trade := range ladders {
		view := DepthOf(trade)
		if (view.RemainingCost == 0) != (view.PlannedRemainingCost == 0) {
			t.Errorf("%s: remaining cost %v, planned cost %v — want both zero or neither", name, view.RemainingCost, view.PlannedRemainingCost)
		}
	}

	// The sweep must reach both sides of that line, or it proves nothing.
	if DepthOf(rowsLadder(2, 16, 4, rowsGrid()...)).PlannedRemainingCost == 0 {
		t.Fatal("a ladder part way down its grid must plan a tail")
	}
	if DepthOf(rowsLadder(2, 0, 4, rowsGrid()...)).PlannedRemainingCost != 0 {
		t.Fatal("a long ladder without a position price must plan nothing, like its remaining cost")
	}
}

// testName labels one ladder of the sweep.
func testName(side string, filled int, price float64) string {
	return fmt.Sprintf("%s, %d filled, position at %v", side, filled, price)
}

// A ladder that opened raised is ranked on the cost of the depths it still
// has to fill, the ones its opened row added included, walked down the grid
// from its last fill at the raised percentage — the same walk the remaining
// cost takes, anchored where only a fill moves it.
//
// Four entries on the four-row grid, the last at 96: the stored rows leave
// nothing to plan. Raised by two depths the fifth and sixth entries both read
// the base row — 96x2 = 192 at 96x0.5 = 48, then 384 at 24 — so the plan is
// 9216 + 9216 = 18432, whatever the position price does.
func TestPlannedRemainingCostOfARaisedLadderAtItsStoredCeilingWalksTheExtraDepthsFromTheLastFill(t *testing.T) {
	grid := rowsGrid()
	stored := rowsLadder(len(grid), 16, float64(len(grid)), grid...)

	if planned := DepthOf(stored).PlannedRemainingCost; planned != 0 {
		t.Fatalf("planned cost = %v, want a ladder at its stored ceiling to plan nothing", planned)
	}

	raised := withOpenedRow(stored, 0, raiseDepths)

	planned := DepthOf(raised).PlannedRemainingCost
	testutil.AssertFloatEqual(t, planned, 18432, rowsEpsilon, "the extra depths planned from the last fill")

	for _, price := range []float64{0.0625, 16, 3175.5} {
		moved := raised
		moved.PositionPrice = price

		view := DepthOf(moved)
		testutil.AssertFloatEqual(t, view.PlannedRemainingCost, planned, rowsEpsilon, "the plan with the position moved")
		if view.RemainingCost <= 0 {
			t.Errorf("position at %v: remaining cost = %v, want a reserve for the extra depths", price, view.RemainingCost)
		}
	}
}

// Short of the stored ceiling the raise shows in both halves of the plan: the
// extra depths are more tail, and the percentage an opened row adds steps every
// remaining entry further down the grid from the last fill. Each amount form
// the opened row writes is planned on the rows it raises, read against the
// walk taken on the raised rows by hand.
func TestPlannedRemainingCostOfARaisedLadderWalksTheRaisedRows(t *testing.T) {
	grid := rowsGrid()
	cases := []struct {
		name   string
		points float64
		depths int
	}{
		{"depths only", 0, raiseDepths},
		{"percentage only", raisePoints, 0},
		{"percentage and depths", raisePoints, raiseDepths},
	}

	for _, c := range cases {
		for _, filled := range []int{1, 2, len(grid)} {
			stored := rowsLadder(filled, 16, float64(len(grid)), grid...)
			raisedRows := dynamicparams.RaiseBy(stored.StrategyPair.StrategySettings, c.points, c.depths)
			lastFill := stored.History[len(stored.History)-1].Price

			want := rowsTail(raisedRows, 1, filled, len(grid)+c.depths, lastFill)
			got := DepthOf(withOpenedRow(stored, c.points, c.depths)).PlannedRemainingCost

			testutil.AssertFloatEqual(t, got, want, rowsEpsilon+want*rowsEpsilon, fmt.Sprintf("%s, %d entries filled: the plan on the raised rows", c.name, filled))
		}
	}

	stored := rowsLadder(1, 16, float64(len(grid)), grid...)
	depthsOnly := DepthOf(withOpenedRow(stored, 0, raiseDepths)).PlannedRemainingCost
	wider := DepthOf(withOpenedRow(stored, raisePoints, raiseDepths)).PlannedRemainingCost
	if wider >= depthsOnly {
		t.Fatalf("a wider grid plans %v, want under the %v of the same depths on the stored percentage: the entries are bought lower", wider, depthsOnly)
	}
}
