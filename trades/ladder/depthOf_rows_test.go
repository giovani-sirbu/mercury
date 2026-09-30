package ladder

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/dynamicparams"
)

// rowPerDepthTrade is a long whose pair configures one settings row per depth,
// each row allowing a different number of entries, with the given number of
// entries already filled.
func rowPerDepthTrade(filled int, depths ...float64) aggragates.Trades {
	rows := make([]aggragates.StrategySettings, 0, len(depths))
	for _, allowed := range depths {
		rows = append(rows, aggragates.StrategySettings{Percentage: 2.5, Depths: allowed})
	}

	return depthTrade(filled, rows...)
}

// On a row-per-depth grid the ceiling moves with the ladder: each fill hands
// the next entry to the next row, and it is that row's Depths the wallet view
// carries. The flip happens on the fill that crosses into the row — a row
// early or late is a different ceiling, and the depth-priority band is
// measured against it.
func TestConfiguredDepthsFlipsOnTheFillThatCrossesIntoTheNextRow(t *testing.T) {
	rows := []float64{4, 5, 6, 7}

	for filled, allowed := range rows {
		trade := rowPerDepthTrade(filled, rows...)

		if got := ConfiguredDepths(trade); got != int(allowed) {
			t.Errorf("%d entries filled: ConfiguredDepths = %d, want the row of the next fill (%d)", filled, got, int(allowed))
		}

		view := DepthOf(trade)
		if view.Depth != filled || view.MaxDepth != int(allowed) {
			t.Errorf("%d entries filled: DepthOf = %+v, want depth %d of %d", filled, view, filled, int(allowed))
		}
	}
}

// A partial fill updates the exchange order it belongs to, so it moves neither
// the depth nor the row the ceiling is read from. Both halves matter: a ladder
// that "deepened" on a partial would be handed a reservation it never earned,
// and a ceiling read one row early would move the band it is measured against.
func TestConfiguredDepthsIgnoresAPartialFill(t *testing.T) {
	rows := []float64{4, 5, 6, 7}

	trade := rowPerDepthTrade(2, rows...)
	trade.History = append(trade.History, aggragates.TradesHistory{
		Type:     "BUY",
		Quantity: 0.4,
		Price:    98,
		OrderId:  2,
	})

	if got := DepthOf(trade); got.Depth != 2 || got.MaxDepth != 6 {
		t.Errorf("DepthOf = %+v, want two filled entries still governed by the third row", got)
	}
}

// Past the last configured row the next fill falls back to the BASE row, never
// to the deepest one — the contract every other ladder read keeps. The base
// row here allows more entries than the last one, so a fallback to the last
// row would be visible instead of hiding behind a coincidence.
func TestConfiguredDepthsFallsBackToTheBaseRowPastTheLastRow(t *testing.T) {
	rows := []float64{9, 5, 6, 7}

	for _, filled := range []int{len(rows), len(rows) + 3} {
		if got := ConfiguredDepths(rowPerDepthTrade(filled, rows...)); got != 9 {
			t.Errorf("%d entries filled: ConfiguredDepths = %d, want the base row (9)", filled, got)
		}
	}
}

// The cost the view carries is measured against the very ceiling it reports,
// so it moves with the row the next fill reads: two ladders identical but for
// how many entries that row allows have different amounts left to pay for,
// and a wallet reserved for the wrong one would be reserved for a tail the
// ladder is not going to place.
func TestDepthOfPricesTheDepthsTheNextRowAllows(t *testing.T) {
	ceilings := func(nextRow float64) []aggragates.StrategySettings {
		return []aggragates.StrategySettings{
			{Percentage: 2.5, Multiplier: 2, Depths: 4},
			{Percentage: 2.5, Multiplier: 2, Depths: 4},
			{Percentage: 2.5, Multiplier: 2, Depths: nextRow},
			{Percentage: 2.5, Multiplier: 2, Depths: 4},
		}
	}

	short := DepthOf(costLadderTrade(2, 10, ceilings(3)...))
	long := DepthOf(costLadderTrade(2, 10, ceilings(6)...))

	if short.MaxDepth != 3 || long.MaxDepth != 6 {
		t.Fatalf("ceilings = %d and %d, want the row of the next fill in each", short.MaxDepth, long.MaxDepth)
	}
	if short.RemainingCost <= 0 {
		t.Fatalf("RemainingCost = %f, want the one entry that row still allows", short.RemainingCost)
	}
	if long.RemainingCost <= short.RemainingCost {
		t.Fatalf("a taller ceiling left %f to pay for, want more than %f", long.RemainingCost, short.RemainingCost)
	}
}

