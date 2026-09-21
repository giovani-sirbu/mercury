package actions_test

import (
	"math"
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/actions"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/fees"
	"github.com/giovani-sirbu/mercury/trades/profit"
	"github.com/giovani-sirbu/mercury/trades/quantities"
)

// dustEntryPrice is the price the single entry below filled at.
const dustEntryPrice = 95862.54

// minimumNotionalEvent is the position a depth ladder opens when the wallet is
// nearly all committed: one entry bumped to the pair's minimum notional — six
// lot steps of base — with its commission billed in base, so flooring what is
// left back to the lot size cuts a whole step.
//
// Built fresh per call: the actions below append a simulated close onto the
// history they are handed.
func minimumNotionalEvent(positionType string, price float64) events.Events {
	trade := scenarioBuildTrade(positionType, price, false)
	scenarioAppendHistory(&trade, "BUY", 0.00006, dustEntryPrice, "BTC", 0.00000006)
	return events.Events{Trade: trade}
}

// closeNetProfitWithoutDust is what the actions below compute when the lot-size
// remainder is thrown away — the control the fix is measured against.
func closeNetProfitWithoutDust(event events.Events, closePrice float64) float64 {
	quantity, _, historyType := quantities.SimulatedClose(event)

	trade := event.Trade
	trade.History = append(trade.History, aggragates.TradesHistory{
		Type:     historyType,
		Quantity: quantity,
		Price:    closePrice,
	})

	return profit.GetProfit(trade) - fees.GetFees(event) - fees.UnembodiedOpeningFees(event)
}

// TestHasProfitMinimumNotionalPositionCanStillClose is the regression for a
// position the gate could never let go of.
//
// Valuing the remainder at zero costs a sixth of this position, which no 2%
// target can recover: the gate wanted a price above the highest BTC had ever
// traded at, and the trade sat open until the run ended. Counting it the way
// the close itself does puts the exit back inside the market.
func TestHasProfitMinimumNotionalPositionCanStillClose(t *testing.T) {
	// A price the market reached days after this entry filled.
	const reachablePrice = 98000.0

	got, err := actions.HasProfit(minimumNotionalEvent("takeProfit", reachablePrice))
	if err != nil {
		t.Fatalf("HasProfit refused a close the market reached: %v (net profit %v)", err, got.Trade.Profit)
	}

	// The gate still has to refuse a price the round trip cannot pay for, or
	// the fix would have turned it into a pass-through.
	if _, err := actions.HasProfit(minimumNotionalEvent("takeProfit", dustEntryPrice)); err == nil {
		t.Fatal("HasProfit cleared a close at the entry price")
	}
}

// TestAcceptLossReportsWhatTheCloseWillBook covers the take-loss path, which
// accepts a negative rather than gating on one. Its number is still the one
// logged and handed on in Params, so a cut reported deeper than the close books
// misstates every take-loss decision downstream of it.
func TestAcceptLossReportsWhatTheCloseWillBook(t *testing.T) {
	// Underwater, so the path logs an accepted loss rather than a gain.
	const lossPrice = 93000.0

	event := minimumNotionalEvent("sellLoss", lossPrice)

	_, dust, _ := quantities.SimulatedClose(event)
	if dust <= 0 {
		t.Fatalf("expected the lot-size floor to cut a remainder, got %v", dust)
	}

	got, err := actions.AcceptLoss(minimumNotionalEvent("sellLoss", lossPrice))
	if err != nil {
		t.Fatalf("AcceptLoss returned error: %v", err)
	}

	withoutDust := closeNetProfitWithoutDust(event, profit.SimulatedClosePrice(event.Trade))
	if got.Trade.Profit <= withoutDust {
		t.Errorf("accepted loss %v drops the %v dust the close keeps (%v without it)", got.Trade.Profit, dust, withoutDust)
	}

	// Both actions simulate the same close; the only difference is that one
	// accepts a negative. They must not disagree on the number.
	gated, _ := actions.HasProfit(minimumNotionalEvent("sellLoss", lossPrice))
	if math.Abs(got.Trade.Profit-gated.Trade.Profit) > 1e-9 {
		t.Errorf("AcceptLoss reported %v where HasProfit reported %v", got.Trade.Profit, gated.Trade.Profit)
	}
}

// TestParentTradeHasProfitCountsTheCloseDust covers the impasse exit, which
// refuses the parent on a negative total. With no children configured the
// parent's own close is the whole sum, so the remainder decides it.
func TestParentTradeHasProfitCountsTheCloseDust(t *testing.T) {
	// This path prices the close at the position price, with no tolerance
	// haircut, so it clears earlier than the take-profit gate does.
	const clearingPrice = 97000.0
	const refusingPrice = 94000.0

	event := scenarioBuildEvent(minimumNotionalEvent("impasse", clearingPrice).Trade, "USDC", "1000")
	if _, err := actions.ParentTradeHasProfit(event); err != nil {
		t.Fatalf("refused a parent close that books above zero: %v", err)
	}

	refused := scenarioBuildEvent(minimumNotionalEvent("impasse", refusingPrice).Trade, "USDC", "1000")
	if _, err := actions.ParentTradeHasProfit(refused); err == nil {
		t.Error("cleared a parent close that books below zero")
	}
}
