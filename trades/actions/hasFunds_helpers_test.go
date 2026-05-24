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
