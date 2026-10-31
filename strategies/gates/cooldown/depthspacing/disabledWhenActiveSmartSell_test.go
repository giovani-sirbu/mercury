package depthspacing

import (
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell"
)

// withDisabledWhenActiveSTL runs the test with the switch at on and puts it back after.
func withDisabledWhenActiveSTL(t *testing.T, on bool) {
	t.Helper()
	previous := disabledWhenActiveSmartSell
	disabledWhenActiveSmartSell = on
	t.Cleanup(func() { disabledWhenActiveSmartSell = previous })
}

// A ladder the gate parks steps through while a smart take loss gate stands triggered on it and the
// switch is on; the hold stands again once the episode closes or the switch is off.
func TestDepthSpacingStepsAsideWhileASmartSellGateStandsTriggered(t *testing.T) {
	trade := testutil.DepthTrade(trade25858[0], trade25858[1])
	trade.Strategy.Params.SmartSell = true
	tick := trade25858[1].Add(time.Minute)
	if !DepthSpacingHold(tickOf(trade, tick), "trailingBuy").Held() {
		t.Fatal("fixture drifted: the gate must park this tick with no episode open")
	}

	fill := trade.History[len(trade.History)-1].Price
	open := smartsell.Append(trade, smartsell.FlatMarketTriggeredRow(fill, nil), tick)
	if !smartsell.Triggered(open) {
		t.Skip("flatmarket.Enabled or flatmarket.Enabled is off: no flat market episode stands")
	}
	closed := smartsell.Append(open, smartsell.FlatMarketResetRow(fill), tick)

	withDisabledWhenActiveSTL(t, true)
	if hold := DepthSpacingHold(tickOf(open, tick), "trailingBuy"); hold.Held() {
		t.Fatalf("an open episode must lift the hold, got %q", hold.Reason)
	}
	if !DepthSpacingHold(tickOf(closed, tick), "trailingBuy").Held() {
		t.Fatal("a closed episode lifts nothing")
	}

	withDisabledWhenActiveSTL(t, false)
	if !DepthSpacingHold(tickOf(open, tick), "trailingBuy").Held() {
		t.Fatal("with disabledWhenActiveSmartSell off the episode lifts nothing")
	}
}
