package actions

import (
	"fmt"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func CloseOrKeepALiveTrade(event events.Events) (events.Events, error) {
	// If AI action is not in same direction position, close it
	if (event.Params.AIIndicators.AIAction == "LONG" && event.Trade.PositionType == "sell") ||
		(event.Params.AIIndicators.AIAction == "SHORT" && event.Trade.PositionType == "buy") {
		newEvent, newError := CloseFuturesTrade(event)
		return newEvent, newError
	}
	// Init futures client
	client, clientError := event.Exchange.FuturesClient()
	if clientError != nil {
		return events.Events{}, clientError
	}

	stopLossOrder, stopOrderErr := client.GetOrderById(event.Trade.Symbol, event.Trade.PendingOrder)

	if stopOrderErr != nil {
		return events.Events{}, stopOrderErr
	}

	if stopLossOrder.Status == "FILLED" {
		pnl, _ := GetLatestIncome(event)

		event.Trade.Status = aggragates.Closed
		event.Trade.Profit = pnl
		event.Trade.USDProfit = pnl
		return event, nil
	}

	return events.Events{}, fmt.Errorf("position was kept alive")
}
