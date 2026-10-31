package funds

import (
	"testing"

	"github.com/giovani-sirbu/mercury/exchange/aggregates"
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