// A ladder that opened raised is, to the wallet, the ladder its rows say it is:
// the whole view of it — depth, ceiling, what it spends, and both costs — is
// exactly the view of the same ladder stored with the raised rows, at every
// depth it can stand at. The stored ceiling is not special: a ladder standing
// on it is not full, it has the depths its opened row added to go, and keeps
// the wallet for them.
func TestDepthOfARaisedLadderIsTheViewOfTheLadderStoredWithTheRaisedRows(t *testing.T) {
	grid := rowsGrid()

	raisedGrid := make([]rowsStep, 0, len(grid))
	for _, step := range grid {
		raisedGrid = append(raisedGrid, rowsStep{multiplier: step.multiplier, percentage: step.percentage + raisePoints})
	}

	const stored = 4

	for filled := 0; filled <= stored+raiseDepths+1; filled++ {
		raised := withOpenedRow(rowsLadder(filled, 16, stored, grid...), raisePoints, raiseDepths)
		asStored := rowsLadder(filled, 16, stored+raiseDepths, raisedGrid...)

		if got, want := DepthOf(raised), DepthOf(asStored); got != want {
			t.Errorf("%d entries filled: DepthOf = %+v, want the view of the ladder stored with the raised rows %+v", filled, got, want)
		}
	}
}

// At the stored ceiling the two readings part: without its opened row the
// ladder is full and reserves nothing, with it the same ladder has the added
// depths left and a reserve to match, and its view says so on every field the
// gate reads.
func TestDepthOfARaisedLadderAtTheStoredCeilingIsNotFull(t *testing.T) {
	grid := rowsGrid()
	stored := rowsLadder(len(grid), 16, float64(len(grid)), grid...)

	full := DepthOf(stored)
	if full.Depth != full.MaxDepth || full.RemainingCost != 0 || full.PlannedRemainingCost != 0 {
		t.Fatalf("DepthOf = %+v, want a full ladder reserving nothing", full)
	}

	raised := DepthOf(withOpenedRow(stored, 0, raiseDepths))
	if raised.Depth != full.Depth {
		t.Errorf("Depth = %d, want the %d entries filled", raised.Depth, full.Depth)
	}
	if raised.MaxDepth != full.MaxDepth+raiseDepths {
		t.Errorf("MaxDepth = %d, want the stored ceiling %d raised by %d", raised.MaxDepth, full.MaxDepth, raiseDepths)
	}
	if raised.RemainingCost <= 0 || raised.PlannedRemainingCost <= 0 {
		t.Errorf("DepthOf = %+v, want a reserve for the depths the opened row added", raised)
	}
	if raised.Asset != full.Asset || raised.TradeID != full.TradeID || raised.Symbol != full.Symbol {
		t.Errorf("DepthOf = %+v, want the same ladder as %+v apart from its ceiling and costs", raised, full)
	}
}

// The raise adds exactly the depths the shipped constant names, and the
// ladder is full again where they end: short of the raised ceiling it still
// has a depth to pay for, on it — and past it — it is full and reserves
// nothing. The opened row is the one the engine writes at open, from the
// shipped amounts, so the ceiling moves with a retune of the constants and the
// ladder stays exactly the depths the constant adds from being full.
func TestDepthOfARaisedLadderIsFullAgainWhereTheShippedDepthsEnd(t *testing.T) {
	grid := rowsGrid()
	stored := len(grid)
	raisedCeiling := stored + dynamicparams.BearDepths

	for filled := stored; filled <= raisedCeiling+1; filled++ {
		trade := withOpenedRow(rowsLadder(filled, 16, float64(stored), grid...), dynamicparams.BearPercentagePoints, dynamicparams.BearDepths)

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

// Where the flag does not shape the ladder, a stray opened row in its logs
// changes nothing: the view of an inverse ladder, a futures strategy, an
// impasse child and a trade with the flag off is exactly the view of the same
// trade without the row — the stored ceiling, full and reserving nothing at
// it, and the stored tail short of it.
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

			got := DepthOf(c.shape(withOpenedRow(plain, raisePoints, raiseDepths)))
			want := DepthOf(c.shape(plain))

			if got != want {
				t.Errorf("%s, %d entries filled: DepthOf = %+v, want the view of the same trade without the opened row %+v", c.name, filled, got, want)
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
