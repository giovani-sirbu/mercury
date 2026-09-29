package smarttakeloss

import (
	"slices"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// sellChain is the `sell` chain every engine maps the position to.
func sellChain() []string {
	return []string{"cancelPendingOrder", "hasProfit", "sell", "updateTrade"}
}

// sameSlice is whether got is the very slice the engine handed in, its
// backing array included.
func sameSlice(got, handed []string) bool {
	return len(got) == len(handed) && (len(got) == 0 || &got[0] == &handed[0])
}

// On a latched ladder the `sell` chain runs acceptLoss where it runs
// hasProfit, in a new slice: the engine's own is left as it was, the rest of
// the chain keeps its order, and every hasProfit is swapped. The position
// type the engine already rewrote plays no part.
func TestSaleActionsSwapsTheProfitGateOnALatchedSale(t *testing.T) {
	trade := latchedBy(watchedTrade())
	for _, positionType := range []string{"buy", "sell", "takeProfit", ""} {
		trade.PositionType = positionType
		chain := sellChain()
		got := SaleActions(trade, "sell", chain)
		if want := []string{"cancelPendingOrder", "acceptLoss", "sell", "updateTrade"}; !slices.Equal(got, want) {
			t.Fatalf("state %q: got %q, want %q", positionType, got, want)
		}
		if !slices.Equal(chain, sellChain()) || sameSlice(got, chain) {
			t.Fatalf("state %q: the engine's slice must stay as it was, got %q", positionType, chain)
		}
	}
	if got := SaleActions(trade, "sell", []string{"hasProfit", "sell", "hasProfit"}); !slices.Equal(got, []string{"acceptLoss", "sell", "acceptLoss"}) {
		t.Fatalf("every hasProfit is swapped, got %q", got)
	}
}

// Every other chain comes back as the engine's own slice: the take profit's
// arming chains, the sellLoss chain and the adds on a latched ladder, and the
// `sell` chain on a ladder that is not latched — watched without the row,
// pending without it, the row without a price, a ladder the rule does not
// watch carrying the row, a child, a strategy without the flag, and every
// ladder while IndecisionDirection is off.
func TestSaleActionsLeavesEveryOtherChainAlone(t *testing.T) {
	latched := latchedBy(watchedTrade())
	for _, position := range []string{"takeProfit", "update_takeProfit", "sellLoss", "sellParent", "buy", "stopLoss", "update_buy", ""} {
		chain := []string{"shouldHold", "hasFunds", "hasProfit", "cancelPendingOrder", "updateTrade"}
		if got := SaleActions(latched, position, chain); !sameSlice(got, chain) || !slices.Equal(got, []string{"shouldHold", "hasFunds", "hasProfit", "cancelPendingOrder", "updateTrade"}) {
			t.Errorf("%q on a latched ladder must come back as it was, got %q", position, got)
		}
	}

	unpriced := watchedTrade()
	unpriced.Logs = []aggragates.TradesLogs{{Message: IndecisionMessage("buy", indecisionReasons)}}
	futures := latchedBy(watchedTrade())
	futures.Strategy.TradeType = aggragates.Futures
	child := latchedBy(watchedTrade())
	child.ParentID = 7
	off := latchedBy(watchedTrade())
	off.Strategy.Params.SmartTakeLoss = false
	for name, trade := range map[string]aggragates.Trades{
		"a watched ladder without the row":   watchedTrade(),
		"a pending ladder without the row":   pendingTrade(),
		"the row without a price":            unpriced,
		"one short of IndecisionArmDepth":    latchedBy(testutil.LadderTrade(false, fills(IndecisionArmDepth-1, "17:38:00")...)),
		"an inverse ladder carrying the row": latchedBy(testutil.LadderTrade(true, fills(watchedFills, "17:38:00")...)),
		"a futures ladder carrying the row":  futures,
		"a child":                            child,
		"a strategy without the flag":        off,
	} {
		chain := sellChain()
		if got := SaleActions(trade, "sell", chain); !sameSlice(got, chain) || !slices.Equal(got, sellChain()) {
			t.Errorf("%s: the sell chain must come back as it was, got %q", name, got)
		}
	}

	withIndecisionDirection(t, false)
	chain := sellChain()
	if got := SaleActions(latched, "sell", chain); !sameSlice(got, chain) {
		t.Fatalf("switched off, the sell chain must come back as it was, got %q", got)
	}
}
