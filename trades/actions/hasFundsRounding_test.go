package actions_test

import (
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/actions"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/virtualexchange"
)

// The scenario pair prices BTC at 100000 with a x2 multiplier, so a single
// 0.001 BTC fill makes the next entry need 200 USDC. Every case below moves
// the wallet around that number.
const roundingEntryNeeds = 200.0

// A wallet a fraction under what the next entry needs is the case the
// allowance exists for: the entry is placed for what the wallet holds and the
// trade keeps laddering, instead of blocking on a shortfall it can absorb.
func TestHasFundsRounding_WaivesShortfallInsideTheAllowance(t *testing.T) {
	trade := scenarioBuildTrade("buy", 100000, false)
	scenarioAppendHistory(&trade, "BUY", 0.001, 100000, "", 0)

	event := scenarioBuildEvent(trade, "USDC", "195")

	got, err := actions.HasFunds(event)
	AssertNoError(t, err)

	if got.Trade.Status == aggragates.Blocked {
		t.Error("a shortfall inside the allowance must not block the trade")
	}
	if got.Trade.PositionType != "buy" {
		t.Errorf("PositionType changed unexpectedly to %q", got.Trade.PositionType)
	}
	AssertFloatEqual(t, got.Params.AvailableQuantity, 195, 1e-9, "waived entry must carry the wallet balance")
}

// The allowance is inclusive: a shortfall of exactly
// AllowRoundedQuantityPercentage is still waived.
func TestHasFundsRounding_WaivesTheShortfallAtTheAllowanceEdge(t *testing.T) {
	trade := scenarioBuildTrade("buy", 100000, false)
	scenarioAppendHistory(&trade, "BUY", 0.001, 100000, "", 0)

	edge := roundingEntryNeeds - roundingEntryNeeds*actions.AllowRoundedQuantityPercentage/100
	event := scenarioBuildEvent(trade, "USDC", "180")

	got, err := actions.HasFunds(event)
	AssertNoError(t, err)
	AssertFloatEqual(t, got.Params.AvailableQuantity, edge, 1e-9, "the edge of the allowance must still be waived")
}

// Past the allowance the wallet is genuinely short, and the trade blocks with
// the same error and the same Blocked status it had before the waiver existed.
func TestHasFundsRounding_BlocksShortfallOutsideTheAllowance(t *testing.T) {
	trade := scenarioBuildTrade("buy", 100000, false)
	scenarioAppendHistory(&trade, "BUY", 0.001, 100000, "", 0)

	event := scenarioBuildEvent(trade, "USDC", "175")

	got, err := actions.HasFunds(event)
	AssertError(t, err)

	if !strings.Contains(err.Error(), "Insufficient funds") {
		t.Errorf("expected insufficient-funds error, got: %v", err)
	}
	if got.Trade.Status != aggragates.Blocked {
		t.Errorf("expected the trade blocked on the shortfall, got %q", got.Trade.Status)
	}
	if got.Params.AvailableQuantity != 0 {
		t.Errorf("a blocked entry must carry no wallet balance, got %f", got.Params.AvailableQuantity)
	}
}

// The closing side keeps the strict check. A shortfall there is a missing
// position, and a close trimmed to the wallet leaves a remainder no later
// tick can sell once it falls under the exchange minimum.
func TestHasFundsRounding_LeavesTheClosingSideBlocked(t *testing.T) {
	trade := scenarioBuildTrade("takeProfit", 101000, false)
	scenarioAppendHistory(&trade, "BUY", 0.001, 100000, "", 0)

	// 2% of the position missing from the wallet: inside the allowance if the
	// closing side were subject to it.
	event := scenarioBuildEvent(trade, "BTC", "0.00098")

	got, err := actions.HasFunds(event)
	AssertError(t, err)

	if !strings.Contains(err.Error(), "Insufficient funds") {
		t.Errorf("expected insufficient-funds error, got: %v", err)
	}
	if got.Params.AvailableQuantity != 0 {
		t.Errorf("a close must never carry a waived wallet balance, got %f", got.Params.AvailableQuantity)
	}
}

// An inverse close is the same rule read against the other asset: it buys the
// base back with the quote, so the shortfall is counted in quote, and it
// blocks for the same reason a spot close does.
func TestHasFundsRounding_LeavesTheInverseCloseBlocked(t *testing.T) {
	trade := scenarioBuildTrade("takeProfit", 99000, true)
	scenarioAppendHistory(&trade, "SELL", 0.001, 100000, "", 0)

	// Closing the position needs 99 USDC; the wallet is ~2% under it.
	event := scenarioBuildEvent(trade, "USDC", "97")

	got, err := actions.HasFunds(event)
	AssertError(t, err)

	if !strings.Contains(err.Error(), "Insufficient funds") {
		t.Errorf("expected insufficient-funds error, got: %v", err)
	}
	if got.Params.AvailableQuantity != 0 {
		t.Errorf("a close must never carry a waived wallet balance, got %f", got.Params.AvailableQuantity)
	}
}

