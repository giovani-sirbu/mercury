package events

import (
	"github.com/giovani-sirbu/mercury/exchange"
	"github.com/giovani-sirbu/mercury/log"
	"github.com/giovani-sirbu/mercury/messagebroker"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

type (
	Events struct {
		Storage     interface{}
		Broker      messagebroker.MessageBroker
		Exchange    exchange.Exchange
		Trade       aggragates.Trade
		EventsNames []string
		Events      map[string]func(Events) (Events, error)
	}
)

// Next Function to run the next event if we have multiple events
func (e Events) Next() {
	if len(e.EventsNames) == 1 {
		return
	}
	e.EventsNames = e.EventsNames[1:]
	e.Run()
}

// Run Function to run events
func (e Events) Run() {
	if len(e.EventsNames) == 0 {
		return
	}
	if e.Events[e.EventsNames[0]] == nil {
		return
	}

	newEvent, err := e.Events[e.EventsNames[0]](e)
	if err != nil {
		log.Error(err.Error(), "Run events", "")
		return
	}
	newEvent.Next()
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
