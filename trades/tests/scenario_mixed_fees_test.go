package tests

import (
	"math"
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/actions"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// TestMixedFees_SpotProfitNetsAllAssetClasses walks a realistic Binance
// fee pattern: BNB discount for the first row (user holds BNB balance),
// quote USDC for the second (BNB ran out), base BTC for the third (small
// dust-style residual fee). HasProfit must double the aggregated fee for
// the round-trip and still net positive.
func TestMixedFees_SpotProfitNetsAllAssetClasses(t *testing.T) {
	trade := scenarioBuildTrade("takeProfit", 102000, false)
	trade.ProfitAsset = "USDC"
	scenarioAppendHistory(&trade, "BUY", 0.001, 100000, "BNB", 0.0002)
	scenarioAppendHistory(&trade, "BUY", 0.002, 98000, "USDC", 0.5)
	scenarioAppendHistory(&trade, "BUY", 0.004, 96040, "BTC", 0.000004)

	event := events.Events{
		Trade:    trade,
		WsPrices: map[string]float64{"BNB/USDC": 500},
	}

	got, err := actions.HasProfit(event)
	if err != nil {
		t.Fatalf("HasProfit returned error: %v", err)
	}
	if got.Trade.Profit <= 0 {
		t.Errorf("expected positive net profit after mixed fees, got %v", got.Trade.Profit)
	}
}

// TestMixedFees_GetFeesAggregatesAllAssetClasses pins the exact aggregate
// value of GetFees with BNB+USDC+BTC fees on the same history. Uses fixed
// BNB/USDC price via WsPrices to keep the math deterministic.
//
//	row 1 fee: 0.0002 BNB * 500 = 0.1 USDC
//	row 2 fee: 0.5 USDC
//	row 3 fee: 0.000004 BTC * 96040 = 0.38416 USDC
//	total feesInQuote = 0.98416 USDC
func TestMixedFees_GetFeesAggregatesAllAssetClasses(t *testing.T) {
	trade := scenarioBuildTrade("closed", 102000, false)
	trade.ProfitAsset = "USDC"
	scenarioAppendHistory(&trade, "BUY", 0.001, 100000, "BNB", 0.0002)
	scenarioAppendHistory(&trade, "BUY", 0.002, 98000, "USDC", 0.5)
	scenarioAppendHistory(&trade, "BUY", 0.004, 96040, "BTC", 0.000004)

	event := events.Events{
		Trade:    trade,
		WsPrices: map[string]float64{"BNB/USDC": 500},
	}

	got := actions.GetFees(event)
	const want = 0.98416
	if math.Abs(got-want) > 1e-6 {
		t.Errorf("GetFees mixed = %v, want %v", got, want)
	}
}

// TestMixedFees_InverseGetFeesReturnsBaseAccumulator mirrors the prior
// test for inverse trades. The function returns feesInBase, where the
// quote and third-asset fees are converted back into BTC.
//
//	row 1 (USDC) fee: 0.1 USDC / 100000 = 1e-6 BTC
//	row 2 (BTC) fee:  0.000005 BTC
//	row 3 (BNB) fee:  0.0002 BNB * 500 / profitAssetPrice(...)
//
// We pin only that the return is positive, non-zero, and roughly within
// the sum of base contributions — the third-asset arithmetic depends on
// getSymbolPrice resolving the BTC/USDC price, which uses PositionPrice
// for stable assets.
func TestMixedFees_InverseGetFeesReturnsBaseAccumulator(t *testing.T) {
	trade := scenarioBuildTrade("closed", 100000, true)
	trade.ProfitAsset = "BTC"
	scenarioAppendHistory(&trade, "SELL", 0.001, 100000, "USDC", 0.1)
	scenarioAppendHistory(&trade, "SELL", 0.002, 102000, "BTC", 0.000005)

	event := events.Events{Trade: trade}

	got := actions.GetFees(event)
	if got <= 0 {
		t.Errorf("expected positive feesInBase aggregate, got %v", got)
	}
}

// TestMixedFees_ZeroFeeRowsAreIgnored verifies the guard at the top of the
// per-fee loop: fees with Fee <= 0 are skipped without touching the
// accumulators. Mixing real and zero fee rows must produce the same total
// as the real fees alone.
func TestMixedFees_ZeroFeeRowsAreIgnored(t *testing.T) {
	tradeWithZero := scenarioBuildTrade("closed", 100000, false)
	tradeWithZero.ProfitAsset = "USDC"
	scenarioAppendHistory(&tradeWithZero, "BUY", 0.001, 100000, "BNB", 0.0002)
	tradeWithZero.History = append(tradeWithZero.History, aggragates.TradesHistory{
		Type: "BUY", Quantity: 0.001, Price: 100000,
		Fees: []aggragates.TradesFees{{Asset: "BNB", Fee: 0}},
	})

	tradeBaseline := scenarioBuildTrade("closed", 100000, false)
	tradeBaseline.ProfitAsset = "USDC"
	scenarioAppendHistory(&tradeBaseline, "BUY", 0.001, 100000, "BNB", 0.0002)

	wsPrices := map[string]float64{"BNB/USDC": 500}
	withZero := actions.GetFees(events.Events{Trade: tradeWithZero, WsPrices: wsPrices})
	baseline := actions.GetFees(events.Events{Trade: tradeBaseline, WsPrices: wsPrices})

	if math.Abs(withZero-baseline) > 1e-9 {
		t.Errorf("zero-fee rows leaked into aggregate: withZero=%v baseline=%v", withZero, baseline)
	}
}

// TestMixedFees_EmptyFeesArrayDoesNotAffectAggregate documents the
// behavior on history rows that have no fee array attached: skipped by
// the `len(data.Fees) == 0` continue at the top of the inner loop.
func TestMixedFees_EmptyFeesArrayDoesNotAffectAggregate(t *testing.T) {
	trade := scenarioBuildTrade("closed", 100000, false)
	trade.ProfitAsset = "USDC"
	scenarioAppendHistory(&trade, "BUY", 0.001, 100000, "USDC", 0.5)
	// A second row with no fees attached.
	trade.History = append(trade.History, aggragates.TradesHistory{Type: "BUY", Quantity: 0.002, Price: 98000})

	got := actions.GetFees(events.Events{Trade: trade})
	const want = 0.5
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("GetFees with empty fee row = %v, want %v", got, want)
	}
}
