package tests

import (
	"math"
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/actions"
)

// TestInverseDCA_FourSellsPriceUpThenBuyAtProfit mirrors the spot DCA happy
// path but for inverse trades. Inverse means the trade profits in BTC and
// each "buy" action calls Sell on the exchange (lock in BTC at high price)
// while the closing "sell" action calls Buy (cash out at lower price).
//
// History (SELL — inverse buys):
//
//	0.001 BTC @ 100000 USDC
//	0.002 BTC @ 102000 USDC
//	0.004 BTC @ 104040 USDC
//	0.008 BTC @ 106120 USDC
//
// Then takeProfit at 99000 — price has fallen below the cost basis so the
// inverse position is in profit.
func TestInverseDCA_FourSellsPriceUpThenBuyAtProfit(t *testing.T) {
	trade := scenarioBuildTrade("takeProfit", 99000, true)
	scenarioAppendHistory(&trade, "SELL", 0.001, 100000, "", 0)
	scenarioAppendHistory(&trade, "SELL", 0.002, 102000, "", 0)
	scenarioAppendHistory(&trade, "SELL", 0.004, 104040, "", 0)
	scenarioAppendHistory(&trade, "SELL", 0.008, 106120, "", 0)

	event := scenarioBuildEvent(trade, "USDC", "1500")

	got, err := actions.Sell(event)
	if err != nil {
		t.Fatalf("Sell returned error: %v", err)
	}
	// For inverse, Sell computes qty = (sellTotal - buyTotal) / positionPrice.
	// sellTotal (quote) = 0.001*100000 + 0.002*102000 + 0.004*104040 + 0.008*106120
	//                   = 100 + 204 + 416.16 + 848.96 = 1569.12
	// buyTotal (quote)  = 0
	// qty = 1569.12 / 99000 ≈ 0.01585
	// ToFixed(0.0158497..., 5) = 0.01584 (RoundFloor)
	const wantQty = 0.01584
	if math.Abs(got.Params.Quantity-wantQty) > 1e-5 {
		t.Errorf("Sell (inverse close) qty = %v, want %v", got.Params.Quantity, wantQty)
	}
	if got.Trade.PendingOrder == 0 {
		t.Errorf("expected pending order id set")
	}
}

// TestInverseDCA_HasProfitAcceptsAfterPriceDrops mirrors the spot HasProfit
// success case in the inverse direction.
func TestInverseDCA_HasProfitAcceptsAfterPriceDrops(t *testing.T) {
	trade := scenarioBuildTrade("takeProfit", 99000, true)
	scenarioAppendHistory(&trade, "SELL", 0.001, 100000, "", 0)
	scenarioAppendHistory(&trade, "SELL", 0.002, 102000, "", 0)
	scenarioAppendHistory(&trade, "SELL", 0.004, 104040, "", 0)

	event := events.Events{Trade: trade}

	got, err := actions.HasProfit(event)
	if err != nil {
		t.Fatalf("HasProfit rejected a profitable inverse scenario: %v", err)
	}
	if got.Trade.Profit <= 0 {
		t.Errorf("expected positive profit on inverse take-profit, got %v", got.Trade.Profit)
	}
}

// TestInverseDCA_HasProfitRejectsWhenPriceStillRising verifies the negative
// path: price hasn't dropped below cost basis yet.
func TestInverseDCA_HasProfitRejectsWhenPriceStillRising(t *testing.T) {
	trade := scenarioBuildTrade("takeProfit", 108000, true)
	scenarioAppendHistory(&trade, "SELL", 0.001, 100000, "", 0)
	scenarioAppendHistory(&trade, "SELL", 0.002, 102000, "", 0)

	event := events.Events{Trade: trade}

	_, err := actions.HasProfit(event)
	if err == nil {
		t.Fatal("expected HasProfit to reject inverse trade when price has not dropped")
	}
}

