package smarttakeloss

import (
	"math"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

func TestEstimateCloseProfitInverseMath(t *testing.T) {
	trade := testutil.DeepLadderTrade(3, true) // SELL 1@100, 1@102, 1@104: 306 quote in
	profit, invested := estimateCloseProfit(trade, 90)
	if invested != 3 {
		t.Fatalf("inverse invested = %f base, want 3", invested)
	}
	if math.Abs(profit-(306.0/90-3)) > 1e-9 {
		t.Fatalf("inverse profit at 90 = %f base, want %f", profit, 306.0/90-3)
	}

	// Long-branch regression pin: five 1-unit buys at 100..92 (480 quote).
	long := testutil.DeepLadderTrade(5, false)
	profit, invested = estimateCloseProfit(long, 97)
	if invested != 480 || math.Abs(profit-5) > 1e-9 {
		t.Fatalf("long branch = (%f, %f), want (5, 480)", profit, invested)
	}
}

func TestRequiredRecoveryPctClosedForms(t *testing.T) {
	inverse := testutil.DeepLadderTrade(3, true) // break even 306/3 = 102, price must fall
	got := requiredRecoveryPct(inverse, 110)
	if math.Abs(got-(110.0-102)/110*100) > 1e-9 {
		t.Fatalf("inverse required recovery = %f, want %f", got, (110.0-102)/110*100)
	}
	if requiredRecoveryPct(inverse, 95) != 0 {
		t.Fatal("a price past break even must need zero recovery")
	}
	long := testutil.DeepLadderTrade(5, false) // break even 480/5 = 96
	if requiredRecoveryPct(long, 97) != 0 {
		t.Fatal("a profitable long must need zero recovery")
	}
}

// The STL close estimate is fee-aware: a chain exactly at gross break-even
// settles negative once the embodied and closing commissions are charged.
func TestEstimateCloseProfitChargesFees(t *testing.T) {
	trade := aggragates.Trades{Symbol: "SOL/USDT", PositionPrice: 100}
	trade.History = []aggragates.TradesHistory{{
		Type: "BUY", Quantity: 1, Price: 100, OrderId: 1,
		Fees: []aggragates.TradesFees{{Asset: "SOL", Fee: 0.001}},
	}}
	profit, invested := estimateCloseProfit(trade, 100)
	if invested != 100 {
		t.Fatalf("invested = %v, want 100", invested)
	}
	// 0.999 * 100 - 100 = -0.1, minus the closing leg 0.999*100*0.001 = -0.0999
	if math.Abs(profit-(-0.1999)) > 1e-9 {
		t.Errorf("fee-aware close profit = %v, want -0.1999", profit)
	}
	bare := trade
	bare.History = []aggragates.TradesHistory{{Type: "BUY", Quantity: 1, Price: 100, OrderId: 1}}
	if profit, _ := estimateCloseProfit(bare, 100); profit != 0 {
		t.Errorf("without fee rows the estimate must stay fee-free, got %v", profit)
	}
}
