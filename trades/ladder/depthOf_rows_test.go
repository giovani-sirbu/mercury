package ladder

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
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
