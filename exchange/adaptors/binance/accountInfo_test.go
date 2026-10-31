package binanceAdaptor

import (
	"testing"

	"github.com/giovani-sirbu/mercury/exchange/aggregates"
)

// TestHeldBalancesDropsEmptyRows guards the shape the GetUserAssets fallback
// hands back. /api/v3/account lists every asset the exchange knows, nearly all
// at zero, while the /sapi endpoint it substitutes for reports only what is
// held — callers that take the first N rows would otherwise show empty assets.
func TestHeldBalancesDropsEmptyRows(t *testing.T) {
	balances := []aggregates.UserAssetRecord{
		{Asset: "AAVE", Free: "0.00000000", Locked: "0.00000000"},
		{Asset: "BTC", Free: "0.50000000", Locked: "0.00000000"},
		{Asset: "ETH", Free: "0.00000000", Locked: "1.25000000"},
		{Asset: "LTC", Free: "0.00000000", Locked: "0.00000000", Freeze: "3.00000000"},
		{Asset: "XRP", Free: "", Locked: ""},
	}

	held := heldBalances(balances)

	want := []string{"BTC", "ETH", "LTC"}
	if len(held) != len(want) {
		t.Fatalf("got %d rows (%v), want %d", len(held), held, len(want))
	}
	for i, asset := range want {
		if held[i].Asset != asset {
			t.Errorf("row %d: got %q, want %q", i, held[i].Asset, asset)
		}
	}
}
