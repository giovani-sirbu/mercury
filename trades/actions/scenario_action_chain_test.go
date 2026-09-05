package actions_test

import (
	"testing"

	"github.com/giovani-sirbu/mercury/events"
)

// TestActionChain_BuyPipelineSpotSucceeds drives the full action pipeline an
// agora/hermes producer would invoke when DCA-buying mid-position:
// regulatePriceChange -> hasEnoughFunds -> hasFunds -> buy -> updateTrade.
// All five must succeed for a fresh DCA buy on BTC/USDC.
func TestActionChain_BuyPipelineSpotSucceeds(t *testing.T) {
	trade := scenarioBuildTrade("buy", 97800, false) // -2.2% from last entry
	scenarioAppendHistory(&trade, "BUY", 0.001, 100000, "", 0)

	event := scenarioBuildEvent(trade, "USDC", "1000")
	event.EventsNames = []string{
		"regulatePriceChange",
		"hasEnoughFunds",
		"hasFunds",
		"buy",
		"updateTrade",
	}

	if err := event.Run(); err != nil {
		t.Fatalf("buy pipeline failed: %v", err)
	}
}

// TestActionChain_BuyPipelineRejectsPriceTooClose breaks the chain at the
// regulatePriceChange step when the new buy is too close to the last entry.
// events.Run() reports the error via its event-error logger; we only assert
// the chain bails out before reaching buy.
func TestActionChain_BuyPipelineRejectsPriceTooClose(t *testing.T) {
	trade := scenarioBuildTrade("buy", 99500, false) // -0.5%
	scenarioAppendHistory(&trade, "BUY", 0.001, 100000, "", 0)

	event := scenarioBuildEvent(trade, "USDC", "1000")
	event.EventsNames = []string{
		"regulatePriceChange",
		"hasEnoughFunds",
		"hasFunds",
		"buy",
		"updateTrade",
	}

	if err := event.Run(); err == nil {
		t.Fatal("expected pipeline to short-circuit when price change is below threshold")
	}
}

// TestActionChain_SellPipelineSpotSucceeds drives the close pipeline:
// hasProfit -> hasFunds -> sell -> updateTrade. Used when the engine
// decides to take profit on a position.
func TestActionChain_SellPipelineSpotSucceeds(t *testing.T) {
	trade := scenarioBuildTrade("takeProfit", 101000, false)
	scenarioAppendHistory(&trade, "BUY", 0.001, 100000, "", 0)

	event := scenarioBuildEvent(trade, "BTC", "0.001")
	event.EventsNames = []string{
		"hasProfit",
		"hasFunds",
		"sell",
		"updateTrade",
	}

	if err := event.Run(); err != nil {
		t.Fatalf("sell pipeline failed: %v", err)
	}
}

// TestActionChain_SellPipelineRejectsUnprofitable proves the hasProfit gate
// stops the close chain when price is below cost basis. The Sell action is
// never invoked, so the wallet does not need to be funded.
func TestActionChain_SellPipelineRejectsUnprofitable(t *testing.T) {
	trade := scenarioBuildTrade("takeProfit", 95000, false)
	scenarioAppendHistory(&trade, "BUY", 0.001, 100000, "", 0)

	event := scenarioBuildEvent(trade, "BTC", "0.001")
	event.EventsNames = []string{
		"hasProfit",
		"hasFunds",
		"sell",
		"updateTrade",
	}

	if err := event.Run(); err == nil {
		t.Fatal("expected sell pipeline to short-circuit on unprofitable trade")
	}
}

// TestActionChain_SellPipelineInverseSucceeds mirrors the spot close
// pipeline for inverse. PositionPrice 99k after a sell at 100k is in
// profit; hasFunds checks USDC balance (the quote needed to buy back).
func TestActionChain_SellPipelineInverseSucceeds(t *testing.T) {
	trade := scenarioBuildTrade("takeProfit", 99000, true)
	scenarioAppendHistory(&trade, "SELL", 0.001, 100000, "", 0)

	event := scenarioBuildEvent(trade, "USDC", "200")
	event.EventsNames = []string{
		"hasProfit",
		"hasFunds",
		"sell",
		"updateTrade",
	}

	if err := event.Run(); err != nil {
		t.Fatalf("inverse sell pipeline failed: %v", err)
	}
}

// TestActionChain_BuyPipelineInverseSucceeds — inverse DCA buy: price has
// risen 2.5% above last entry, plenty of BTC, full chain green.
func TestActionChain_BuyPipelineInverseSucceeds(t *testing.T) {
	trade := scenarioBuildTrade("buy", 102500, true)
	scenarioAppendHistory(&trade, "SELL", 0.001, 100000, "", 0)

	event := scenarioBuildEvent(trade, "BTC", "0.1")
	event.EventsNames = []string{
		"regulatePriceChange",
		"hasEnoughFunds",
		"hasFunds",
		"buy",
		"updateTrade",
	}

	if err := event.Run(); err != nil {
		t.Fatalf("inverse buy pipeline failed: %v", err)
	}
}

// TestActionChain_CancelPendingThenBuyAllowsResumeAfterFailedOrder pins the
// flow used by the engine when a stale pending order needs to clear before
// a new entry: cancelPendingOrder -> regulatePriceChange -> hasFunds ->
// buy. Mirrors how hermes recovers when an old limit order is sitting on
// Binance.
func TestActionChain_CancelPendingThenBuyAllowsResumeAfterFailedOrder(t *testing.T) {
	trade := scenarioBuildTrade("buy", 97800, false)
	trade.PendingOrder = 9999 // simulate stale order id
	scenarioAppendHistory(&trade, "BUY", 0.001, 100000, "", 0)

	event := scenarioBuildEvent(trade, "USDC", "1000")
	event.EventsNames = []string{
		"cancelPendingOrder",
		"regulatePriceChange",
		"hasEnoughFunds",
		"hasFunds",
		"buy",
		"updateTrade",
	}

	if err := event.Run(); err != nil {
		t.Fatalf("cancel+buy pipeline failed: %v", err)
	}
}

// TestActionChain_RunNoOpWhenEmptyEventsNames sanity-checks the base case of
// events.Run(): an empty pipeline returns nil immediately.
func TestActionChain_RunNoOpWhenEmptyEventsNames(t *testing.T) {
	event := events.Events{}
	if err := event.Run(); err != nil {
		t.Errorf("Run on empty pipeline returned error: %v", err)
	}
}

// TestActionChain_AddRegistersOverrideAction verifies events.Add: replacing
// a default action lets a test inject a deterministic stub without mutating
// the action map shared by other tests.
func TestActionChain_AddRegistersOverrideAction(t *testing.T) {
	called := false
	stub := func(e events.Events) (events.Events, error) {
		called = true
		return e, nil
	}

	trade := scenarioBuildTrade("buy", 100000, false)
	event := scenarioBuildEvent(trade, "USDC", "0").Add("buy", stub)
	event.EventsNames = []string{"buy"}

	if err := event.Run(); err != nil {
		t.Fatalf("Run with stubbed buy returned error: %v", err)
	}
	if !called {
		t.Error("expected stubbed buy action to be invoked")
	}
}
