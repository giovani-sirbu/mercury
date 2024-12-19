package actions

import (
	"encoding/json"
	"fmt"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"strings"
)

func UpdateTrade(event events.Events) (events.Events, error) {
	message := fmt.Sprintf("%s_TO_%s", event.Params.OldPosition, event.Trade.PositionType)

	// prevent duplicate logs
	if len(event.Trade.Logs) > 0 {
		lastError := event.Trade.Logs[len(event.Trade.Logs)-1].Message
		if RemoveNumbersFromString(lastError) == RemoveNumbersFromString(message) {
			return event, nil
		}
	}

	// If no error occurs save only info logs
	if !event.Params.PreventInfoLog {
		event.Trade.Logs = append(event.Trade.Logs, aggragates.TradesLogs{
			Percentage: event.Params.Percentage,
			Message:    strings.ToUpper(message),
			Type:       aggragates.LOG_INFO,
			Price:      event.Trade.PositionPrice,
			Quantity:   event.Params.Quantity,
			TradeID:    event.Trade.ID,
		})
	}
	tradeInBytes, _ := json.Marshal(event.Trade)
	topic := "update-trade"
	event.Broker.Produce(topic, nil, tradeInBytes, event.Broker.Producer)
	return event, nil
}
