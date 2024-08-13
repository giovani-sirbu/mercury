package events

import (
	"github.com/giovani-sirbu/mercury/exchange"
	"github.com/giovani-sirbu/mercury/log"
	"github.com/giovani-sirbu/mercury/messagebroker"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

type (
	Events struct {
		Storage               interface{}
		Broker                messagebroker.MessageBroker
		Exchange              exchange.Exchange
		Trade                 aggragates.Trades
		ChildrenTrades        []aggragates.Trades
		EventsNames           []string
		TradeSettings         aggragates.TradeSettings
		ChildrenTradeSettings []aggragates.TradeSettings
		Params                aggragates.Params
		Events                map[string]func(Events) (Events, error)
	}
)

// Run Function to run events
func (e Events) Run() error {
	if len(e.EventsNames) == 0 {
		return nil
	}
	if e.Events[e.EventsNames[0]] == nil {
		return nil
	}

	for _, eventName := range e.EventsNames {
		log.Debug("In progress action", eventName)
		_, err := e.Events[eventName](e)
		if err != nil {
			log.Error(err.Error(), "Run events", "")
			return err
		}
		continue
	}

	log.Debug("Finish all event actions", e.EventsNames)

	return nil
}

// Add Function to register a new event or replace a default one
func (e Events) Add(event string, action func(Events) (Events, error)) Events {
	var newEvent = make(map[string]func(Events) (Events, error))
	for key, value := range e.Events {
		newEvent[key] = value
	}
	newEvent[event] = action
	e.Events = newEvent
	return e
}
