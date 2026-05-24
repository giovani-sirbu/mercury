package tests

import (
	"math"
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/actions"
)

// TestFees_BaseAssetFeeReturnsQuoteEquivalent pins GetFees for fees paid in
// the BASE asset (BTC) on a spot BTC/USDC trade. Result is feesInQuote =
// fee * historyData.Price.
func TestFees_BaseAssetFeeReturnsQuoteEquivalent(t *testing.T) {
	trade := scenarioBuildTrade("closed", 100000, false)
	scenarioAppendHistory(&trade, "BUY", 0.001, 100000, "BTC", 0.000001)
	scenarioAppendHistory(&trade, "sell", 0.001, 102000, "BTC", 0.000001)

	got := actions.GetFees(events.Events{Trade: trade})
	// feesInQuote = 0.000001*100000 + 0.000001*102000 = 0.1 + 0.102 = 0.202
	const want = 0.202
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("GetFees base asset = %v, want %v", got, want)
	}
}

// TestFees_QuoteAssetFeeReturnsDirectly pins GetFees for fees paid in the
// QUOTE asset (USDC).
func TestFees_QuoteAssetFeeReturnsDirectly(t *testing.T) {
	trade := scenarioBuildTrade("closed", 100000, false)
	scenarioAppendHistory(&trade, "BUY", 0.001, 100000, "USDC", 0.1)
	scenarioAppendHistory(&trade, "sell", 0.001, 102000, "USDC", 0.102)

	got := actions.GetFees(events.Events{Trade: trade})
	const want = 0.202
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("GetFees quote asset = %v, want %v", got, want)
	}
}

// TestFees_ThirdAssetUsesWsPricesSnapshot covers the third-asset fee path
// (BNB) routed through event.WsPrices to avoid a real exchange call.
func TestFees_ThirdAssetUsesWsPricesSnapshot(t *testing.T) {
	trade := scenarioBuildTrade("closed", 100000, false)
	trade.ProfitAsset = "USDC"
	scenarioAppendHistory(&trade, "BUY", 0.001, 100000, "BNB", 0.005)
	scenarioAppendHistory(&trade, "sell", 0.001, 102000, "BNB", 0.005)

	event := events.Events{
		Trade: trade,
		WsPrices: map[string]float64{
			"BNB/USDC": 500, // BNB price in USDC
		},
	}

	got := actions.GetFees(event)
	// feesInQuote = 0.005 * 500 + 0.005 * 500 = 5.0
	const want = 5.0
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("GetFees third-asset = %v, want %v", got, want)
	}
}

// TestFees_InverseReturnsBaseCurrencyTotal mirrors the prior tests for an
// inverse trade: the returned figure is feesInBase, not feesInQuote.
func TestFees_InverseReturnsBaseCurrencyTotal(t *testing.T) {
	trade := scenarioBuildTrade("closed", 100000, true)
	scenarioAppendHistory(&trade, "SELL", 0.001, 100000, "BTC", 0.000001)
	scenarioAppendHistory(&trade, "buy", 0.001, 99000, "BTC", 0.000001)

	got := actions.GetFees(events.Events{Trade: trade})
	const want = 0.000002 // sum of base-asset fees, unchanged
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("GetFees inverse = %v, want %v", got, want)
	}
}

// TestFees_MixedAssetFeesAcrossHistory covers a realistic Binance scenario
// where fees alternate between BNB (when the user holds a BNB balance for
// discounts) and USDC (when BNB balance dips to zero).
func TestFees_MixedAssetFeesAcrossHistory(t *testing.T) {
	trade := scenarioBuildTrade("closed", 100000, false)
	trade.ProfitAsset = "USDC"
	scenarioAppendHistory(&trade, "BUY", 0.001, 100000, "BNB", 0.001)
	scenarioAppendHistory(&trade, "BUY", 0.002, 98000, "USDC", 0.2)
	scenarioAppendHistory(&trade, "sell", 0.003, 102000, "BNB", 0.003)

	event := events.Events{
		Trade:    trade,
		WsPrices: map[string]float64{"BNB/USDC": 500},
	}

	got := actions.GetFees(event)
	// BNB row 1: 0.001 * 500 = 0.5
	// USDC row 2: 0.2
	// BNB row 3: 0.003 * 500 = 1.5
	// total = 2.2
	const want = 2.2
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("GetFees mixed assets = %v, want %v", got, want)
	}
}

// TestFees_CalculateFeesOldSeparatesBaseAndQuote ensures the older helper
// returns the two buckets independently — required by Sell + HasFunds.
func TestFees_CalculateFeesOldSeparatesBaseAndQuote(t *testing.T) {
	trade := scenarioBuildTrade("closed", 100000, false)
	scenarioAppendHistory(&trade, "BUY", 0.001, 100000, "BTC", 0.000001)
	scenarioAppendHistory(&trade, "BUY", 0.002, 98000, "USDC", 0.2)
	scenarioAppendHistory(&trade, "sell", 0.001, 102000, "BTC", 0.000001)

	feeInBase, feeInQuote := actions.CalculateFeesOld(events.Events{Trade: trade})
	if math.Abs(feeInBase-0.000002) > 1e-12 {
		t.Errorf("feeInBase = %v, want 0.000002", feeInBase)
	}
	if math.Abs(feeInQuote-0.2) > 1e-9 {
		t.Errorf("feeInQuote = %v, want 0.2", feeInQuote)
	}
}

// TestFees_HasProfitDoublesFeesForRoundTripSimulation pins the doubling
// HasProfit applies to fees: it adds a simulated sell row and expects the
// realised round-trip cost to be ~2x the historic accumulator. With one
// buy at 100k charging 0.1 USDC fee, simulated sell at 100.74k (after the
// 0.25% tolerance haircut from 101k), and doubled fees = 0.2 USDC, the net
// profit comes out to ~0.54 USDC. Just asserts profit > 0 here; precise
// math is covered in the unit tests inside actions/.
func TestFees_HasProfitDoublesFeesForRoundTripSimulation(t *testing.T) {
	trade := scenarioBuildTrade("takeProfit", 101000, false)
	scenarioAppendHistory(&trade, "BUY", 0.001, 100000, "USDC", 0.1)

	event := events.Events{Trade: trade}
	got, err := actions.HasProfit(event)
	if err != nil {
		t.Fatalf("HasProfit returned error: %v", err)
	}
	if got.Trade.Profit <= 0 {
		t.Errorf("expected positive net profit after doubled fees, got %v", got.Trade.Profit)
	}
}
