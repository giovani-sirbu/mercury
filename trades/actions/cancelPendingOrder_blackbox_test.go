package actions_test

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/actions"
)

func TestCancelPendingOrder(t *testing.T) {
	t.Run("NoPendingOrder", func(t *testing.T) {
		trade := MakeTrade("ATOM/USDT", 5, false, nil)
		trade.PendingOrder = 0
		event := MakeEvent(trade, "USDT", "1000", []string{"cancelPendingOrder"})

		result, err := actions.CancelPendingOrder(event)
		AssertNoError(t, err)
		if result.Trade.PendingOrder != 0 {
			t.Errorf("PendingOrder should remain 0, got %d", result.Trade.PendingOrder)
		}
	})

	t.Run("HasPendingOrder", func(t *testing.T) {
		trade := MakeTrade("ATOM/USDT", 5, false, nil)
		trade.PendingOrder = 12345
		event := MakeEvent(trade, "USDT", "1000", []string{"cancelPendingOrder"})

		result, err := actions.CancelPendingOrder(event)
		AssertNoError(t, err)
		if result.Trade.PendingOrder != 0 {
			t.Errorf("PendingOrder should be 0 after cancel, got %d", result.Trade.PendingOrder)
		}
	})
}
