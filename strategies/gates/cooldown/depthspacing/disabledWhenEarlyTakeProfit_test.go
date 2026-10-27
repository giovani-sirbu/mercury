package depthspacing

import (
	"testing"
	"time"

	"github.com/giovani-sirbu/mercury/internal/testutil"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss"
)

// withDisabledWhenEarlyTakeProfit runs the test with the switch at on and puts it back after.
func withDisabledWhenEarlyTakeProfit(t *testing.T, on bool) {
	t.Helper()
	previous := disabledWhenEarlyTakeProfit
	disabledWhenEarlyTakeProfit = on
	t.Cleanup(func() { disabledWhenEarlyTakeProfit = previous })
}

// A ladder the gate parks steps through once the early take profit stands latched on it and the
// switch is on, the STL switch off or not; the hold stands on an unlatched ladder or with the switch off.
func TestDepthSpacingStepsAsideWhileTheEarlyTakeProfitStandsLatched(t *testing.T) {
	trade := testutil.DepthTrade(trade25858[0], trade25858[1])
	trade.Strategy.Params.SmartTakeLoss = true
	tick := trade25858[1].Add(time.Minute)
	if !DepthSpacingHold(tickOf(trade, tick), "stopLoss").Held() {
		t.Fatal("fixture drifted: the gate must park this tick with no latch on the ladder")
	}

	fill := trade.History[len(trade.History)-1].Price
	latched := smarttakeloss.Append(trade, smarttakeloss.LatchedRow(fill, smarttakeloss.Finding{Feature: smarttakeloss.FeatureEarlyTakeProfit}), tick)
	if !smarttakeloss.EarlyTakeProfitLatched(latched) {
		t.Skip("earlytakeprofit.Enabled is off: no ladder is latched")
	}

	withDisabledWhenActiveSTL(t, false)
	withDisabledWhenEarlyTakeProfit(t, true)
	if hold := DepthSpacingHold(tickOf(latched, tick), "stopLoss"); hold.Held() {
		t.Fatalf("a latched ladder must lift the hold, got %q", hold.Reason)
	}
	if !DepthSpacingHold(tickOf(trade, tick), "stopLoss").Held() {
		t.Fatal("an unlatched ladder lifts nothing")
	}

	withDisabledWhenEarlyTakeProfit(t, false)
	if !DepthSpacingHold(tickOf(latched, tick), "stopLoss").Held() {
		t.Fatal("with disabledWhenEarlyTakeProfit off the latch lifts nothing")
	}
}
