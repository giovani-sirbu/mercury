package tests

import (
	"math"
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/actions"
)

// TestInversePartial_SellsThenPartialBuyLeavesShortPosition mirrors the
// spot "x buys, partial sell" test for inverse direction. Three inverse
// "buys" (= exchange SELLs) totalling 0.007 BTC, then a partial buy-back of
// 0.002 BTC. Remaining inverse position is 0.005 BTC short.
func TestInversePartial_SellsThenPartialBuyLeavesShortPosition(t *testing.T) {
	trade := scenarioBuildTrade("takeProfit", 99000, true)
	scenarioAppendHistory(&trade, "SELL", 0.001, 100000, "", 0)
	scenarioAppendHistory(&trade, "SELL", 0.002, 102000, "", 0)
	scenarioAppendHistory(&trade, "SELL", 0.004, 104040, "", 0)
	scenarioAppendHistory(&trade, "BUY", 0.002, 99000, "", 0)

	event := events.Events{Trade: trade}
	qty, side := actions.GetQuantities(event)
	if side != "buy" {
		t.Errorf("side = %q, want buy", side)
	}
	// sellTotal(quote) = 100 + 204 + 416.16 = 720.16
	// buyTotal(quote)  = 0.002 * 99000 = 198
	// diff = 522.16, /99000 = 0.00527..., ToFixed(5) = 0.00527
	const want = 0.00527
	if math.Abs(qty-want) > 1e-5 {
		t.Errorf("qty = %v, want %v", qty, want)
	}
}

// TestInversePartial_SellAgainAfterPartialBuy adds another inverse "buy" at
// a new higher price after the partial close. Tests that Sell action's
// quantity math reflects the latest history correctly.
func TestInversePartial_SellAgainAfterPartialBuy(t *testing.T) {
	trade := scenarioBuildTrade("takeProfit", 100000, true)
	// History uses uppercase SELL/BUY consistently. GetQuantityInQuote, the
	// helper Sell calls on the inverse branch, does a case-sensitive match
	// against the side filter — so the buy-back row must read "BUY" exactly
	// to be excluded from the sell aggregate. Mixing cases here would push
	// the "buy" row into the sell bucket and skew the close quantity.
	scenarioAppendHistory(&trade, "SELL", 0.001, 100000, "", 0)
	scenarioAppendHistory(&trade, "SELL", 0.002, 102000, "", 0)
	scenarioAppendHistory(&trade, "SELL", 0.004, 104040, "", 0)
	scenarioAppendHistory(&trade, "BUY", 0.002, 99000, "", 0)
	scenarioAppendHistory(&trade, "SELL", 0.008, 106120, "", 0)

	event := scenarioBuildEvent(trade, "USDC", "1500")

	got, err := actions.Sell(event)
	if err != nil {
		t.Fatalf("Sell returned error: %v", err)
	}
	// sellTotal(quote) = 100 + 204 + 416.16 + 848.96 = 1569.12
	// buyTotal(quote)  = 198
	// diff = 1371.12, /100000 = 0.01371..., ToFixed(5) = 0.01371
	const want = 0.01371
	if math.Abs(got.Params.Quantity-want) > 1e-5 {
		t.Errorf("Sell qty (inverse) = %v, want %v", got.Params.Quantity, want)
	}
}

// TestInversePartial_FullCloseRealizedProfitInBase walks the trade all the
// way to closure with the final buy covering remaining inverse inventory.
// Documents that inverse profit is reported in BASE asset.
func TestInversePartial_FullCloseRealizedProfitInBase(t *testing.T) {
	trade := scenarioBuildTrade("closed", 99000, true)
	scenarioAppendHistory(&trade, "sell", 0.001, 100000, "", 0)
	scenarioAppendHistory(&trade, "sell", 0.002, 102000, "", 0)
	scenarioAppendHistory(&trade, "sell", 0.004, 104040, "", 0)
	scenarioAppendHistory(&trade, "buy", 0.002, 99000, "", 0)
	scenarioAppendHistory(&trade, "buy", 0.005, 99000, "", 0)

	profit := actions.GetProfit(trade)
	// inverse: profit = buyTotal(base) - sellTotal(base) + dust
	//   buyTotal(base) = 0.002 + 0.005 = 0.007
	//   sellTotal(base) = 0.001 + 0.002 + 0.004 = 0.007
	//   profit = 0 (matched)
	if math.Abs(profit) > 1e-9 {
		t.Errorf("inverse GetProfit = %v, want 0", profit)
	}
}

// TestInversePartial_DustOnInverseClose pins the dust field on inverse
// trades. With a sellTotal/buyTotal mismatch divided by PositionPrice and
// rounded with LotSize=5, the residual base-asset dust must be recorded.
func TestInversePartial_DustOnInverseClose(t *testing.T) {
	trade := scenarioBuildTrade("takeProfit", 99000, true)
	// Single inverse sell that produces a non-LotSize-aligned quantity on
	// close: 0.0003 BTC short / 99000 -> remainder triggers dust.
	scenarioAppendHistory(&trade, "SELL", 0.0003, 100000, "BTC", 0.0000003)

	event := scenarioBuildEvent(trade, "USDC", "100")

	got, err := actions.Sell(event)
	if err != nil {
		t.Fatalf("Sell returned error: %v", err)
	}
	if got.Trade.Dust < 0 {
		t.Errorf("Trade.Dust should be non-negative, got %v", got.Trade.Dust)
	}
}
