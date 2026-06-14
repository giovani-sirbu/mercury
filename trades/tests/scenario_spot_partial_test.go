package tests

import (
	"math"
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/actions"
)

// TestSpotPartial_BuysThenPartialSellLeavesInventory exercises the
// "x buys, partial sell" half of the user's scenario. Three buys totalling
// 0.007 BTC, then a partial sell of 0.003 BTC, leaving 0.004 BTC in the
// position. GetQuantities must report exactly 0.004 for the next sell.
func TestSpotPartial_BuysThenPartialSellLeavesInventory(t *testing.T) {
	trade := scenarioBuildTrade("takeProfit", 101000, false)
	scenarioAppendHistory(&trade, "BUY", 0.001, 100000, "", 0)
	scenarioAppendHistory(&trade, "BUY", 0.002, 98000, "", 0)
	scenarioAppendHistory(&trade, "BUY", 0.004, 96040, "", 0)
	scenarioAppendHistory(&trade, "sell", 0.003, 101000, "", 0)

	event := events.Events{Trade: trade}
	qty, side := actions.GetQuantities(event)
	if side != "sell" {
		t.Errorf("side = %q, want sell", side)
	}
	if math.Abs(qty-0.004) > 1e-9 {
		t.Errorf("remaining qty = %v, want 0.004", qty)
	}
}

// TestSpotPartial_BuyAgainAfterPartialSell extends the prior history with a
// fourth buy (price dropped again). Sell quantity must now reflect
// remaining inventory minus any base-asset fee. With qty 0.004 remaining
// before the new buy and a new buy of 0.008, total bought is 0.015, total
// sold 0.003, net 0.012 BTC. Sell honours that.
func TestSpotPartial_BuyAgainAfterPartialSell(t *testing.T) {
	trade := scenarioBuildTrade("takeProfit", 102000, false)
	scenarioAppendHistory(&trade, "BUY", 0.001, 100000, "", 0)
	scenarioAppendHistory(&trade, "BUY", 0.002, 98000, "", 0)
	scenarioAppendHistory(&trade, "BUY", 0.004, 96040, "", 0)
	scenarioAppendHistory(&trade, "sell", 0.003, 101000, "", 0)
	scenarioAppendHistory(&trade, "BUY", 0.008, 94120, "", 0)

	event := scenarioBuildEvent(trade, "BTC", "0.012")

	got, err := actions.Sell(event)
	if err != nil {
		t.Fatalf("Sell returned error: %v", err)
	}
	// quantity = 0.015 - 0.003 = 0.012, ToFixed(0.012, 5) = 0.012
	const wantQty = 0.012
	if math.Abs(got.Params.Quantity-wantQty) > 1e-9 {
		t.Errorf("Sell quantity = %v, want %v", got.Params.Quantity, wantQty)
	}
}

// TestSpotPartial_FullCloseAfterPartialMatchesGrossProfit drives the trade
// to closure: same history as the prior test plus a final sell of all
// remaining inventory. GetProfit returns the realised quote-currency gain.
func TestSpotPartial_FullCloseAfterPartialMatchesGrossProfit(t *testing.T) {
	trade := scenarioBuildTrade("closed", 102000, false)
	scenarioAppendHistory(&trade, "buy", 0.001, 100000, "", 0)
	scenarioAppendHistory(&trade, "buy", 0.002, 98000, "", 0)
	scenarioAppendHistory(&trade, "buy", 0.004, 96040, "", 0)
	scenarioAppendHistory(&trade, "sell", 0.003, 101000, "", 0)
	scenarioAppendHistory(&trade, "buy", 0.008, 94120, "", 0)
	scenarioAppendHistory(&trade, "sell", 0.012, 102000, "", 0)

	profit := actions.GetProfit(trade)
	// buyTotal  = 0.001*100000 + 0.002*98000 + 0.004*96040 + 0.008*94120 = 1433.12
	// sellTotal = 0.003*101000 + 0.012*102000 = 303 + 1224 = 1527
	// dust=0 -> profit = 1527 - 1433.12 = 93.88
	const want = 93.88
	if math.Abs(profit-want) > 1e-6 {
		t.Errorf("GetProfit = %v, want %v", profit, want)
	}
}

// TestSpotPartial_DustFlagSetWhenQuantityRoundsDown asserts the Sell path
// tracks the truncated dust amount. Net qty 0.014985 with LotSize=5 rounds
// down to 0.01498 -> dust = 0.000005. The action records this in Trade.Dust
// so GetProfit can add it back during settlement.
func TestSpotPartial_DustFlagSetWhenQuantityRoundsDown(t *testing.T) {
	trade := scenarioBuildTrade("takeProfit", 101000, false)
	scenarioAppendHistory(&trade, "BUY", 0.015, 100000, "BTC", 0.000015)

	event := scenarioBuildEvent(trade, "BTC", "0.015")

	got, err := actions.Sell(event)
	if err != nil {
		t.Fatalf("Sell returned error: %v", err)
	}
	// quantityBeforeLotSize = 0.014985, after ToFixed = 0.01498, dust = 0.000005
	if math.Abs(got.Trade.Dust-0.000005) > 1e-9 {
		t.Errorf("dust = %v, want 0.000005", got.Trade.Dust)
	}
}

// TestSpotPartial_ClosedWhenNothingLeftToSell pins the corner case where the
// fee in base exceeds the bought quantity. quantity becomes <= 0 and Sell
// closes the trade without hitting the exchange.
func TestSpotPartial_ClosedWhenNothingLeftToSell(t *testing.T) {
	trade := scenarioBuildTrade("takeProfit", 101000, false)
	// buy 0.00001 BTC, fee 0.00001 BTC -> net 0, Sell closes
	scenarioAppendHistory(&trade, "BUY", 0.00001, 100000, "BTC", 0.00001)

	event := scenarioBuildEvent(trade, "BTC", "0.00001")

	got, err := actions.Sell(event)
	if err != nil {
		t.Fatalf("Sell returned error: %v", err)
	}
	if got.Trade.Status != "closed" {
		t.Errorf("Trade.Status = %q, want closed", got.Trade.Status)
	}
	if got.Trade.PendingOrder != 0 {
		t.Errorf("expected no exchange order when net qty is 0")
	}
}
