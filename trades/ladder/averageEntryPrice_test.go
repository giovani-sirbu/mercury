package ladder

import (
	"math"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// Backtest 119 / trade 39385: eight ETH buys, 36898.508251 USDT for 19.1505 ETH.
func TestAverageEntryPriceIsTheCostPerUnitAcrossEveryEntry(t *testing.T) {
	trade := aggragates.Trades{History: []aggragates.TradesHistory{
		{Type: "BUY", Quantity: 0.0751, Price: 2883.13, OrderId: 1},
		{Type: "BUY", Quantity: 0.1502, Price: 2738.60, OrderId: 2},
		{Type: "BUY", Quantity: 0.3004, Price: 2671.99, OrderId: 3},
		{Type: "BUY", Quantity: 0.6008, Price: 2606.99, OrderId: 4},
		{Type: "BUY", Quantity: 1.2016, Price: 2543.68, OrderId: 5},
		{Type: "BUY", Quantity: 2.4032, Price: 2451.16, OrderId: 6},
		{Type: "BUY", Quantity: 4.8064, Price: 2391.53, OrderId: 7},
		{Type: "BUY", Quantity: 9.6128, Price: 1400.21, OrderId: 8},
	}}

	if got := AverageEntryPrice(trade); math.Abs(got-1926.7647) > 1e-4 {
		t.Fatalf("average entry price = %f, want 1926.7647", got)
	}
}

func TestAverageEntryPriceAddsPartialFillsAndIgnoresExits(t *testing.T) {
	trade := aggragates.Trades{History: []aggragates.TradesHistory{
		{Type: "BUY", Quantity: 1, Price: 100, OrderId: 10},
		{Type: "BUY", Quantity: 0.5, Price: 99, OrderId: 11},
		{Type: "BUY", Quantity: 0.5, Price: 99, OrderId: 11},
		{Type: "SELL", Quantity: 1, Price: 130, OrderId: 12},
	}}

	if got := AverageEntryPrice(trade); math.Abs(got-99.5) > 1e-9 {
		t.Fatalf("average entry price = %f, want 99.5", got)
	}
}

func TestAverageEntryPriceUsesSellsForInverseTrades(t *testing.T) {
	trade := aggragates.Trades{Inverse: true, History: []aggragates.TradesHistory{
		{Type: "SELL", Quantity: 1, Price: 100, OrderId: 20},
		{Type: "SELL", Quantity: 3, Price: 104, OrderId: 21},
		{Type: "BUY", Quantity: 1, Price: 90, OrderId: 22},
	}}

	if got := AverageEntryPrice(trade); math.Abs(got-103) > 1e-9 {
		t.Fatalf("inverse average entry price = %f, want 103", got)
	}
}

func TestAverageEntryPriceSkipsAccountingRowsAndEmptyLadders(t *testing.T) {
	ledgerOnly := aggragates.Trades{History: []aggragates.TradesHistory{
		{Type: "BUY", Quantity: 12.5, Price: 0.0000000000001, OrderId: 30},
	}}
	if got := AverageEntryPrice(ledgerOnly); got != 0 {
		t.Fatalf("a child-profit ledger row is not an entry, got %f", got)
	}

	if got := AverageEntryPrice(aggragates.Trades{}); got != 0 {
		t.Fatalf("no fills must give no average, got %f", got)
	}
}
