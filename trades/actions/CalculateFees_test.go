package actions

import (
	"math"
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func TestCalculateFeesOldSumsBaseAndQuoteFees(t *testing.T) {
	trade := aggragates.Trades{Symbol: "BTC/USDT"}
	trade.History = []aggragates.TradesHistory{
		{Fees: []aggragates.TradesFees{{Asset: "BTC", Fee: 0.001}, {Asset: "USDT", Fee: 5}}},
		{Fees: []aggragates.TradesFees{{Asset: "BTC", Fee: 0.002}, {Asset: "USDT", Fee: 10}}},
	}

	feeInBase, feeInQuote := CalculateFeesOld(events.Events{Trade: trade})

	if math.Abs(feeInBase-0.003) > 1e-9 {
		t.Errorf("feeInBase = %v, want 0.003", feeInBase)
	}
	if math.Abs(feeInQuote-15) > 1e-9 {
		t.Errorf("feeInQuote = %v, want 15", feeInQuote)
	}
}

func TestCalculateFeesOldIgnoresUnknownAssetFees(t *testing.T) {
	trade := aggragates.Trades{Symbol: "BTC/USDT"}
	trade.History = []aggragates.TradesHistory{
		{Fees: []aggragates.TradesFees{{Asset: "BNB", Fee: 0.5}}},
	}

	feeInBase, feeInQuote := CalculateFeesOld(events.Events{Trade: trade})
	if feeInBase != 0 || feeInQuote != 0 {
		t.Errorf("expected zero base and quote fees, got base=%v quote=%v", feeInBase, feeInQuote)
	}
}

func TestCalculateFeesOldReturnsZeroForEmptyHistory(t *testing.T) {
	trade := aggragates.Trades{Symbol: "BTC/USDT"}
	feeInBase, feeInQuote := CalculateFeesOld(events.Events{Trade: trade})
	if feeInBase != 0 || feeInQuote != 0 {
		t.Errorf("expected zero fees for empty history, got base=%v quote=%v", feeInBase, feeInQuote)
	}
}
