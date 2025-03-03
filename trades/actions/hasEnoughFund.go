package actions

import (
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func hasNegativeHistoryAmount(history []aggragates.TradesHistory) bool {
	for _, tradesHistory := range history {
		if tradesHistory.Quantity < 0 {
			return true
		}
	}
	return false
}

func HasEnoughFunds(event events.Events) (events.Events, error) {
	negativeAmount := hasNegativeHistoryAmount(event.Trade.History)
	if negativeAmount {
		remainedQuantity, neededQuantity, _, _ := GetFundsQuantities(event)
		if remainedQuantity < neededQuantity {
			event.Trade.History = append(event.Trade.History, aggragates.TradesHistory{Quantity: (remainedQuantity - neededQuantity) + (remainedQuantity-neededQuantity)*0.000000000001, Type: "ADJUST", Price: 0.000000000001})
			newEvent, err := event.Events["updateTrade"](event)
			return newEvent, err
		}
	}
	return events.Events{}, nil
}
