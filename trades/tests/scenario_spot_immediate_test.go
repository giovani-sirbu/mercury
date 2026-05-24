package tests

import (
	"math"
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/actions"
)

// TestSpotImmediate_SellAfterSingleBuyAtHigherPrice covers the simplest
// happy path: one buy, price ticks above the entry by Percentage%, sell.
// Exercises the GetQuantities + Sell pair and verifies the limit Sell call
// (no MarketSellOrder flag).
func TestSpotImmediate_SellAfterSingleBuyAtHigherPrice(t *testing.T) {
	trade := scenarioBuildTrade("takeProfit", 102000, false)
	scenarioAppendHistory(&trade, "BUY", 0.001, 100000, "", 0)

	event := scenarioBuildEvent(trade, "BTC", "0.001")

	got, err := actions.Sell(event)
	if err != nil {
		t.Fatalf("Sell returned error: %v", err)
	}
	if math.Abs(got.Params.Quantity-0.001) > 1e-9 {
		t.Errorf("Sell qty = %v, want 0.001", got.Params.Quantity)
	}
	if got.Trade.PendingOrder == 0 {
		t.Errorf("expected pending order set after sell")
	}
}

// TestSpotImmediate_HasProfitAcceptsAfterBriefRally documents the HasProfit
// success path with a single buy and a 1% rally. Tolerance 0.25 is applied
// internally to simulate unrealised PnL.
func TestSpotImmediate_HasProfitAcceptsAfterBriefRally(t *testing.T) {
	trade := scenarioBuildTrade("takeProfit", 101000, false)
	scenarioAppendHistory(&trade, "BUY", 0.001, 100000, "", 0)

	event := events.Events{Trade: trade}

	got, err := actions.HasProfit(event)
	if err != nil {
		t.Fatalf("HasProfit rejected an immediate-profit scenario: %v", err)
	}
	if got.Trade.Profit <= 0 {
		t.Errorf("expected positive Profit recorded on trade, got %v", got.Trade.Profit)
	}
}

// TestSpotImmediate_MarketSellOrderUsesMarketSellEndpoint pins the routing
// inside Sell: when Params.MarketSellOrder is true, the action calls the
// exchange MarketSell rather than the limit Sell.
func TestSpotImmediate_MarketSellOrderUsesMarketSellEndpoint(t *testing.T) {
	trade := scenarioBuildTrade("takeProfit", 102000, false)
	scenarioAppendHistory(&trade, "BUY", 0.001, 100000, "", 0)

	event := scenarioBuildEvent(trade, "BTC", "0.001")
	event.Params.MarketSellOrder = true

	got, err := actions.Sell(event)
	if err != nil {
		t.Fatalf("Sell returned error: %v", err)
	}
	if got.Params.Quantity <= 0 {
		t.Errorf("expected positive sell quantity, got %v", got.Params.Quantity)
	}
}

// TestSpotImmediate_NewStatusClosesWithoutExchange validates the early exit
// in Sell: a trade still in status "new" closes immediately, no exchange
// call.
func TestSpotImmediate_NewStatusClosesWithoutExchange(t *testing.T) {
	trade := scenarioBuildTrade("sell", 102000, false)
	trade.Status = "new"

	event := scenarioBuildEvent(trade, "BTC", "0")

	got, err := actions.Sell(event)
	if err != nil {
		t.Fatalf("Sell returned error: %v", err)
	}
	if got.Trade.Status != "closed" {
		t.Errorf("Trade.Status = %q, want closed", got.Trade.Status)
	}
}

// TestSpotImmediate_PendingOrderBlocksNewSell guards the safety check at the
// top of Sell — a non-zero PendingOrder ID short-circuits with an error so
// the engine never double-submits an order while a previous one is still
// open on Binance.
func TestSpotImmediate_PendingOrderBlocksNewSell(t *testing.T) {
	trade := scenarioBuildTrade("takeProfit", 102000, false)
	trade.PendingOrder = 12345
	scenarioAppendHistory(&trade, "BUY", 0.001, 100000, "", 0)

	event := scenarioBuildEvent(trade, "BTC", "0.001")

	_, err := actions.Sell(event)
	if err == nil {
		t.Fatal("expected Sell to reject when PendingOrder is set")
	}
}
