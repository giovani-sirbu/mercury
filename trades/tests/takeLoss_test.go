package tests

import (
	"fmt"
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

func TestHasFunds_EntersTakeLossWhenBudgetInsufficient(t *testing.T) {
	trade := MakeTrade("DOT/USDT", 4.0, false, takeLossHistory())
	trade.PositionType = "buy"
	trade.Strategy.Params.TakeLoss = true

	// next buy needs 20 * 2 (multiplier) * 4.0 = 160 USDT, wallet only has 10
	event := MakeEvent(trade, "USDT", "10", []string{"hasFunds"})

	newEvent, err := actions.HasFunds(event)

	AssertError(t, err)

	if newEvent.Trade.PositionType != "takeLoss" {
		t.Errorf("expected position takeLoss, got %s", newEvent.Trade.PositionType)
	}
	if newEvent.Trade.Status == aggragates.Blocked {
		t.Error("trade must stay unblocked in take loss mode")
	}
	if strings.Contains(err.Error(), "Insufficient funds") {
		t.Error("take loss message must not contain \"Insufficient funds\" or SaveError blocks the trade")
	}
}

func TestHasFunds_TakeLossWinsOverImpasse(t *testing.T) {
	trade := MakeTrade("DOT/USDT", 4.0, false, takeLossHistory())
	trade.PositionType = "buy"
	trade.Strategy.Params.TakeLoss = true
	trade.Strategy.Params.Impasse = true

	event := MakeEvent(trade, "USDT", "10", []string{"hasFunds"})

	newEvent, err := actions.HasFunds(event)

	AssertError(t, err)

	if newEvent.Trade.PositionType != "takeLoss" {
		t.Errorf("expected takeLoss to win over impasse, got %s", newEvent.Trade.PositionType)
	}
}

func TestHasFunds_SkipsTakeLossForChildrenTrades(t *testing.T) {
	trade := MakeTrade("DOT/USDT", 4.0, false, takeLossHistory())
	trade.PositionType = "buy"
	trade.ParentID = 1
	trade.Strategy.Params.TakeLoss = true

	event := MakeEvent(trade, "USDT", "10", []string{"hasFunds"})

	newEvent, err := actions.HasFunds(event)

	AssertError(t, err)

	if newEvent.Trade.PositionType == "takeLoss" {
		t.Error("children trades must not enter take loss mode")
	}
}

func TestAcceptLoss_AllowsNegativeProfit(t *testing.T) {
	// bought at 5.0 and 4.5, selling at 4.0 is a guaranteed loss
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

func TestSaveError_PreservesTakeLossPositionAndActiveStatus(t *testing.T) {
	trade := MakeTrade("DOT/USDT", 4.0, false, takeLossHistory())
	trade.PositionType = "takeLoss"

	event := MakeEvent(trade, "USDT", "10", nil)
	event.Params.OldPosition = "buy"
	event.Params.OldPositionPrice = 4.2

	newEvent, err := actions.SaveError(event, fmt.Errorf("Take loss mode activated: budget too low"))

	AssertError(t, err)

	if newEvent.Trade.PositionType != "takeLoss" {
		t.Errorf("expected takeLoss position to be preserved, got %s", newEvent.Trade.PositionType)
	}
	if newEvent.Trade.Status == aggragates.Blocked {
		t.Error("take loss transition must not block the trade")
	}
}
