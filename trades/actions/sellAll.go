package actions

import (
	"github.com/giovani-sirbu/mercury/events"
)

func SellAll(event events.Events) (events.Events, error) {
	for _, childrenTrade := range event.ChildrenTrades {
		childrenTrade.PreventNewTrade = true
		eventsNames := []string{"sell", "updateTrade"}
		if len(childrenTrade.History) == 0 {
			childrenTrade.Status = "closed"
			eventsNames = []string{"updateTrade"}
		}
		newEvent := events.Events{
			Trade:       childrenTrade,
			Broker:      event.Broker,
			Events:      event.Events,
			Exchange:    event.Exchange,
			EventsNames: eventsNames,
		}
		err := newEvent.Run()
		if err != nil {
			return events.Events{}, err
		}
	}

	event.Trade.PositionType = "sellParent"

	newEvent, err := UpdateTrade(event)

	return newEvent, err
}