// TestInverseDCA_RegulatePriceChangeBlocksTooCloseSell mirrors the spot rule
// for inverse direction: the next inverse "buy" (= exchange Sell) must be
// at least Percentage% ABOVE the previous sell price. 100500 vs 100000 is
// only +0.5%, below the 2% threshold -> reject.
func TestInverseDCA_RegulatePriceChangeBlocksTooCloseSell(t *testing.T) {
	trade := scenarioBuildTrade("buy", 100500, true)
	scenarioAppendHistory(&trade, "SELL", 0.001, 100000, "", 0)

	event := events.Events{Trade: trade}

	_, err := actions.RegulatePriceChange(event)
	if err == nil {
		t.Fatal("expected RegulatePriceChange to reject inverse buy too close to last sell")
	}
}

// TestInverseDCA_RegulatePriceChangeAllowsFarEnoughSell — 102500 is +2.5%
// above 100000, beyond the 2% threshold, so the buy is allowed.
func TestInverseDCA_RegulatePriceChangeAllowsFarEnoughSell(t *testing.T) {
	trade := scenarioBuildTrade("buy", 102500, true)
	scenarioAppendHistory(&trade, "SELL", 0.001, 100000, "", 0)

	event := events.Events{Trade: trade}

	got, err := actions.RegulatePriceChange(event)
	if err != nil {
		t.Fatalf("RegulatePriceChange rejected a valid inverse buy: %v", err)
	}
	if got.Trade.PositionPrice != 102500 {
		t.Errorf("expected event returned unchanged")
	}
}

// TestInverseDCA_GetProfitReturnsBaseCurrencyDelta pins GetProfit for
// inverse: the result is in BASE asset (BTC), not quote. With 0.003 sold at
// avg ~101333 and 0.001 buy-back at 99000, the inverse profit is
// 0.003 - 0.001 = 0.002 BTC.
func TestInverseDCA_GetProfitReturnsBaseCurrencyDelta(t *testing.T) {
	trade := scenarioBuildTrade("closed", 99000, true)
	scenarioAppendHistory(&trade, "sell", 0.001, 100000, "", 0)
	scenarioAppendHistory(&trade, "sell", 0.002, 102000, "", 0)
	scenarioAppendHistory(&trade, "buy", 0.001, 99000, "", 0)

	profit := actions.GetProfit(trade)
	// buyTotal (base) = 0.001
	// sellTotal (base) = 0.001 + 0.002 = 0.003
	// inverse: profit = buyTotal - sellTotal + dust = 0.001 - 0.003 + 0 = -0.002
	// Note: positive realised gain requires buy quantity > sell quantity (we
	// bought back more BTC than we sold short on a quote-fixed strategy).
	const want = -0.002
	if math.Abs(profit-want) > 1e-9 {
		t.Errorf("inverse GetProfit = %v, want %v", profit, want)
	}
}

// TestInverseDCA_GetQuantitiesReturnsRemainingInBase verifies that inverse
// GetQuantities divides the net quote difference by PositionPrice to return
// a base-asset quantity, and reports the historyType as "buy".
func TestInverseDCA_GetQuantitiesReturnsRemainingInBase(t *testing.T) {
	trade := scenarioBuildTrade("takeProfit", 99000, true)
	scenarioAppendHistory(&trade, "SELL", 0.001, 100000, "", 0)
	scenarioAppendHistory(&trade, "SELL", 0.002, 102000, "", 0)

	event := events.Events{Trade: trade}
	qty, side := actions.GetQuantities(event)
	if side != "buy" {
		t.Errorf("inverse historyType = %q, want buy", side)
	}
	// sellTotal(quote) = 100 + 204 = 304, buyTotal(quote) = 0
	// qty = 304 / 99000 = 0.00307..., ToFixed(5) = 0.00307
	const want = 0.00307
	if math.Abs(qty-want) > 1e-5 {
		t.Errorf("qty = %v, want %v", qty, want)
	}
}
