package ladder

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/dynamicparams"
)

// The raise adds exactly the depths the shipped constant names, and the ladder
// is full again where they end: short of the raised ceiling it still has a
// depth to pay for, on it — and past it — it is full and reserves nothing. The
// opened event carries the shipped amounts, as the engine writes it at open, so
// the ceiling moves with a retune of the constants and the ladder stays exactly
// the depths the constant adds from being full.
func TestDepthOfARaisedLadderIsFullAgainWhereTheShippedDepthsEnd(t *testing.T) {
	grid := rowsGrid()
	stored := len(grid)
	raisedCeiling := stored + dynamicparams.BearDepths

	for filled := stored; filled <= raisedCeiling+1; filled++ {
		trade := withOpenedEvent(rowsLadder(filled, 16, float64(stored), grid...), dynamicparams.BearPercentagePoints, dynamicparams.BearDepths)

		view := DepthOf(trade)

		if view.MaxDepth != raisedCeiling {
			t.Errorf("%d entries filled: MaxDepth = %d, want the stored %d raised by %d", filled, view.MaxDepth, stored, dynamicparams.BearDepths)
		}

		full := filled >= raisedCeiling
		switch {
		case full && (view.RemainingCost != 0 || view.PlannedRemainingCost != 0):
			t.Errorf("%d entries filled: DepthOf = %+v, want a full ladder reserving nothing", filled, view)
		case !full && (view.RemainingCost <= 0 || view.PlannedRemainingCost <= 0):
			t.Errorf("%d entries filled: DepthOf = %+v, want a reserve for the depth still to fill", filled, view)
		}
	}
}

// Where the flag does not shape the ladder, a stray opened event in its
// strategy events changes nothing: the view of an inverse ladder, a futures
// strategy, an impasse child and a trade with the flag off is exactly the view
// of the same trade without the event — the stored ceiling, full and reserving
// nothing at it, and the stored tail short of it.
func TestDepthOfMeasuresOnTheStoredRowsWhereTheFlagDoesNotApply(t *testing.T) {
	grid := rowsGrid()
	stored := len(grid)

	cases := []struct {
		name  string
		shape func(aggragates.Trades) aggragates.Trades
	}{
		{"an inverse ladder", rowsInverse},
		{"a futures strategy", func(trade aggragates.Trades) aggragates.Trades {
			trade.Strategy.TradeType = aggragates.Futures
			return trade
		}},
		{"an impasse child", func(trade aggragates.Trades) aggragates.Trades {
			trade.ParentID = 7
			return trade
		}},
		{"the flag off", func(trade aggragates.Trades) aggragates.Trades {
			trade.Strategy.Params.DynamicParams = false
			return trade
		}},
	}

	for _, c := range cases {
		for filled := stored - 1; filled <= stored+1; filled++ {
			plain := rowsLadder(filled, 16, float64(stored), grid...)

			got := DepthOf(c.shape(withOpenedEvent(plain, raisePoints, raiseDepths)))
			want := DepthOf(c.shape(plain))

			if got != want {
				t.Errorf("%s, %d entries filled: DepthOf = %+v, want the view of the same trade without the opened event %+v", c.name, filled, got, want)
			}
			if got.MaxDepth != stored {
				t.Errorf("%s, %d entries filled: MaxDepth = %d, want the stored ceiling %d", c.name, filled, got.MaxDepth, stored)
			}
			if filled >= stored && (got.RemainingCost != 0 || got.PlannedRemainingCost != 0) {
				t.Errorf("%s, %d entries filled: DepthOf = %+v, want a full ladder reserving nothing", c.name, filled, got)
			}
		}
	}
}

// The view reads the opened event and never the row beside it: a ladder at its
// stored ceiling whose logs carry the opened row but whose strategy events do
// not is the view of the ladder without the pair, full and reserving nothing,
// where the same ladder with its event is not.
func TestDepthOfReadsTheOpenedEventNeverTheRow(t *testing.T) {
	grid := rowsGrid()
	plain := rowsLadder(len(grid), 16, float64(len(grid)), grid...)

	paired := withOpenedEvent(plain, raisePoints, raiseDepths)
	textOnly := paired
	textOnly.StrategyEvents = nil

	if len(textOnly.Logs) != 1 {
		t.Fatalf("fixture drifted: the trade carries %d rows, want the opened row", len(textOnly.Logs))
	}
	if got, want := DepthOf(textOnly), DepthOf(plain); got != want {
		t.Errorf("DepthOf = %+v, want the view of the ladder without the pair %+v: a row without its event opens nothing", got, want)
	}
	if view := DepthOf(paired); view.MaxDepth != len(grid)+raiseDepths || view.RemainingCost <= 0 {
		t.Errorf("DepthOf = %+v, want the raised ceiling and a reserve when the event stands beside the row", view)
	}
}
