package profit

import (
	"math"
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// TestCalculateProfitInverseNoLongerDoubleChargesSellFees replays backtest 76
// trade 12840 (LINK/USDT inverse, closed 2025-10-10 21:18): three sell rungs,
// one buy-back sized from the fee-net proceeds, booked at -0.0024 LINK by the
// old math although the wallet ended the cycle ahead. With the sell-leg fees
// recognized as embodied, the same fills settle positive.
func TestCalculateProfitInverseNoLongerDoubleChargesSellFees(t *testing.T) {
	trade := aggragates.Trades{
		Symbol:        "LINK/USDT",
		Inverse:       true,
		Dust:          0.00029,
		PositionPrice: 15.69,
		History: []aggragates.TradesHistory{
			{Type: "sell", Quantity: 3.76, Price: 15.45, Fees: []aggragates.TradesFees{{Asset: "USDT", Fee: 0.0581}}},
			{Type: "sell", Quantity: 7.52, Price: 15.63, Fees: []aggragates.TradesFees{{Asset: "USDT", Fee: 0.1175}}},
			{Type: "sell", Quantity: 15.04, Price: 15.86, Fees: []aggragates.TradesFees{{Asset: "USDT", Fee: 0.2385}}},
			{Type: "buy", Quantity: 26.37, Price: 15.69, Fees: []aggragates.TradesFees{{Asset: "LINK", Fee: 0.02637}}},
		},
	}

	profit := CalculateProfit(events.Events{Trade: trade})

	// Gross 26.37-26.32 = +0.05 LINK, minus the buy-back fee 0.02637, plus
	// dust: ~+0.0239 LINK. The old cross-converted math landed at ~-0.0024.
	const want = 0.05 - 0.02637 + 0.00029
	if math.Abs(profit-want) > 1e-9 {
		t.Fatalf("profit = %v, want %v", profit, want)
	}
	if profit <= 0 {
		t.Fatalf("trade 12840's fills must settle positive, got %v", profit)
	}
}
