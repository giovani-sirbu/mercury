package cooldown

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// hbarTrade carries HBAR/USDT's real ladder row from backtest 112 — the run
// the price release was specified against.
func hbarTrade(step int) aggragates.Trades {
	trade := testutil.DepthTrade(trade25858[:step]...)
	trade.StrategyPair.StrategySettings = []aggragates.StrategySettings{
		{MinDepths: 6, Depths: 8, Percentage: 2.5, Multiplier: 2, Tolerance: 0.2},
	}
	return trade
}

// The numbers the rule was specified with, on trade 32309's first fill of
// 0.5652: a depth that would normally arm 2.7% down has to come 2.5% further
// for each escalation level before the hold lets it through.
func TestReleasePriceAsksOneLadderStepPerEscalationLevel(t *testing.T) {
	const lastFill = 0.5652

	for _, c := range []struct {
		step     int
		wantDrop float64 // percent below the last fill
	}{
		{1, 5.2},  // 2.5 + 0.2 + 2.5
		{2, 7.7},  // 2.5 + 0.2 + 5.0
		{3, 10.2}, // 2.5 + 0.2 + 7.5
	} {
		got, ok := depthSpacingReleasePrice(hbarTrade(1), lastFill, c.step)
		if !ok {
			t.Fatalf("step %d: no release price", c.step)
		}
		want := lastFill * (1 - c.wantDrop/100)
		if diff := got - want; diff > 1e-9 || diff < -1e-9 {
			t.Errorf("step %d release = %.6f, want %.6f (−%.1f%%)", c.step, got, want, c.wantDrop)
		}
	}
}

func TestPriceReleaseLiftsTheHoldOnlyOnceTheDropArrives(t *testing.T) {
	trade := hbarTrade(1)
	const lastFill = 0.5652
	release, _ := depthSpacingReleasePrice(trade, lastFill, 1) // 0.535810

	if depthSpacingPriceReleased(trade, 0.5519, lastFill, 1) {
		t.Error("0.5519 is where the depth would have armed anyway; the hold must stand")
	}
	if depthSpacingPriceReleased(trade, release+1e-6, lastFill, 1) {
		t.Error("a hair above the release price must still hold")
	}
	if !depthSpacingPriceReleased(trade, release, lastFill, 1) {
		t.Error("exactly at the release price the depth must be admitted")
	}
	if !depthSpacingPriceReleased(trade, 0.50, lastFill, 1) {
		t.Error("well past the release price the depth must be admitted")
	}
}

// A deeper escalation demands a deeper discount: the price that bought the
// first hold out is not enough for the second.
func TestPriceReleaseGetsHarderAsTheEscalationClimbs(t *testing.T) {
	trade := hbarTrade(2)
	const lastFill = 0.5652
	atStepOne, _ := depthSpacingReleasePrice(trade, lastFill, 1)

	if !depthSpacingPriceReleased(trade, atStepOne, lastFill, 1) {
		t.Fatal("step 1 must release at its own price")
	}
	if depthSpacingPriceReleased(trade, atStepOne, lastFill, 2) {
		t.Error("step 2 asks 2.5% more; the step-1 price must not release it")
	}
}

// An inverse trade enters on SELL, so its ladder climbs: the release is the
// same distance, in the other direction.
func TestPriceReleaseFollowsTheInverseLadderUpwards(t *testing.T) {
	trade := hbarTrade(1)
	trade.Inverse = true
	const lastFill = 0.5652

	release, ok := depthSpacingReleasePrice(trade, lastFill, 1)
	if !ok {
		t.Fatal("no release price")
	}
	if release <= lastFill {
		t.Fatalf("inverse release = %.6f, must be above the last fill %.4f", release, lastFill)
	}
	if depthSpacingPriceReleased(trade, lastFill, lastFill, 1) {
		t.Error("the entry price itself must not release an inverse hold")
	}
	if !depthSpacingPriceReleased(trade, release, lastFill, 1) {
		t.Error("at the release price the inverse depth must be admitted")
	}
}

// Fail closed on unreadable inputs: without settings or a price there is no
// discount to measure, so the clock keeps the hold rather than a zero
// threshold releasing every depth.
func TestPriceReleaseNeedsAReadableLadderRow(t *testing.T) {
	noSettings := testutil.DepthTrade(trade25858[0])
	noSettings.StrategyPair.StrategySettings = nil
	if _, ok := depthSpacingReleasePrice(noSettings, 0.5652, 1); ok {
		t.Error("a trade with no strategy settings has no release price")
	}
	if depthSpacingPriceReleased(noSettings, 0.01, 0.5652, 1) {
		t.Error("no settings must not release the hold at any price")
	}

	zeroPct := hbarTrade(1)
	zeroPct.StrategyPair.StrategySettings[0].Percentage = 0
	if _, ok := depthSpacingReleasePrice(zeroPct, 0.5652, 1); ok {
		t.Error("a zero percentage has no ladder step to ask for")
	}

	trade := hbarTrade(1)
	if depthSpacingPriceReleased(trade, 0, 0.5652, 1) {
		t.Error("an unknown tick price must not release the hold")
	}
	if _, ok := depthSpacingReleasePrice(trade, 0, 1); ok {
		t.Error("an unknown last fill has no reference to discount from")
	}
	if _, ok := depthSpacingReleasePrice(trade, 0.5652, 0); ok {
		t.Error("no escalation level means no hold to release")
	}
}

// A discount that would take the price to zero or below is not a release, it
// is a broken settings row.
func TestPriceReleaseRefusesAnImpossibleDiscount(t *testing.T) {
	trade := hbarTrade(1)
	trade.StrategyPair.StrategySettings[0].Percentage = 60
	if _, ok := depthSpacingReleasePrice(trade, 0.5652, 2); ok {
		t.Error("a 180% discount must be refused, not wrapped into a negative price")
	}
}
