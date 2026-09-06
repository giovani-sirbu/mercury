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

		// Timestamp is the tick time as a Unix value (seconds or millis).
		// Zero is inert: mercury does not advance the in-process 5m print
		// bucket from this field. Engines that already stamp Trade.UpdatedAt
		// (sisyphus backtests) do not need to set it.
		Timestamp int64

		// FiveMinOHLC is an optional injected 5m print bucket for this tick.
		// Zero Last is inert; ShouldHold then synthesizes from Timestamp /
		// Trade.UpdatedAt and the current print when those are present.
		FiveMinOHLC FiveMinOHLC
	}

	// FiveMinOHLC is a 5-minute print bar synthesized from ticks the engine
	// already has. Last==0 means unset.
	FiveMinOHLC struct {
		Open float64
		High float64
		Low  float64
		Last float64
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
		// A hold is a decision, not a failure. It already wrote its own INFO
		// entry on the trade, so logging it here only duplicates it once per
		// tick — and, more importantly, the backoff below must not run for it.
		// LockTradeWithBackOff exists to stop a trade whose chain keeps
		// erroring from being retried every tick; extending the lock for a
		// hold instead throttled a held trade to one evaluation per minute
		// (lockTradeBackoff.go: 1s doubling to a 60s ceiling), so the gate
		// that held it could not re-examine it until the lock expired.
		if errors.Is(err, ErrTradeHeld) {
			return err
		}
		e.LockTradeWithBackOff()
		// A failure the trade already carries (SaveError wrote it, this is a
		// repeat) or the same failure this backoff episode already logged is
		// not logged again: the tick that runs into it only has to keep the
		// backoff. At tick rate the same refusal otherwise repeats word for
		// word, thousands of lines a minute, in the backtest and live alike.
		if errors.Is(err, ErrRepeatedFailure) || e.repeatsLastFailure(err) {
			return err
		}
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
	// helpers.SplitSymbol, not strings.Split(...)[1]: a symbol without exactly
	// one slash indexed out of range and panicked the whole trade goroutine —
	// on the ERROR path, so a malformed pair turned any action error into a
	// process-level panic instead of a logged failure.
	_, assetSymbol := helpers.SplitSymbol(e.Trade.Symbol)

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
		helpers.FindUsedAmount(e.Params.InverseUsedAmount, assetSymbol),
	)

	log.Error(msg, "Run events", "")
	return err
}
