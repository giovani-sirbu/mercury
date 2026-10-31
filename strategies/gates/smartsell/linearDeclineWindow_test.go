package smartsell

import (
	"testing"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/lineardecline"
)

// The gate reads the newest lineardecline.Window fills alone: a ladder whose fills before the
// window sit flat at the first price, their bars' closes with them, latches on the line its newest
// fills fell on, the reading framed from the window's first fill.
func TestTheLinearDeclineLatchesOnTheNewestWindowFillsWhateverTheStartDid(t *testing.T) {
	onlyTheLinearDecline(t)
	depth := linearDepth + 2
	fills := linearFills(linearSlope, depth)
	block := linearBlock(linearSlope, depth, linearTail, linearTrendMet)
	from := depth - lineardecline.Window
	for index := range from {
		fills[index].Price = linearFirstPrice
	}
	for index := range linearLead + from*linearGap {
		block.SeriesCloses[index] = linearFirstPrice
	}
	trade := testutil.LadderTrade(false, fills...)
	if reading := linearReading(t, trade, block); reading.FromDepth != from+1 || reading.ToDepth != depth {
		t.Fatalf("the reading runs from depth %d to %d, got %+v", from+1, depth, reading)
	}
	got := Apply(trade, "", trade.PositionPrice, withBlock(block))
	assertRow(t, got, linearRow(t, trade, block))
	assertNoSale(t, got, "")
}
