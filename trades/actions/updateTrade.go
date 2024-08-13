package actions

import (
	"encoding/json"
	"fmt"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func UpdateTrade(event events.Events) (events.Events, error) {
	// If no error occurs save only info logs
	if !event.Params.PreventInfoLog {
		message := fmt.Sprintf("Updated position to %s from %s", event.Trade.PositionType, event.Params.OldPosition)
		event.Trade.Logs = append(event.Trade.Logs, aggragates.TradesLogs{
			Percentage: event.Params.Percentage,
			Message:    message,
			Type:       aggragates.LOG_INFO,
			Price:      event.Trade.PositionPrice,
			TradeID:    event.Trade.ID,
		})
	}
	tradeInBytes, _ := json.Marshal(event.Trade)
	topic := "update-trade"
	event.Broker.Produce(topic, nil, tradeInBytes, event.Broker.Producer)
	return event, nil
}
