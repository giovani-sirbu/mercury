package ladder

import (
	"fmt"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/gates/dynamicparams"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// A ladder that opened raised is ranked on the cost of the depths it still has
// to fill, the ones its opened event added included, walked down the grid from
// its last fill at the raised percentage — the same walk the remaining cost
// takes, anchored where only a fill moves it.
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

	raised := withOpenedEvent(stored, 0, raiseDepths)

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
// extra depths are more tail, and the percentage an opened event adds steps
// every remaining entry further down the grid from the last fill. Each amount
// form the opened event carries is planned on the rows it raises, read against
// the walk taken on the raised rows by hand.
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
			got := DepthOf(withOpenedEvent(stored, c.points, c.depths)).PlannedRemainingCost

			testutil.AssertFloatEqual(t, got, want, rowsEpsilon+want*rowsEpsilon, fmt.Sprintf("%s, %d entries filled: the plan on the raised rows", c.name, filled))
		}
	}

	stored := rowsLadder(1, 16, float64(len(grid)), grid...)
	depthsOnly := DepthOf(withOpenedEvent(stored, 0, raiseDepths)).PlannedRemainingCost
	wider := DepthOf(withOpenedEvent(stored, raisePoints, raiseDepths)).PlannedRemainingCost
	if wider >= depthsOnly {
		t.Fatalf("a wider grid plans %v, want under the %v of the same depths on the stored percentage: the entries are bought lower", wider, depthsOnly)
	}
}
