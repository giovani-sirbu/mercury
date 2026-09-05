package funds_test

import (
	"testing"

	"github.com/giovani-sirbu/mercury/exchange/aggregates"
	"github.com/giovani-sirbu/mercury/trades/funds"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

func TestGetAssetBudget(t *testing.T) {
	tests := []struct {
		name   string
		assets []aggregates.UserAssetRecord
		symbol string
		want   float64
	}{
		{
			"AssetFound",
			[]aggregates.UserAssetRecord{{Asset: "USDT", Free: "100.5"}},
			"USDT",
			100.5,
		},
		{
			"AssetNotFound",
			[]aggregates.UserAssetRecord{{Asset: "BTC", Free: "1.0"}},
			"USDT",
			0.0,
		},
		{
			"EmptyAssets",
			[]aggregates.UserAssetRecord{},
			"USDT",
			0.0,
		},
		{
			"InvalidFreeValue",
			[]aggregates.UserAssetRecord{{Asset: "USDT", Free: "abc"}},
			"USDT",
			0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := funds.GetAssetBudget(tt.assets, tt.symbol)
			testutil.AssertFloatEqual(t, got, tt.want, 1e-10, "GetAssetBudget")
		})
	}
}
