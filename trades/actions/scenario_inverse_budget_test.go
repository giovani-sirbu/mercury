package actions_test

import (
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/actions"
)

// TestInverseBudget_FirstSellFailsWhenBaseAssetEmpty mirrors the spot empty-
// wallet first-buy failure but inverted: for inverse the first action needs
// base asset (BTC). With zero BTC, CalculateInitialBid cannot satisfy
// MinNotional and Buy errors out.
func TestInverseBudget_FirstSellFailsWhenBaseAssetEmpty(t *testing.T) {
	trade := scenarioBuildTrade("buy", 100000, true)

	event := scenarioBuildEvent(trade, "BTC", "0")

	_, err := actions.Buy(event)
	if err == nil {
		t.Fatal("expected Buy (inverse first action) to error when BTC wallet is empty")
	}
	if !strings.Contains(err.Error(), "Insufficient funds") {
		t.Errorf("expected insufficient-funds error, got: %v", err)
	}
}

// TestInverseBudget_HasFundsRejectsWhenBaseBalanceBelowNeeded asserts the
// inverse HasFunds path: with five inverse sells already on the books, the
// next x2 multiplier sell needs more BTC than the wallet holds.
func TestInverseBudget_HasFundsRejectsWhenBaseBalanceBelowNeeded(t *testing.T) {
	trade := scenarioBuildTrade("buy", 110000, true)
	scenarioAppendHistory(&trade, "SELL", 0.001, 100000, "", 0)
	scenarioAppendHistory(&trade, "SELL", 0.002, 102000, "", 0)
	scenarioAppendHistory(&trade, "SELL", 0.004, 104040, "", 0)
	scenarioAppendHistory(&trade, "SELL", 0.008, 106120, "", 0)
	scenarioAppendHistory(&trade, "SELL", 0.016, 108240, "", 0)

	// Tiny BTC balance left, far less than the x2 multiplier needs.
	event := scenarioBuildEvent(trade, "BTC", "0.0001")

	_, err := actions.HasFunds(event)
	if err == nil {
		t.Fatal("expected inverse HasFunds to reject when BTC balance is below needed")
	}
}

// TestInverseBudget_HasFundsForTakeProfitChecksQuoteAsset documents that
// inverse takeProfit/sellParent positions check the QUOTE asset (USDC) on
// the wallet, since closing an inverse position requires buying back the
// base from USDC.
func TestInverseBudget_HasFundsForTakeProfitChecksQuoteAsset(t *testing.T) {
	trade := scenarioBuildTrade("takeProfit", 99000, true)
	scenarioAppendHistory(&trade, "SELL", 0.001, 100000, "", 0)

	// Plenty of BTC, but no USDC -> reject because closing the inverse needs
	// quote currency.
	event := scenarioBuildEvent(trade, "BTC", "10")

	_, err := actions.HasFunds(event)
	if err == nil {
		t.Fatal("expected inverse HasFunds to reject takeProfit with no quote balance")
	}
}

// TestInverseBudget_HasFundsAllowsInverseBuyWhenBaseSufficient mirrors the
// happy path: plenty of BTC for the next inverse buy.
func TestInverseBudget_HasFundsAllowsInverseBuyWhenBaseSufficient(t *testing.T) {
	trade := scenarioBuildTrade("buy", 100000, true)
	scenarioAppendHistory(&trade, "SELL", 0.001, 100000, "", 0)

	event := scenarioBuildEvent(trade, "BTC", "1")

	got, err := actions.HasFunds(event)
	if err != nil {
		t.Fatalf("HasFunds rejected the happy inverse path: %v", err)
	}
	if got.Trade.PositionType != "buy" {
		t.Errorf("PositionType unexpectedly changed to %q", got.Trade.PositionType)
	}
}
