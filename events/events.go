package events

import (
	"errors"
	"fmt"
	"github.com/adshao/go-binance/v2/common"
	"github.com/giovani-sirbu/mercury/exchange"
	"github.com/giovani-sirbu/mercury/helpers"
	"github.com/giovani-sirbu/mercury/log"
	"github.com/giovani-sirbu/mercury/messagebroker"
	"github.com/giovani-sirbu/mercury/storage/memory"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

type (
	Events struct {
		// Storage is the shared cache client; a pointer, since Memory owns singleton state.
		Storage *memory.Memory

		// WsPrices is the engine's in-process price snapshot; nil falls back to the exchange API.
		WsPrices map[string]float64

		Broker         messagebroker.BrokerMethods
		Exchange       exchange.Exchange
		Trade          aggragates.Trades
		ChildrenTrades []aggragates.Trades
		EventsNames    []string
		Params         aggragates.Params
		Events         map[string]func(Events) (Events, error)

		// CorrelationID is the per-chain correlation id every message and log line of the chain carries.
		CorrelationID string

		// Timestamp is the tick time as a Unix value, read through TickTime;
		// zero falls back to Trade.UpdatedAt.
		Timestamp int64
	}
)

// Next runs the next action of the chain, or clears the trade's backoff when the chain is done.
func (e Events) Next() error {
	if len(e.EventsNames) <= 1 {
		// Under rwLocker: the sweeper and LockTradeWithBackOff write this map from other goroutines.
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

// Run runs the action chain in order: each action hands the event on, a hold
// (ErrTradeHeld) stops it quietly, and any other error backs the trade off
// and is logged once per backoff episode.
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
		// A hold is a decision: no log, no backoff, so the gate re-examines the trade next tick.
		if errors.Is(err, ErrTradeHeld) {
			return err
		}
		e.LockTradeWithBackOff()
		// A failure already on the trade, or already logged this episode, keeps the backoff only.
		if errors.Is(err, ErrRepeatedFailure) || e.repeatsLastFailure(err) {
			return err
		}
		return e.logEventError(err)
	}

	return newEvent.Next()
}

// Add registers an action, or replaces a default one, on a copy of the map.
func (e Events) Add(event string, action func(Events) (Events, error)) Events {
	var newEvent = make(map[string]func(Events) (Events, error))
	for key, value := range e.Events {
		newEvent[key] = value
	}
	newEvent[event] = action
	e.Events = newEvent
	return e
}

// logEventError logs an action error with the trade's context and returns it.
func (e Events) logEventError(err error) error {
	// SplitSymbol never panics on a malformed pair.
	_, assetSymbol := helpers.SplitSymbol(e.Trade.Symbol)

	var errorMessage string

	if err != nil {
		// A nil *APIError is a non-nil error.
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
		helpers.FindUsedAmount(e.Params.InverseUsedAmount, assetSymbol),
	)

	log.Error(msg, "Run events", "")
	return err
}
