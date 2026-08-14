package actions

import "testing"

func TestGetDefaultActionsRegistersExpectedActions(t *testing.T) {
	expected := []string{
		"updateTrade",
		"cancelPendingOrder",
		"hasFunds",
		"buy",
		"sell",
		"sellAll",
		"hasProfit",
		"acceptLoss",
		"createChildrenTrades",
		"parentTradeHasProfit",
		"shouldHold",
		"hasEnoughFunds",
		"regulatePriceChange",
		"closeFuturesTrade",
		"checkOldFuturesOrders",
		"createFuturesOrders",
		"closeOrKeepALiveTrade",
		"updateStopLossOrder",
		"checkFuturesOrderHealth",
	}

	got := GetDefaultActions()
	if len(got) != len(expected) {
		t.Errorf("default actions count = %d, want %d", len(got), len(expected))
	}
	for _, key := range expected {
		fn, ok := got[key]
		if !ok {
			t.Errorf("missing default action %q", key)
			continue
		}
		if fn == nil {
			t.Errorf("default action %q is nil", key)
		}
	}
}
