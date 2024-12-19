package events

import (
	"fmt"
	"github.com/giovani-sirbu/mercury/exchange"
	"github.com/giovani-sirbu/mercury/log"
	"github.com/giovani-sirbu/mercury/messagebroker"
	"github.com/giovani-sirbu/mercury/storage/memory"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

type (
	Events struct {
		Storage        memory.Memory
		Broker         messagebroker.BrokerMethods
		Exchange       exchange.Exchange
		Trade          aggragates.Trades
		ChildrenTrades []aggragates.Trades
		EventsNames    []string
		Params         aggragates.Params
		Events         map[string]func(Events) (Events, error)
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
		e.LockTradeWithBackOff()
		msg := fmt.Sprintf("%s | User ID: #%d | Trade Info: (ID: #%d, Position Type: %s, Position Price: %f, Impasse: %t, Profit: %f, Depths: %d)",
			err.Error(),
			e.Trade.UserID,
			e.Trade.ID,
			e.Trade.PositionType,
			e.Trade.PositionPrice,
			e.Trade.Inverse,
			e.Params.Profit,
			len(e.Trade.History),
		)
		log.Error(msg, "Run events", "")
		return err
	}
	err = newEvent.Next()
	if err != nil {
		e.LockTradeWithBackOff()
	}
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
