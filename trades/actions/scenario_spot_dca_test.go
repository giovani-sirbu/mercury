package actions_test

import (
	"github.com/giovani-sirbu/mercury/trades/fees"
	"github.com/giovani-sirbu/mercury/trades/profit"
	"github.com/giovani-sirbu/mercury/trades/quantities"
	"math"
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/actions"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// TestSpotDCA_FourBuysPriceDownThenSellAtProfit walks the full DCA happy path
// on BTC/USDC: four bullish-bias buys at falling prices, then a final sell
// once price recovers above the weighted-average entry. Every action chain
// stop is exercised: history accumulation, GetGrossQuantities, GetProfit,
// GetFeesBaseQuote, GetFees, Sell.
//
// History (BUY):
//
//	0.001 BTC @ 100000 USDC  (fee 0.000001 BTC)
//	0.002 BTC @  98000 USDC  (fee 0.000002 BTC)
//	0.004 BTC @  96040 USDC  (fee 0.000004 BTC)
//	0.008 BTC @  94120 USDC  (fee 0.000008 BTC)
//
// Total bought  = 0.015 BTC, total spent = 1448.84 USDC.
// Total fees in base = 0.000015 BTC.
// Sell @ 101000 USDC closes the position.
func TestSpotDCA_FourBuysPriceDownThenSellAtProfit(t *testing.T) {
	trade := scenarioBuildTrade("takeProfit", 101000, false)
	scenarioAppendHistory(&trade, "BUY", 0.001, 100000, "BTC", 0.000001)
	scenarioAppendHistory(&trade, "BUY", 0.002, 98000, "BTC", 0.000002)
	scenarioAppendHistory(&trade, "BUY", 0.004, 96040, "BTC", 0.000004)
	scenarioAppendHistory(&trade, "BUY", 0.008, 94120, "BTC", 0.000008)

	event := scenarioBuildEvent(trade, "BTC", "0.015")

	// --- Step 1: profit calculation must report a positive net profit at
	// sell price 101000. GetProfit = sellTotal - buyTotal in quote.
	feeInBase, _ := fees.GetFeesBaseQuote(event)
	if math.Abs(feeInBase-0.000015) > 1e-9 {
		t.Fatalf("GetFeesBaseQuote base = %v, want 0.000015", feeInBase)
	}

	// --- Step 2: Sell must consume the entire net long position and emit a
	// single SELL order at PositionPrice.
	got, err := actions.Sell(event)
	if err != nil {
		t.Fatalf("Sell returned error: %v", err)
	}
	// Net quantity = buy - sell - feeInBase = 0.015 - 0 - 0.000015 = 0.014985
	// ToFixed(0.014985, 5) = 0.01498 (RoundFloor truncates the 5th decimal)
	const wantQty = 0.01498
	if math.Abs(got.Params.Quantity-wantQty) > 1e-9 {
		t.Errorf("Sell quantity = %v, want %v", got.Params.Quantity, wantQty)
	}
	if got.Trade.PendingOrder == 0 {
		t.Errorf("expected sell to create a pending order id")
	}
}

// TestSpotDCA_HasProfitAcceptsAfterRecovery proves the HasProfit gate opens
// once enough averaging-down rows have improved cost basis. With three buys
// at 100k / 98k / 96.04k and a position price of 99k, the simulated sell on
// the remaining 0.007 BTC nets ~50 USDC after the tolerance haircut, well
// above MinNotional * Percentage/100 = 0.1 USDC.
func TestSpotDCA_HasProfitAcceptsAfterRecovery(t *testing.T) {
	trade := scenarioBuildTrade("takeProfit", 99000, false)
	scenarioAppendHistory(&trade, "BUY", 0.001, 100000, "", 0)
	scenarioAppendHistory(&trade, "BUY", 0.002, 98000, "", 0)
	scenarioAppendHistory(&trade, "BUY", 0.004, 96040, "", 0)

	event := scenarioBuildEvent(trade, "USDC", "1000")

	got, err := actions.HasProfit(event)
	if err != nil {
		t.Fatalf("HasProfit rejected a profitable scenario: %v", err)
	}
	if got.Trade.Profit <= 0 {
		t.Errorf("expected positive profit, got %v", got.Trade.Profit)
	}
}

// TestSpotDCA_HasProfitRejectsWhenPriceStillUnderwater inverts the prior
// scenario: PositionPrice 94000 is below the cost basis, so HasProfit must
// short-circuit with an error.
func TestSpotDCA_HasProfitRejectsWhenPriceStillUnderwater(t *testing.T) {
	trade := scenarioBuildTrade("takeProfit", 94000, false)
	scenarioAppendHistory(&trade, "BUY", 0.001, 100000, "", 0)
	scenarioAppendHistory(&trade, "BUY", 0.002, 98000, "", 0)
	scenarioAppendHistory(&trade, "BUY", 0.004, 96040, "", 0)

	event := scenarioBuildEvent(trade, "USDC", "1000")

	_, err := actions.HasProfit(event)
	if err == nil {
		t.Fatal("expected HasProfit to reject when price is below cost basis")
	}
}

// TestSpotDCA_RegulatePriceChangeBlocksTooCloseBuy ensures that after a buy
// at 100000, a new buy attempt at 99500 (only 0.5% below) is rejected — the
// strategy requires at least Percentage% (=2%) drop from the last buy price.
func TestSpotDCA_RegulatePriceChangeBlocksTooCloseBuy(t *testing.T) {
	trade := scenarioBuildTrade("buy", 99500, false)
	scenarioAppendHistory(&trade, "BUY", 0.001, 100000, "", 0)

	event := events.Events{Trade: trade}

	_, err := actions.RegulatePriceChange(event)
	if err == nil {
		t.Fatal("expected RegulatePriceChange to reject a buy too close to the last entry")
	}
}

// TestSpotDCA_RegulatePriceChangeAllowsFarEnoughBuy mirrors the prior test:
// at 97800 (>=2% below 100000) the new buy is allowed.
func TestSpotDCA_RegulatePriceChangeAllowsFarEnoughBuy(t *testing.T) {
	trade := scenarioBuildTrade("buy", 97800, false)
	scenarioAppendHistory(&trade, "BUY", 0.001, 100000, "", 0)

	event := events.Events{Trade: trade}

	got, err := actions.RegulatePriceChange(event)
	if err != nil {
		t.Fatalf("RegulatePriceChange rejected a valid buy: %v", err)
	}
	if got.Trade.PositionPrice != 97800 {
		t.Errorf("expected event returned unchanged, price = %v", got.Trade.PositionPrice)
	}
}

// TestSpotDCA_CalculateProfitMatchesGetProfitMinusFees pins the CalculateProfit
// public API: gross profit minus quote-asset fees. Uses a closed two-row
// history (one buy + one sell at higher price) to assert the exact value.
func TestSpotDCA_CalculateProfitMatchesGetProfitMinusFees(t *testing.T) {
	trade := scenarioBuildTrade("closed", 102000, false)
	// buy 0.01 @ 100000 (fee 0.5 USDC), sell 0.01 @ 102000 (fee 0.51 USDC)
	scenarioAppendHistory(&trade, "buy", 0.01, 100000, "USDC", 0.5)
	scenarioAppendHistory(&trade, "sell", 0.01, 102000, "USDC", 0.51)

	event := events.Events{Trade: trade}

	got := profit.CalculateProfit(event)
	// gross = 1020 - 1000 = 20, fees in quote = 0.5 + 0.51 = 1.01, net = 18.99
	const want = 18.99
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("CalculateProfit = %v, want %v", got, want)
	}
}

