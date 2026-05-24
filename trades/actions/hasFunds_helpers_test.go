package actions

import (
	"math"
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func TestGetAssetBudgetReturnsFreeAmountForMatchingAsset(t *testing.T) {
	assets := []aggregates.UserAssetRecord{
		{Asset: "USDT", Free: "1000.5"},
		{Asset: "BTC", Free: "0.25"},
	}

	if got := GetAssetBudget(assets, "BTC"); got != 0.25 {
		t.Errorf("GetAssetBudget(BTC) = %v, want 0.25", got)
	}
	if got := GetAssetBudget(assets, "USDT"); got != 1000.5 {
		t.Errorf("GetAssetBudget(USDT) = %v, want 1000.5", got)
	}
}

func TestGetAssetBudgetReturnsZeroForMissingAsset(t *testing.T) {
	assets := []aggregates.UserAssetRecord{{Asset: "USDT", Free: "100"}}

	if got := GetAssetBudget(assets, "ETH"); got != 0 {
		t.Errorf("GetAssetBudget for missing asset = %v, want 0", got)
	}
}

func TestGetUsedQuantitiesSpotSubtractsSellAndFeeFromBuy(t *testing.T) {
	trade := aggragates.Trades{Symbol: "BTC/USDT"}
	trade.History = []aggragates.TradesHistory{
		{Type: "buy", Quantity: 2, Price: 100, Fees: []aggragates.TradesFees{{Asset: "BTC", Fee: 0.01}}},
		{Type: "sell", Quantity: 0.5, Price: 110},
	}
	trade.StrategyPair.TradeFilters = aggragates.TradeFilters{LotSize: 2}

	got := GetUsedQuantities(events.Events{Trade: trade})
	const want = 1.49 // 2 - 0.5 - 0.01
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("GetUsedQuantities spot = %v, want %v", got, want)
	}
}

// TestGetUsedQuantitiesIgnoresUSDCFeesOnBase pins the post-fix behavior: a
// USDC-paid fee in the history must NOT reduce the BTC wallet calculation.
// Pre-fix (CalculateFees was GetFeesBaseQuote with cross-conversion), a USDC
// fee would have cross-converted into a non-zero feeInBase and over-deducted.
func TestGetUsedQuantitiesIgnoresUSDCFeesOnBase(t *testing.T) {
	trade := aggragates.Trades{Symbol: "BTC/USDC"}
	trade.History = []aggragates.TradesHistory{
		{Type: "buy", Quantity: 2, Price: 100, Fees: []aggragates.TradesFees{{Asset: "USDC", Fee: 0.5}}},
	}
	trade.StrategyPair.TradeFilters = aggragates.TradeFilters{LotSize: 2}

	got := GetUsedQuantities(events.Events{Trade: trade})
	const want = 2.0
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("GetUsedQuantities with USDC fee = %v, want %v (USDC fee must not touch BTC wallet)", got, want)
	}
}

// TestGetUsedQuantitiesIgnoresBNBFeesOnBase pins the symmetric BNB case.
// BNB fees come from a separate wallet — they must not reduce base or quote
// usage estimates.
func TestGetUsedQuantitiesIgnoresBNBFeesOnBase(t *testing.T) {
	trade := aggragates.Trades{Symbol: "BTC/USDC"}
	trade.History = []aggragates.TradesHistory{
		{Type: "buy", Quantity: 2, Price: 100, Fees: []aggragates.TradesFees{{Asset: "BNB", Fee: 0.001}}},
	}
	trade.StrategyPair.TradeFilters = aggragates.TradeFilters{LotSize: 2}

	got := GetUsedQuantities(events.Events{Trade: trade})
	const want = 2.0
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("GetUsedQuantities with BNB fee = %v, want %v (BNB fee must not touch BTC wallet)", got, want)
	}
}

// TestGetUsedQuantitiesMixedKeepsOnlyBaseAssetFee covers the realistic Binance
// row where the same trade has a base-asset fee plus a BNB row. Only the
// base-asset portion deducts from the base wallet calculation.
func TestGetUsedQuantitiesMixedKeepsOnlyBaseAssetFee(t *testing.T) {
	trade := aggragates.Trades{Symbol: "BTC/USDC"}
	trade.History = []aggragates.TradesHistory{
		{Type: "buy", Quantity: 2, Price: 100, Fees: []aggragates.TradesFees{
			{Asset: "BTC", Fee: 0.01},
			{Asset: "BNB", Fee: 0.001}, // must be ignored
		}},
	}
	trade.StrategyPair.TradeFilters = aggragates.TradeFilters{LotSize: 2}

	got := GetUsedQuantities(events.Events{Trade: trade})
	const want = 1.99 // 2 - 0.01 (only the literal BTC fee deducts)
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("GetUsedQuantities mixed = %v, want %v (only BTC fee should deduct, BNB ignored)", got, want)
	}
}
