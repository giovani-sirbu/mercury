package actions

import (
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func SaveError(event events.Events, err error) (events.Events, error) {
	// prevent duplicate logs
	message := err.Error()
	lastLog := event.Trade.Logs[len(event.Trade.Logs)-1]
	if len(event.Trade.Logs) > 0 && lastLog.Message == message {
		return event, nil
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
		TradeID:    event.Trade.ID,
		Quantity:   event.Params.Quantity,
	})

	newEvent, _ := UpdateTrade(event)

	return newEvent, err
}
