package actions

import (
	"context"
	"encoding/json"
	"github.com/giovani-sirbu/mercury/events"
)

func UpdateTrade(event events.Events) (events.Events, error) {
	tradeInBytes, _ := json.Marshal(event.Trade)
	topic := "update-trade"
	event.Broker.Producer(topic, context.Background(), nil, tradeInBytes)
	return event, nil
}
