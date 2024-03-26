package events

import (
	"encoding/json"
	"github.com/giovani-sirbu/mercury/exchange"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

type (
	Events struct {
		Storage     interface{}
		Broker      interface{}
		Exchange    exchange.Exchange
		Trade       aggragates.Trade
		EventsNames []string
		Events      map[string]func([]byte)
	}
	EventPayload struct {
		Storage     interface{}
		Broker      interface{}
		Exchange    exchange.Exchange
		Trade       aggragates.Trade
		EventsNames []string
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
	eventInBytes, _ := json.Marshal(EventPayload{e.Storage, e.Broker, e.Exchange, e.Trade, e.EventsNames})

	e.Events[e.EventsNames[0]](eventInBytes)
	e.Next()
}

// Add Function to register a new event or replace a default one
func (e Events) Add(event string, action func([]byte)) Events {
	var newEvent = make(map[string]func([]byte))
	for key, value := range e.Events {
		newEvent[key] = value
	}
	newEvent[event] = action
	e.Events = newEvent
	return e
}