// TestSpotDCA_GetQuantitiesTracksRunningBalance documents how the engine
// reports outstanding inventory after a partial sell. With 0.015 BTC bought
// and 0.004 sold, the SELL action's expected quantity is 0.011 BTC.
func TestSpotDCA_GetQuantitiesTracksRunningBalance(t *testing.T) {
	trade := scenarioBuildTrade("takeProfit", 101000, false)
	scenarioAppendHistory(&trade, "BUY", 0.001, 100000, "", 0)
	scenarioAppendHistory(&trade, "BUY", 0.002, 98000, "", 0)
	scenarioAppendHistory(&trade, "BUY", 0.004, 96040, "", 0)
	scenarioAppendHistory(&trade, "BUY", 0.008, 94120, "", 0)
	scenarioAppendHistory(&trade, "sell", 0.004, 101000, "", 0)

	event := events.Events{Trade: trade}

	qty, historyType := quantities.GetQuantities(event)
	if historyType != "sell" {
		t.Errorf("historyType = %q, want sell", historyType)
	}
	if math.Abs(qty-0.011) > 1e-9 {
		t.Errorf("qty = %v, want 0.011", qty)
	}
}

// TestSpotDCA_GetUsedQuantitiesNetOfFees mirrors GetQuantities but subtracts
// the cumulative base-asset fee. Important for the HasFunds path when the
// exchange charges fees in the base currency (BTC).
func TestSpotDCA_GetUsedQuantitiesNetOfFees(t *testing.T) {
	trade := scenarioBuildTrade("buy", 95000, false)
	scenarioAppendHistory(&trade, "BUY", 0.001, 100000, "BTC", 0.000001)
	scenarioAppendHistory(&trade, "BUY", 0.002, 98000, "BTC", 0.000002)

	event := events.Events{Trade: trade}

	got := quantities.GetUsedQuantities(event)
	// buy = 0.003, sell = 0, feeInBase = 0.000003 -> 0.002997, ToFixed(5) = 0.00299
	const want = 0.00299
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("GetUsedQuantities = %v, want %v", got, want)
	}
}

// helper used by sub-tests above (no public dependency on aggragates here):
var _ = aggragates.Trades{}
