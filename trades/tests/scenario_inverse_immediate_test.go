package tests

import (
	"math"
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/actions"
)

// TestInverseImmediate_BuyBackAfterSinglySoldAtLowerPrice covers the
// simplest inverse close: one inverse "buy" (SELL on exchange), then price
// drops, close with exchange buy.
func TestInverseImmediate_BuyBackAfterSinglySoldAtLowerPrice(t *testing.T) {
	trade := scenarioBuildTrade("takeProfit", 99000, true)
	scenarioAppendHistory(&trade, "SELL", 0.001, 100000, "", 0)

	event := scenarioBuildEvent(trade, "USDC", "100")

	got, err := actions.Sell(event)
	if err != nil {
		t.Fatalf("Sell returned error: %v", err)
	}
	// inverse: qty = (100000*0.001 - 0) / 99000 = 0.00101..., ToFixed(5) = 0.00101
	const want = 0.00101
	if math.Abs(got.Params.Quantity-want) > 1e-5 {
		t.Errorf("Sell qty (inverse immediate) = %v, want %v", got.Params.Quantity, want)
	}
	if got.Trade.PendingOrder == 0 {
		t.Errorf("expected pending order set after inverse close")
	}
}

// TestInverseImmediate_HasProfitOnSingleSell mirrors the spot single-buy
// HasProfit path for inverse.
func TestInverseImmediate_HasProfitOnSingleSell(t *testing.T) {
	trade := scenarioBuildTrade("takeProfit", 99000, true)
	scenarioAppendHistory(&trade, "SELL", 0.001, 100000, "", 0)

	event := events.Events{Trade: trade}

	got, err := actions.HasProfit(event)
	if err != nil {
		t.Fatalf("HasProfit rejected immediate inverse profit: %v", err)
	}
	if got.Trade.Profit <= 0 {
		t.Errorf("expected positive Profit, got %v", got.Trade.Profit)
	}
}

// TestInverseImmediate_MarketSellOrderRoutesToMarketBuy proves the routing
// inside Sell: inverse + MarketSellOrder -> client.MarketBuy.
func TestInverseImmediate_MarketSellOrderRoutesToMarketBuy(t *testing.T) {
	trade := scenarioBuildTrade("takeProfit", 99000, true)
	scenarioAppendHistory(&trade, "SELL", 0.001, 100000, "", 0)

	event := scenarioBuildEvent(trade, "USDC", "100")
	event.Params.MarketSellOrder = true

	got, err := actions.Sell(event)
	if err != nil {
		t.Fatalf("Sell returned error: %v", err)
	}
	if got.Params.Quantity <= 0 {
		t.Errorf("expected positive market-buy quantity, got %v", got.Params.Quantity)
	}
}

// TestInverseImmediate_FeesInQuoteShrinkCloseQuantity asserts that quote-
// currency fees recorded on inverse sells reduce the close quantity (Sell
// subtracts feeInQuote from sellQuantity in quote before dividing).
func TestInverseImmediate_FeesInQuoteShrinkCloseQuantity(t *testing.T) {
	trade := scenarioBuildTrade("takeProfit", 99000, true)
	scenarioAppendHistory(&trade, "SELL", 0.001, 100000, "USDC", 0.1)

	event := scenarioBuildEvent(trade, "USDC", "100")

	got, err := actions.Sell(event)
	if err != nil {
		t.Fatalf("Sell returned error: %v", err)
	}
	// qty = (100 - 0 - 0.1) / 99000 - feeInBase(0) = 0.001008..., ToFixed(5) = 0.00100
	const want = 0.001
	if math.Abs(got.Params.Quantity-want) > 1e-5 {
		t.Errorf("Sell qty = %v, want %v", got.Params.Quantity, want)
	}
}