// Waiving a shortfall the exchange would reject anyway is worse than blocking:
// Buy raises a quantity under the minimum back up to it, which puts the order
// over the wallet again. Those shortfalls keep blocking.
func TestHasFundsRounding_RefusesWhenTheAffordableEntryFallsUnderTheMinimum(t *testing.T) {
	trade := scenarioBuildTrade("buy", 100000, false)
	// A next entry worth barely more than MinNotional: 0.000026 BTC x2 at
	// 100000 needs 5.2 USDC against a 5 USDC minimum.
	scenarioAppendHistory(&trade, "BUY", 0.000026, 100000, "", 0)

	// 4.9 USDC is under 10% short, but buys less than the minimum order.
	event := scenarioBuildEvent(trade, "USDC", "4.9")

	_, err := actions.HasFunds(event)
	AssertError(t, err)

	if !strings.Contains(err.Error(), "Insufficient funds") {
		t.Errorf("expected insufficient-funds error, got: %v", err)
	}
}

// An inverse entry spends the base asset and its needed quantity is already
// counted in it, so the waiver names the base balance untouched by the price.
func TestHasFundsRounding_InverseWaivesAgainstTheBaseBalance(t *testing.T) {
	trade := scenarioBuildTrade("buy", 100000, true)
	scenarioAppendHistory(&trade, "SELL", 0.001, 100000, "", 0)

	event := scenarioBuildEvent(trade, "BTC", "0.00195")

	got, err := actions.HasFunds(event)
	AssertNoError(t, err)
	AssertFloatEqual(t, got.Params.AvailableQuantity, 0.00195, 1e-12, "inverse entry must carry the base balance")
}

// The waiver is only half the behaviour: the entry that follows it has to be
// sized to the wallet, or the exchange rejects what HasFunds just let through.
// This runs the real chain so the handoff through Params is covered too.
func TestHasFundsRounding_BuyPlacesTheQuantityTheWalletAffords(t *testing.T) {
	trade := scenarioBuildTrade("buy", 100000, false)
	scenarioAppendHistory(&trade, "BUY", 0.001, 100000, "", 0)

	placed, err := runRoundingEntryChain(trade, "USDC", "195")
	AssertNoError(t, err)

	// 195 USDC at 100000 buys 0.00195 BTC; the ladder asked for 0.002.
	AssertFloatEqual(t, placed, 0.00195, 1e-12, "entry quantity")
	if placed*trade.PositionPrice > 195 {
		t.Errorf("entry costs %f USDC, more than the wallet holds", placed*trade.PositionPrice)
	}
}

// The inverse entry sells the base, so the balance caps the quantity directly
// with no price in the conversion. Same chain, other side of the pair.
func TestHasFundsRounding_InverseEntryPlacesTheQuantityTheWalletAffords(t *testing.T) {
	trade := scenarioBuildTrade("buy", 100000, true)
	scenarioAppendHistory(&trade, "SELL", 0.001, 100000, "", 0)

	placed, err := runRoundingEntryChain(trade, "BTC", "0.00195")
	AssertNoError(t, err)

	// The ladder asked for 0.002 BTC; the wallet holds 0.00195.
	AssertFloatEqual(t, placed, 0.00195, 1e-12, "inverse entry quantity")
	if placed > 0.00195 {
		t.Errorf("inverse entry sells %f BTC, more than the wallet holds", placed)
	}
}

// A whole wallet is untouched by any of this: the ladder's own quantity stands.
func TestHasFundsRounding_BuyKeepsTheLadderQuantityWhenTheWalletIsWhole(t *testing.T) {
	trade := scenarioBuildTrade("buy", 100000, false)
	scenarioAppendHistory(&trade, "BUY", 0.001, 100000, "", 0)

	placed, err := runRoundingEntryChain(trade, "USDC", "1000000")
	AssertNoError(t, err)

	AssertFloatEqual(t, placed, 0.002, 1e-12, "entry quantity")
}

// runRoundingEntryChain runs hasFunds -> buy -> updateTrade against a wallet
// holding freeAmount of asset and returns the quantity Buy placed.
func runRoundingEntryChain(trade aggragates.Trades, asset string, freeAmount string) (float64, error) {
	virtualexchange.ResetWallet()

	var placed float64
	chainActions := actions.GetDefaultActions()
	chainActions["updateTrade"] = func(event events.Events) (events.Events, error) {
		placed = event.Params.Quantity
		return event, nil
	}

	event := events.Events{
		Trade:       trade,
		Exchange:    GetVirtualExchange(asset, freeAmount),
		Events:      chainActions,
		EventsNames: []string{"hasFunds", "buy", "updateTrade"},
	}

	// The chain has to finish before `placed` is read: a single return
	// statement would evaluate it first and always report zero.
	err := event.Run()

	return placed, err
}
