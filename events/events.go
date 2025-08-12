package events

import (
	"fmt"
	"github.com/giovani-sirbu/mercury/exchange"
	"github.com/giovani-sirbu/mercury/log"
	"github.com/giovani-sirbu/mercury/messagebroker"
	"github.com/giovani-sirbu/mercury/storage/memory"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"strings"
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
	if len(e.EventsNames) <= 1 {
		// Safely clean up backoffTries
		_, exists := backoffTries[e.Trade.ID]
		if exists {
			log.Debug("backoffTries[before]: ", len(backoffTries), e.Trade.ID)

			rwLocker.Lock()
			defer rwLocker.Unlock()
			delete(backoffTries, e.Trade.ID)

			log.Debug("backoffTries[after]: ", len(backoffTries), e.Trade.ID)
		}

		return nil
	}

	e.EventsNames = e.EventsNames[1:]
	return e.Run()
}

// Run Function to run events
func (e Events) Run() error {
	if len(e.EventsNames) == 0 {
		return nil
	}

	currentEvent := e.EventsNames[0]
	eventFunc, exists := e.Events[currentEvent]
	if !exists || eventFunc == nil {
		return nil
	}

	newEvent, err := eventFunc(e)

	if err != nil {
		e.LockTradeWithBackOff()
		return e.logEventError(err)
	}

	return newEvent.Next()
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

// logEventError formats and logs event execution errors
func (e Events) logEventError(err error) error {
	pairSymbols := strings.Split(e.Trade.Symbol, "/")
	assetSymbol := pairSymbols[1]

	msg := fmt.Sprintf(
		"%s | User ID: #%d | Trade Info: (ID: #%d, Position Type: %s, Position Price: %f, Impasse: %t, Profit: %f, Quantity: %f, Dust: %f, Depths: %d, Inverse used: %f)",
		err.Error(),
		e.Trade.UserID,
		e.Trade.ID,
		e.Trade.PositionType,
		e.Trade.PositionPrice,
		e.Trade.Inverse,
		e.Params.Profit,
		e.Params.Quantity,
		e.Trade.Dust,
		len(e.Trade.History),
		aggragates.FindUsedAmount(e.Params.InverseUsedAmount, assetSymbol),
	)

	log.Error(msg, "Run events", "")
	return err
}
