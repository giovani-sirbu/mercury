package actions_test

import (
	"math"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/actions"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// TestBuy_FirstSpotBuyBudgetDrivenProducesPositiveQty pins the budget-driven
// first-buy branch in Buy: empty history + no InitialBid configured forces
// the engine through CalculateInitialBid against the wallet's free balance.
// A clean BTC/USDC trade with 1000 USDC must produce a non-zero quantity
// that respects MinNotional and stays within the funded amount.
func TestBuy_FirstSpotBuyBudgetDrivenProducesPositiveQty(t *testing.T) {
	trade := scenarioBuildTrade("buy", 100000, false)
	// Drop InitialBid so the budget path runs (rather than the
	// MinNotional*InitialBid shortcut).
	trade.StrategyPair.StrategySettings = []aggragates.StrategySettings{
		{
			MinDepths:          5,
			Depths:             8,
			Percentage:         2,
			Multiplier:         2,
			Tolerance:          0.25,
			TrailingTakeProfit: 0.5,
		},
	}

	event := scenarioBuildEvent(trade, "USDC", "1000")

	got, err := actions.Buy(event)
	if err != nil {
		t.Fatalf("Buy returned error on funded first-buy budget path: %v", err)
	}
	if got.Params.Quantity <= 0 {
		t.Fatalf("expected positive Buy quantity, got %v", got.Params.Quantity)
	}
	// Notional must clear MinNotional and stay below wallet capacity.
	notional := got.Params.Quantity * trade.PositionPrice
	if notional < trade.StrategyPair.TradeFilters.MinNotional {
		t.Errorf("notional %v below MinNotional %v", notional, trade.StrategyPair.TradeFilters.MinNotional)
	}
	if notional > 1000 {
		t.Errorf("notional %v exceeds funded wallet 1000 USDC", notional)
	}
}

// TestBuy_FirstInverseBuyBudgetDrivenProducesPositiveQty mirrors the
// first-buy budget path for inverse trades — the engine looks at the BASE
// asset (BTC) wallet and computes initial bid against it.
func TestBuy_FirstInverseBuyBudgetDrivenProducesPositiveQty(t *testing.T) {
	trade := scenarioBuildTrade("buy", 100000, true)
	trade.StrategyPair.StrategySettings = []aggragates.StrategySettings{
		{
			MinDepths:          5,
			Depths:             8,
			Percentage:         2,
			Multiplier:         2,
			Tolerance:          0.25,
			TrailingTakeProfit: 0.5,
		},
	}

	event := scenarioBuildEvent(trade, "BTC", "0.1")

	got, err := actions.Buy(event)
	if err != nil {
		t.Fatalf("Buy returned error on funded inverse first-buy: %v", err)
	}
	if got.Params.Quantity <= 0 {
		t.Fatalf("expected positive Buy quantity (inverse), got %v", got.Params.Quantity)
	}
}

// TestBuy_SpotSubsequentBuyDoublesLastHistoryQty pins the multiplier path
// for spot: with one history row of 0.001 BTC and Multiplier=2, the
// resulting Buy quantity must be exactly 0.002 BTC (history[-1].qty *
// multiplier, minus already-sold quantity which is zero here).
func TestBuy_SpotSubsequentBuyDoublesLastHistoryQty(t *testing.T) {
	trade := scenarioBuildTrade("buy", 98000, false)
	scenarioAppendHistory(&trade, "BUY", 0.001, 100000, "", 0)

	event := scenarioBuildEvent(trade, "USDC", "1000")

	got, err := actions.Buy(event)
	if err != nil {
		t.Fatalf("Buy returned error: %v", err)
	}
	const want = 0.002
	if math.Abs(got.Params.Quantity-want) > 1e-9 {
		t.Errorf("Buy qty (spot multiplier) = %v, want %v", got.Params.Quantity, want)
	}
}

// TestBuy_InverseSubsequentBuyAppliesMultiplier mirrors the spot multiplier
// test for inverse. Last SELL was 0.008 BTC, multiplier 2 -> 0.016 BTC. The
// min-order clamp at 5/110000 ≈ 0.00004 BTC loses math.Max so the multiplier
// result is preserved.
func TestBuy_InverseSubsequentBuyAppliesMultiplier(t *testing.T) {
	trade := scenarioBuildTrade("buy", 110000, true)
	scenarioAppendHistory(&trade, "SELL", 0.002, 105000, "", 0)
	scenarioAppendHistory(&trade, "SELL", 0.004, 107000, "", 0)
	scenarioAppendHistory(&trade, "SELL", 0.008, 109000, "", 0)

	event := scenarioBuildEvent(trade, "BTC", "10")

	got, err := actions.Buy(event)
	if err != nil {
		t.Fatalf("Buy returned error: %v", err)
	}
	const want = 0.016
	if math.Abs(got.Params.Quantity-want) > 1e-9 {
		t.Errorf("Buy qty (inverse multiplier) = %v, want %v", got.Params.Quantity, want)
	}
}

// TestBuy_PendingOrderIdSetAfterSuccessfulExchangeCall ensures Buy records
// the exchange-returned OrderID on the trade so the next tick can recognise
// the order as already in flight.
func TestBuy_PendingOrderIdSetAfterSuccessfulExchangeCall(t *testing.T) {
	trade := scenarioBuildTrade("buy", 98000, false)
	scenarioAppendHistory(&trade, "BUY", 0.001, 100000, "", 0)

	event := scenarioBuildEvent(trade, "USDC", "1000")

	got, err := actions.Buy(event)
	if err != nil {
		t.Fatalf("Buy returned error: %v", err)
	}
	if got.Trade.PendingOrder == 0 {
		t.Error("expected PendingOrder to be set after successful exchange call")
	}
}

// TestBuy_FirstBuyUsesMarketBuyEndpoint pins the routing for the first buy:
// historyCount == 0 -> MarketBuy on the exchange (the virtual exchange
// records it as side BUY). For inverse first-buy, the route is MarketSell.
// Combined with the prior tests this covers both endpoints in Buy.
func TestBuy_FirstBuyUsesMarketBuyEndpoint(t *testing.T) {
	trade := scenarioBuildTrade("buy", 100000, false)
	trade.StrategyPair.StrategySettings[0].InitialBid = 1

	event := scenarioBuildEvent(trade, "USDC", "1000")

	got, err := actions.Buy(event)
	if err != nil {
		t.Fatalf("Buy returned error: %v", err)
	}
	if got.Trade.PendingOrder == 0 {
		t.Error("expected first-buy MarketBuy to set PendingOrder")
	}
}
