package actions_test

import (
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/actions"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// TestSpotBudget_FirstBuyFailsWhenWalletEmpty pins the very first DCA step:
// no history, empty wallet, CalculateInitialBid cannot satisfy MinNotional.
// Buy must propagate the resulting error via SaveError -> updateTrade stub.
func TestSpotBudget_FirstBuyFailsWhenWalletEmpty(t *testing.T) {
	trade := scenarioBuildTrade("buy", 100000, false)

	event := scenarioBuildEvent(trade, "USDC", "0")

	_, err := actions.Buy(event)
	if err == nil {
		t.Fatal("expected Buy to error when wallet is empty and no InitialBid is set")
	}
	if !strings.Contains(err.Error(), "Insufficient funds") {
		t.Errorf("expected insufficient-funds error, got: %v", err)
	}
}

// TestSpotBudget_HasFundsRejectsWhenBalanceBelowNeeded simulates 6 prior
// buys (typical mid-DCA state) plus a tiny remaining balance. HasFunds
// computes the next buy's needed quote and rejects.
func TestSpotBudget_HasFundsRejectsWhenBalanceBelowNeeded(t *testing.T) {
	trade := scenarioBuildTrade("buy", 92000, false)
	scenarioAppendHistory(&trade, "BUY", 0.001, 100000, "", 0)
	scenarioAppendHistory(&trade, "BUY", 0.002, 98000, "", 0)
	scenarioAppendHistory(&trade, "BUY", 0.004, 96040, "", 0)
	scenarioAppendHistory(&trade, "BUY", 0.008, 94120, "", 0)
	scenarioAppendHistory(&trade, "BUY", 0.016, 92240, "", 0)

	// Only 10 USDC free — needed for the next x2 multiplier buy is far larger.
	event := scenarioBuildEvent(trade, "USDC", "10")

	_, err := actions.HasFunds(event)
	if err == nil {
		t.Fatal("expected HasFunds to reject when wallet balance is below needed quantity")
	}
	if !strings.Contains(err.Error(), "Insufficient funds") {
		t.Errorf("expected insufficient-funds error, got: %v", err)
	}
}

// TestSpotBudget_HasFundsRejectsAndTriggersImpasse verifies that when the
// trade has Strategy.Params.Impasse=true and no ParentID, HasFunds flips
// PositionType to "impasse" on top of saving the error. This is the gate
// for CreateChildrenTrades to fire downstream.
func TestSpotBudget_HasFundsRejectsAndTriggersImpasse(t *testing.T) {
	trade := scenarioBuildTrade("buy", 92000, false)
	trade.Strategy.Params.Impasse = true
	// Sufficient bought volume so CalculateInitialBid clears MinNotional and
	// the impasse branch fires. Lower volumes would short-circuit and leave
	// PositionType untouched.
	scenarioAppendHistory(&trade, "BUY", 0.01, 100000, "", 0)
	scenarioAppendHistory(&trade, "BUY", 0.02, 98000, "", 0)
	scenarioAppendHistory(&trade, "BUY", 0.04, 96040, "", 0)
	scenarioAppendHistory(&trade, "BUY", 0.08, 94120, "", 0)

	event := scenarioBuildEvent(trade, "USDC", "5")

	got, err := actions.HasFunds(event)
	if err == nil {
		t.Fatal("expected HasFunds to reject")
	}
	if got.Trade.PositionType != "impasse" {
		t.Errorf("PositionType = %q, want impasse", got.Trade.PositionType)
	}
}

// TestSpotBudget_HasFundsAllowsBuyWhenBalanceSufficient sanity-checks the
// happy path: wallet has plenty of USDC for a fresh buy, HasFunds returns
// the trade unchanged.
func TestSpotBudget_HasFundsAllowsBuyWhenBalanceSufficient(t *testing.T) {
	trade := scenarioBuildTrade("buy", 100000, false)
	scenarioAppendHistory(&trade, "BUY", 0.001, 100000, "", 0)

	event := scenarioBuildEvent(trade, "USDC", "100000")

	got, err := actions.HasFunds(event)
	if err != nil {
		t.Fatalf("HasFunds rejected the happy path: %v", err)
	}
	if got.Trade.PositionType != "buy" {
		t.Errorf("PositionType changed unexpectedly to %q", got.Trade.PositionType)
	}
}

// TestSpotBudget_HasFundsForSellChecksBaseAsset proves the routing inside
// GetFundsQuantities — when PositionType is takeProfit/sell/sellParent, the
// wallet check looks at the BASE asset (BTC), not the quote.
func TestSpotBudget_HasFundsForSellChecksBaseAsset(t *testing.T) {
	trade := scenarioBuildTrade("takeProfit", 101000, false)
	scenarioAppendHistory(&trade, "BUY", 0.001, 100000, "", 0)

	// Plenty of USDC, but zero BTC -> HasFunds must reject because the sell
	// needs base inventory, not quote.
	event := scenarioBuildEvent(trade, "USDC", "100000")

	_, err := actions.HasFunds(event)
	if err == nil {
		t.Fatal("expected HasFunds to reject sell action with zero base balance")
	}
}

// TestSpotBudget_HasEnoughFundsTopUpInjectsAdjustment exercises the
// negative-quantity correction path: a history with a negative quantity
// (engine-recorded short-fall) plus a remaining balance below need triggers
// HasEnoughFunds to append an ADJUST history row.
func TestSpotBudget_HasEnoughFundsTopUpInjectsAdjustment(t *testing.T) {
	trade := scenarioBuildTrade("sellParent", 100000, false)
	scenarioAppendHistory(&trade, "BUY", 0.001, 100000, "", 0)
	// Engine recorded a negative qty (under-fill from a partial earlier sell)
	trade.History = append(trade.History, aggragates.TradesHistory{
		Quantity: -0.0005, Price: 1e-13, Type: "BUY",
	})

	event := scenarioBuildEvent(trade, "BTC", "0")

	got, err := actions.HasEnoughFunds(event)
	if err != nil {
		t.Fatalf("HasEnoughFunds returned error: %v", err)
	}
	// Last appended history row should be the ADJUST entry.
	last := got.Trade.History[len(got.Trade.History)-1]
	if last.Status != "ADJUST" {
		t.Errorf("expected last history entry status ADJUST, got %q", last.Status)
	}
}
