package actions_test

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/actions"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func TestRegulatePriceChange(t *testing.T) {
	t.Run("Normal_WithinThreshold", func(t *testing.T) {
		// lastBuyPrice=100, percentage=2 -> threshold = 100 - 100*0.02 = 98
		// positionPrice=97 < 98 -> OK
		trade := MakeTrade("ATOM/USDT", 97, false, []aggragates.TradesHistory{
			{Quantity: 10, Price: 100, Type: "BUY"},
		})
		event := MakeEvent(trade, "USDT", "1000", []string{"regulatePriceChange"})
		_, err := actions.RegulatePriceChange(event)
		AssertNoError(t, err)
	})

	t.Run("Normal_ExceedsThreshold", func(t *testing.T) {
		// lastBuyPrice=100, percentage=2 -> threshold = 98
		// positionPrice=99 > 98 -> error
		trade := MakeTrade("ATOM/USDT", 99, false, []aggragates.TradesHistory{
			{Quantity: 10, Price: 100, Type: "BUY"},
		})
		event := MakeEvent(trade, "USDT", "1000", []string{"regulatePriceChange"})
		_, err := actions.RegulatePriceChange(event)
		AssertError(t, err)
	})

	t.Run("Inverse_WithinThreshold", func(t *testing.T) {
		// lastSellPrice=100, percentage=2 -> threshold = 100 + 100*0.02 = 102
		// positionPrice=103 > 102 -> OK (inverse: error when price < threshold)
		trade := MakeTrade("ATOM/USDT", 103, true, []aggragates.TradesHistory{
			{Quantity: 10, Price: 100, Type: "SELL"},
		})
		event := MakeEvent(trade, "USDT", "1000", []string{"regulatePriceChange"})
		_, err := actions.RegulatePriceChange(event)
		AssertNoError(t, err)
	})

	t.Run("Inverse_BelowThreshold", func(t *testing.T) {
		// lastSellPrice=100, percentage=2 -> threshold = 102
		// positionPrice=101 < 102 -> error
		trade := MakeTrade("ATOM/USDT", 101, true, []aggragates.TradesHistory{
			{Quantity: 10, Price: 100, Type: "SELL"},
		})
		event := MakeEvent(trade, "USDT", "1000", []string{"regulatePriceChange"})
		_, err := actions.RegulatePriceChange(event)
		AssertError(t, err)
	})
}
