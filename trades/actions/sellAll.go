package actions

import (
	"github.com/giovani-sirbu/mercury/events"
)

func SellAll(event events.Events) (events.Events, error) {
	for index, childrenTrade := range event.ChildrenTrades {
		newEvent := events.Events{
			Trade:         childrenTrade,
			Broker:        event.Broker,
			Events:        event.Events,
			EventsNames:   []string{"sell", "updateTrade"},
			TradeSettings: event.ChildrenTradeSettings[index],
		}
		newEvent.Run()
	}

	event.Trade.PositionType = "sellParent"

	newEvent, err := UpdateTrade(event)

	return newEvent, err
}
