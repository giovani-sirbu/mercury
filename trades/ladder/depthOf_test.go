package ladder

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// The wallet view of a trade is exactly what the two folds say about it:
// every surface builds it here so none of them can count a depth its own way.
func TestDepthOfMirrorsTheLadderFolds(t *testing.T) {
	trade := depthTrade(5, aggragates.StrategySettings{Percentage: 2.5, Depths: 8})

	got := DepthOf(trade)

	if got.TradeID != trade.ID || got.Symbol != trade.Symbol {
		t.Errorf("DepthOf = %+v, want the trade's own id and symbol", got)
	}
	if got.Depth != CountFilledEntries(trade) {
		t.Errorf("Depth = %d, want CountFilledEntries %d", got.Depth, CountFilledEntries(trade))
	}
	if got.MaxDepth != ConfiguredDepths(trade) {
		t.Errorf("MaxDepth = %d, want ConfiguredDepths %d", got.MaxDepth, ConfiguredDepths(trade))
	}
}

// Partial fills update the same exchange order and are one depth, not two,
// and an impasse child's profit transfer is bookkeeping rather than an
// entry — the membership rule is CountFilledEntries', not a row count.
func TestDepthOfCountsDistinctOrdersOnly(t *testing.T) {
	trade := depthTrade(2, aggragates.StrategySettings{Depths: 8})
	trade.History = append(trade.History,
		aggragates.TradesHistory{Type: "BUY", Quantity: 0.5, Price: 99, OrderId: 2},
		aggragates.TradesHistory{Type: "BUY", Quantity: 3, Price: AccountingPriceCeiling / 10, OrderId: 77},
		aggragates.TradesHistory{Type: "SELL", Quantity: 2, Price: 120, OrderId: 78},
	)

	if got := DepthOf(trade).Depth; got != 2 {
		t.Errorf("Depth = %d, want the two distinct entries", got)
	}
}

// An inverse ladder enters on SELL, so its depth is counted on the other
// side. A wallet view that read only BUYs would report every inverse trade at
// depth 0 and hand it the priority it never earned.
func TestDepthOfReadsTheInverseEntrySide(t *testing.T) {
	trade := depthTrade(3, aggragates.StrategySettings{Depths: 8})
	trade.Inverse = true
	for index := range trade.History {
		trade.History[index].Type = "SELL"
	}

	if got := DepthOf(trade).Depth; got != 3 {
		t.Errorf("Depth = %d, want the three SELL entries of an inverse ladder", got)
	}
}
