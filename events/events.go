package events

import (
	"fmt"
	"github.com/adshao/go-binance/v2/common"
	"github.com/giovani-sirbu/mercury/exchange"
	"github.com/giovani-sirbu/mercury/log"
	"github.com/giovani-sirbu/mercury/messagebroker"
	"github.com/giovani-sirbu/mercury/storage/memory"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"strings"
)

type (
	Events struct {
		// Storage is the shared cache client. Pointer semantics are required because
		// *memory.Memory owns singleton state (sync.Once + reused Redis client).
		Storage *memory.Memory

		// WsPrices is an in-process snapshot of symbol prices, typically populated
		// by hermes before running an event pipeline. Lookups here avoid a Redis
		// round-trip on the trade decision hot path. Nil is valid — callers fall
		// back to Storage and then to the exchange API.
		WsPrices map[string]float64

		Broker         messagebroker.BrokerMethods
		Exchange       exchange.Exchange
		Trade          aggragates.Trades
		ChildrenTrades []aggragates.Trades
		EventsNames    []string
		Params         aggragates.Params
		Events         map[string]func(Events) (Events, error)

		// CorrelationID is the per-action-chain correlation id. Hermes's
		// ManageTrade / ManageFuturesTrade populate it before calling Run()
		// so every action in the chain — including the update-trade and
		// create-children-trades producers — tags the resulting message and
		// log lines with the same id.
		CorrelationID string
	}
)

// Next Function to run the next event if we have multiple events
func (e Events) Next() error {
	if len(e.EventsNames) <= 1 {
		// Safely clean up backoffTries. The existence check, len() and delete
		// must all run under rwLocker: a periodic sweeper (lockTradeBackoff.go)
		// and LockTradeWithBackOff write this package-global map from sibling
		// trade goroutines, so an unlocked read here races a concurrent write
		// and triggers a fatal "concurrent map read and map write".
		rwLocker.Lock()
		if _, exists := backoffTries[e.Trade.ID]; exists {
			log.Debug("backoffTries[before]: ", len(backoffTries), e.Trade.ID)
			delete(backoffTries, e.Trade.ID)
			log.Debug("backoffTries[after]: ", len(backoffTries), e.Trade.ID)
		}
		rwLocker.Unlock()

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

	var errorMessage string

	if err != nil {
		// Handle nil pointer of *APIError safely
		if apiErr, ok := err.(*common.APIError); ok && apiErr == nil {
			errorMessage = "<nil APIError>"
		} else {
			errorMessage = err.Error()
		}
	} else {
		errorMessage = "<nil error>"
	}

	msg := fmt.Sprintf(
		"%s | User ID: #%d | Trade Info: (ID: #%d, Position Type: %s, Position Price: %f, Impasse: %t, Profit: %f, Quantity: %f, Dust: %f, Depths: %d, Inverse used: %f)",
		errorMessage,
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
