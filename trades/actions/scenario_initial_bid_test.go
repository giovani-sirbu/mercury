package actions_test

import (
	"math"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/actions"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// TestInitialBid_SpotFirstBuyUsesConfiguredFraction proves that when
// StrategySettings.InitialBid > 0 and the trade has no history, Buy
// bypasses the wallet lookup and computes quantity from MinNotional *
// InitialBid. With InitialBid=2 and MinNotional=5, qty_in_quote = 10 USDC,
// /PositionPrice (100000) = 0.0001 BTC. ToFixed(5) = 0.0001.
func TestInitialBid_SpotFirstBuyUsesConfiguredFraction(t *testing.T) {
	trade := scenarioBuildTrade("buy", 100000, false)
	// Override the default settings to set an explicit InitialBid.
	trade.StrategyPair.StrategySettings = []aggragates.StrategySettings{
		{
			MinDepths:  6,
			Depths:     8,
			Percentage: 2,
			Multiplier: 2,
			Tolerance:  0.25,
			InitialBid: 2,
		},
	}

	// Wallet does not matter — InitialBid path skips GetUserAssets.
	event := scenarioBuildEvent(trade, "USDC", "0")

	got, err := actions.Buy(event)
	if err != nil {
		t.Fatalf("Buy returned error: %v", err)
	}
	// minNotion = 5 / 100000 = 0.00005, qty = 0.00005 * 2 = 0.0001
	const want = 0.0001
	if math.Abs(got.Params.Quantity-want) > 1e-9 {
		t.Errorf("Buy qty with InitialBid = %v, want %v", got.Params.Quantity, want)
	}
}

// TestInitialBid_InverseFirstBuyUsesConfiguredFraction mirrors the spot
// InitialBid path for inverse trades. With InitialBid=2 and MinNotional=5,
// qty_in_quote = 10 USDC, /PositionPrice (100000) = 0.0001 BTC. The min-order
// clamp (CalculateMinOrderQty) yields 5/100000 = 0.00005, which loses the
// math.Max comparison so the InitialBid result is preserved.
func TestInitialBid_InverseFirstBuyUsesConfiguredFraction(t *testing.T) {
	trade := scenarioBuildTrade("buy", 100000, true)
	trade.StrategyPair.StrategySettings = []aggragates.StrategySettings{
		{
			MinDepths:  6,
			Depths:     8,
			Percentage: 2,
			Multiplier: 2,
			Tolerance:  0.25,
			InitialBid: 2,
		},
	}

	event := scenarioBuildEvent(trade, "BTC", "10")

	got, err := actions.Buy(event)
	if err != nil {
		t.Fatalf("Buy returned error: %v", err)
	}
	// minNotion = 5 / 100000 = 0.00005, qty = 0.00005 * 2 = 0.0001
	const want = 0.0001
	if math.Abs(got.Params.Quantity-want) > 1e-9 {
		t.Errorf("Buy qty (inverse, InitialBid) = %v, want %v", got.Params.Quantity, want)
	}
}

// TestInitialBid_SubsequentBuyAppliesMultiplier checks the post-history
// branch in Buy: with one prior buy of 0.001 BTC and Multiplier=2, the
// next buy quantity is 0.001 * 2 = 0.002.
func TestInitialBid_SubsequentBuyAppliesMultiplier(t *testing.T) {
	trade := scenarioBuildTrade("buy", 98000, false)
	scenarioAppendHistory(&trade, "BUY", 0.001, 100000, "", 0)

	event := scenarioBuildEvent(trade, "USDC", "1000")

	got, err := actions.Buy(event)
	if err != nil {
		t.Fatalf("Buy returned error: %v", err)
	}
	const want = 0.002
	if math.Abs(got.Params.Quantity-want) > 1e-9 {
		t.Errorf("Buy qty (subsequent, multiplier) = %v, want %v", got.Params.Quantity, want)
	}
}

// TestInitialBid_SettingsIndexClampsAtMaxDepth verifies that once the
// history count exceeds the length of StrategySettings, the engine reuses
// the last settings entry. With one settings row and three history rows,
// the second buy still applies Multiplier=2 against the latest entry.
func TestInitialBid_SettingsIndexClampsAtMaxDepth(t *testing.T) {
	trade := scenarioBuildTrade("buy", 94120, false)
	scenarioAppendHistory(&trade, "BUY", 0.001, 100000, "", 0)
	scenarioAppendHistory(&trade, "BUY", 0.002, 98000, "", 0)
	scenarioAppendHistory(&trade, "BUY", 0.004, 96040, "", 0)

	event := scenarioBuildEvent(trade, "USDC", "1500")

	got, err := actions.Buy(event)
	if err != nil {
		t.Fatalf("Buy returned error: %v", err)
	}
	const want = 0.008 // 0.004 * 2 (last history qty * multiplier)
	if math.Abs(got.Params.Quantity-want) > 1e-9 {
		t.Errorf("Buy qty at clamped settings index = %v, want %v", got.Params.Quantity, want)
	}
}
