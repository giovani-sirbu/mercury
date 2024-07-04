package actions

import (
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func SaveError(event events.Events, err error) (events.Events, error) {
	if event.Trade.Logs[len(event.Trade.Logs)-1].Type == aggragates.LOG_WARNING {
		return event, err
	}
	// Reset price and position to allow only the error update
	price := event.Trade.PositionPrice
	event.Trade.PositionType = event.Params.OldPosition
	event.Trade.PositionPrice = event.Params.OldPositionPrice
	event.Params.PreventInfoLog = true

	event.Trade.Logs = append(event.Trade.Logs, aggragates.TradesLogs{
		Percentage: event.Params.Percentage,
		Message:    err.Error(),
		Type:       aggragates.LOG_WARNING,
		Price:      price,
		TradeID:    event.Trade.ID,
		Quantity:   event.Params.Quantity,
	})

	newEvent, _ := UpdateTrade(event)

	return newEvent, err
}
