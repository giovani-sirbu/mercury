package actions

import (
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func SaveError(event events.Events, err error) (events.Events, error) {
	// prevent duplicate logs
	message := err.Error()
	if len(event.Trade.Logs) > 0 && event.Trade.Logs[len(event.Trade.Logs)-1].Message == message {
		return event, err
	}

	// Reset price and position to allow only the error update
	price := event.Trade.PositionPrice
	event.Trade.PositionType = event.Params.OldPosition
	event.Trade.PositionPrice = event.Params.OldPositionPrice
	event.Params.PreventInfoLog = true

	event.Trade.Logs = append(event.Trade.Logs, aggragates.TradesLogs{
		Percentage: event.Params.Percentage,
		Message:    message,
		Type:       aggragates.LOG_WARNING,
		Price:      price,
		Quantity:   event.Params.Quantity,
		TradeID:    event.Trade.ID,
	})

	newEvent, _ := event.Events["updateTrade"](event)

	return newEvent, err
}
