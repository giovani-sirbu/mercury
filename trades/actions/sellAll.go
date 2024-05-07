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
			Exchange:      event.Exchange,
			EventsNames:   []string{"sell", "updateTrade"},
			TradeSettings: event.ChildrenTradeSettings[index],
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
