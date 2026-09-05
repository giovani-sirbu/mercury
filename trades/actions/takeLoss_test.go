package actions_test

import (
	"fmt"
	"github.com/giovani-sirbu/mercury/trades/tradelog"
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/trades/actions"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func takeLossHistory() []aggragates.TradesHistory {
	return []aggragates.TradesHistory{
		{Quantity: 10, Price: 5.0, Type: "BUY", Fees: []aggragates.TradesFees{{Asset: "DOT", Fee: 0.01}}},
		{Quantity: 20, Price: 4.5, Type: "BUY", Fees: []aggragates.TradesFees{{Asset: "DOT", Fee: 0.02}}},
	}
}

func TestHasFunds_BlocksWhenBudgetInsufficient(t *testing.T) {
	trade := MakeTrade("DOT/USDT", 4.0, false, takeLossHistory())
	trade.PositionType = "buy"

	event := MakeEvent(trade, "USDT", "10", []string{"hasFunds"})

	newEvent, err := actions.HasFunds(event)

	AssertError(t, err)

	if newEvent.Trade.PositionType == "takeLoss" {
		t.Error("hasFunds must not enter takeLoss; that mode was removed")
	}
	if newEvent.Trade.Status != aggragates.Blocked {
		t.Error("insufficient funds without impasse must block the trade")
	}
	if !strings.Contains(err.Error(), "Insufficient funds") {
		t.Errorf("expected insufficient-funds error, got %v", err)
	}
}

// impasseHistory is a position big enough for the impasse branch to reach its
// own gate. The two rows of takeLossHistory are worth ~120 USDT, and
// CalculateInitialBid refuses to plan a ladder that small (it needs ~363), so
// a fixture that size never entered impasse at all: the old test asserted only
// the absence of takeLoss and stayed green with the whole impasse block
// deleted.
func impasseHistory() []aggragates.TradesHistory {
	return []aggragates.TradesHistory{
		{Quantity: 100, Price: 5.0, Type: "BUY", Fees: []aggragates.TradesFees{{Asset: "DOT", Fee: 0.01}}},
		{Quantity: 200, Price: 4.5, Type: "BUY", Fees: []aggragates.TradesFees{{Asset: "DOT", Fee: 0.02}}},
	}
}

func TestHasFunds_ImpasseWinsWhenEnabled(t *testing.T) {
	trade := MakeTrade("DOT/USDT", 4.0, false, impasseHistory())
	trade.PositionType = "buy"
	trade.Strategy.Params.Impasse = true

	event := MakeEvent(trade, "USDT", "10", []string{"hasFunds"})

	newEvent, err := actions.HasFunds(event)

	AssertError(t, err)

	if newEvent.Trade.PositionType != "impasse" {
		t.Errorf("a funded position with impasse enabled must enter impasse, got %q", newEvent.Trade.PositionType)
	}
}

// Without the flag the same trade is simply blocked.
func TestHasFunds_NoImpasseWithoutTheFlag(t *testing.T) {
	trade := MakeTrade("DOT/USDT", 4.0, false, impasseHistory())
	trade.PositionType = "buy"

	event := MakeEvent(trade, "USDT", "10", []string{"hasFunds"})

	newEvent, err := actions.HasFunds(event)

	AssertError(t, err)

	if newEvent.Trade.PositionType == "impasse" {
		t.Error("impasse must need its flag")
	}
	if newEvent.Trade.Status != aggragates.Blocked {
		t.Error("insufficient funds without impasse must block the trade")
	}
}

// A trade with no fills holds nothing to average down, so a first entry never
// enters impasse however the strategy is configured — otherwise the next tick
// runs createChildrenTrades and sellAll against no position.
//
// No error is asserted here: with an empty history GetFundsQuantities needs
// nothing (it sizes the next depth from the last fill), so the gate admits the
// tick outright and never reaches the impasse branch at all. The guard in
// HasFunds is the second line of defence, for the case where a wallet lost to
// InverseUsedAmount makes the comparison fail on a first entry anyway.
func TestHasFunds_FirstEntryNeverEntersImpasse(t *testing.T) {
	trade := MakeTrade("DOT/USDT", 4.0, false, nil)
	trade.PositionType = "buy"
	trade.Strategy.Params.Impasse = true

	event := MakeEvent(trade, "USDT", "10", []string{"hasFunds"})

	newEvent, _ := actions.HasFunds(event)

	if newEvent.Trade.PositionType == "impasse" {
		t.Error("a first entry has no position to average down and must not enter impasse")
	}
}

func TestHasFunds_ChildrenNeverEnterImpasse(t *testing.T) {
	trade := MakeTrade("DOT/USDT", 4.0, false, impasseHistory())
	trade.PositionType = "buy"
	trade.ParentID = 1
	trade.Strategy.Params.Impasse = true

	event := MakeEvent(trade, "USDT", "10", []string{"hasFunds"})

	newEvent, err := actions.HasFunds(event)

	AssertError(t, err)

	if newEvent.Trade.PositionType == "impasse" {
		t.Error("a child trade must not open children of its own")
	}
	if newEvent.Trade.PositionType == "takeLoss" {
		t.Error("children trades must not enter take loss mode")
	}
}

func TestAcceptLoss_AllowsNegativeProfit(t *testing.T) {
	trade := MakeTrade("DOT/USDT", 4.0, false, takeLossHistory())
	trade.PositionType = "sellLoss"

	event := MakeEvent(trade, "USDT", "10", []string{"acceptLoss"})

	newEvent, err := actions.AcceptLoss(event)

	AssertNoError(t, err)

	if newEvent.Trade.Profit >= 0 {
		t.Errorf("expected negative profit, got %f", newEvent.Trade.Profit)
	}
	if newEvent.Params.Profit != newEvent.Trade.Profit {
		t.Errorf("params profit (%f) must match trade profit (%f)", newEvent.Params.Profit, newEvent.Trade.Profit)
	}
}

func TestSaveError_RewindsLegacyTakeLossPosition(t *testing.T) {
	trade := MakeTrade("DOT/USDT", 4.0, false, takeLossHistory())
	trade.PositionType = "takeLoss"

	event := MakeEvent(trade, "USDT", "10", nil)
	event.Params.OldPosition = "buy"
	event.Params.OldPositionPrice = 4.2

	newEvent, err := tradelog.SaveError(event, fmt.Errorf("Take loss mode activated: budget too low"))

	AssertError(t, err)

	if newEvent.Trade.PositionType != "buy" {
		t.Errorf("legacy takeLoss must rewind to OldPosition, got %s", newEvent.Trade.PositionType)
	}
	if newEvent.Trade.PositionPrice != 4.2 {
		t.Errorf("legacy takeLoss must rewind to OldPositionPrice, got %f", newEvent.Trade.PositionPrice)
	}
}
