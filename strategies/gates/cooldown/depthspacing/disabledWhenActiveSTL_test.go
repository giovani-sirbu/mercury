package depthspacing

import (
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss"
)

// withDisabledWhenActiveSTL runs the test with the switch at on and puts it back after.
func withDisabledWhenActiveSTL(t *testing.T, on bool) {
	t.Helper()
	previous := disabledWhenActiveSTL
	disabledWhenActiveSTL = on
	t.Cleanup(func() { disabledWhenActiveSTL = previous })
}

// A ladder the gate parks steps through while a smart take loss gate stands triggered on it and the
// switch is on; the hold stands again once the episode closes or the switch is off.
func TestDepthSpacingStepsAsideWhileASmartTakeLossGateStandsTriggered(t *testing.T) {
	trade := testutil.DepthTrade(trade25858[0], trade25858[1])
	trade.Strategy.Params.SmartTakeLoss = true
	tick := trade25858[1].Add(time.Minute)
	if !DepthSpacingHold(tickOf(trade, tick), "stopLoss").Held() {
		t.Fatal("fixture drifted: the gate must park this tick with no episode open")
	}

	fill := trade.History[len(trade.History)-1].Price
	open := smarttakeloss.Append(trade, smarttakeloss.FlatMarketTriggeredRow(fill, nil), tick)
	if !smarttakeloss.Triggered(open) {
		t.Skip("flatmarket.Enabled or flatmarket.Enabled is off: no flat market episode stands")
	}
	closed := smarttakeloss.Append(open, smarttakeloss.FlatMarketResetRow(fill), tick)

	withDisabledWhenActiveSTL(t, true)
	if hold := DepthSpacingHold(tickOf(open, tick), "stopLoss"); hold.Held() {
		t.Fatalf("an open episode must lift the hold, got %q", hold.Reason)
	}
	if !DepthSpacingHold(tickOf(closed, tick), "stopLoss").Held() {
		t.Fatal("a closed episode lifts nothing")
	}

	withDisabledWhenActiveSTL(t, false)
	if !DepthSpacingHold(tickOf(open, tick), "stopLoss").Held() {
		t.Fatal("with disabledWhenActiveSTL off the episode lifts nothing")
	}
}
