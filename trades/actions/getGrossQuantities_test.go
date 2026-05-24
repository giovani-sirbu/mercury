package actions

import (
	"math"
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// TestGetGrossQuantitiesSumsBuyAndSellSeparately confirms the function returns
// raw aggregated totals per side regardless of fill price — same contract as
// the legacy trades.GetQuantitiesOld it replaces.
func TestGetGrossQuantitiesSumsBuyAndSellSeparately(t *testing.T) {
	trade := aggragates.Trades{
		History: []aggragates.TradesHistory{
			{Type: "BUY", Quantity: 0.001, Price: 100000},
			{Type: "BUY", Quantity: 0.002, Price: 98000},
			{Type: "SELL", Quantity: 0.0005, Price: 102000},
		},
	}

	buyTotal, sellTotal := GetGrossQuantities(events.Events{Trade: trade})

	const wantBuy = 0.003
	const wantSell = 0.0005
	if math.Abs(buyTotal-wantBuy) > 1e-12 {
		t.Errorf("buyTotal = %v, want %v", buyTotal, wantBuy)
	}
	if math.Abs(sellTotal-wantSell) > 1e-12 {
		t.Errorf("sellTotal = %v, want %v", sellTotal, wantSell)
	}
}

// TestGetGrossQuantitiesIsCaseInsensitiveOnType pins the dual-case acceptance
// guarantee — Binance order responses use uppercase "BUY"/"SELL" while some
// internal adjustments emit lowercase. The function must aggregate both into
// the same bucket.
func TestGetGrossQuantitiesIsCaseInsensitiveOnType(t *testing.T) {
	trade := aggragates.Trades{
		History: []aggragates.TradesHistory{
			{Type: "BUY", Quantity: 0.001, Price: 100000},
			{Type: "buy", Quantity: 0.002, Price: 98000},
			{Type: "SELL", Quantity: 0.0005, Price: 102000},
			{Type: "sell", Quantity: 0.001, Price: 101000},
		},
	}

	buyTotal, sellTotal := GetGrossQuantities(events.Events{Trade: trade})

	const wantBuy = 0.003
	const wantSell = 0.0015
	if math.Abs(buyTotal-wantBuy) > 1e-12 {
		t.Errorf("buyTotal = %v, want %v", buyTotal, wantBuy)
	}
	if math.Abs(sellTotal-wantSell) > 1e-12 {
		t.Errorf("sellTotal = %v, want %v", sellTotal, wantSell)
	}
}

// TestGetGrossQuantitiesReturnsZerosForEmptyHistory covers the no-history edge
// case (e.g. a freshly created trade before its first fill).
func TestGetGrossQuantitiesReturnsZerosForEmptyHistory(t *testing.T) {
	buyTotal, sellTotal := GetGrossQuantities(events.Events{Trade: aggragates.Trades{}})

	if buyTotal != 0 || sellTotal != 0 {
		t.Errorf("empty history = (%v, %v), want (0, 0)", buyTotal, sellTotal)
	}
}

// TestGetGrossQuantitiesIgnoresPriceForInverse documents the deliberate
// difference from the new GetQuantities: this primitive does NOT multiply by
// per-record price even when Inverse is set. Callers in sell.go/hasFunds.go
// that need quote-denominated totals call trades.GetQuantityInQuote
// separately. buy.go relies on this raw-only contract.
func TestGetGrossQuantitiesIgnoresPriceForInverse(t *testing.T) {
	trade := aggragates.Trades{
		Inverse: true,
		History: []aggragates.TradesHistory{
			{Type: "BUY", Quantity: 100, Price: 50000},
			{Type: "SELL", Quantity: 50, Price: 51000},
		},
	}

	buyTotal, sellTotal := GetGrossQuantities(events.Events{Trade: trade})

	if math.Abs(buyTotal-100) > 1e-12 {
		t.Errorf("inverse buyTotal = %v, want 100 (no price multiplication)", buyTotal)
	}
	if math.Abs(sellTotal-50) > 1e-12 {
		t.Errorf("inverse sellTotal = %v, want 50 (no price multiplication)", sellTotal)
	}
}
