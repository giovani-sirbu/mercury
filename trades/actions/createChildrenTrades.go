package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/giovani-sirbu/mercury/events"
)

func CreateChildrenTrades(event events.Events) (events.Events, error) {
	if len(event.ChildrenTrades) > 0 {
		return event, nil
	}
	tradeInBytes, _ := json.Marshal(event.Trade)
	topic := "create-children-trades"
	event.Broker.Producer(topic, context.Background(), nil, tradeInBytes)

	return events.Events{}, fmt.Errorf("childrens not created yet")
}
