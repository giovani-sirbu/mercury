package events

import (
	"github.com/giovani-sirbu/mercury/exchange"
	"github.com/giovani-sirbu/mercury/log"
	"github.com/giovani-sirbu/mercury/messagebroker"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

type (
	Events struct {
		Storage       interface{}
		Broker        messagebroker.MessageBroker
		Exchange      exchange.Exchange
		Trade         aggragates.Trade
		EventsNames   []string
		TradeSettings aggragates.TradeSettings
		Events        map[string]func(Events) (Events, error)
	}
)

// Next Function to run the next event if we have multiple events
func (e Events) Next() error {
	if len(e.EventsNames) == 1 {
		return nil
	}
	e.EventsNames = e.EventsNames[1:]
	err := e.Run()
	return err
}

// Run Function to run events
func (e Events) Run() error {
	if len(e.EventsNames) == 0 {
		return nil
	}
	if e.Events[e.EventsNames[0]] == nil {
		return nil
	}

	newEvent, err := e.Events[e.EventsNames[0]](e)
	if err != nil {
		log.Error(err.Error(), "Run events", "")
		return err
	}
	err = newEvent.Next()
	return err
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
