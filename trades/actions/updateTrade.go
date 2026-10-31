package actions

import (
	"encoding/json"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/tradelog"
)

// UpdateTrade appends the transition's info row (tradelog.TransitionLog) and publishes the trade on
// update-trade, which agora persists before releasing the trade lock.
func UpdateTrade(event events.Events) (events.Events, error) {
	if !event.Params.PreventInfoLog {
		event.Trade.Logs = append(event.Trade.Logs, tradelog.TransitionLog(event))
	}
	tradeInBytes, _ := json.Marshal(event.Trade)
	topic := "update-trade"
	// The Produce error is deliberately not propagated: returning it would
	// back off the lock and let the next tick run the chain, Buy and Sell
	// included, a second time on the exchange, and those are not idempotent.
	// A broker outage after a fill leaves the trade behind its lock until the
	// next chain publishes or an operator reconciles it; the Producer logs the
	// failure. An idempotent chain would let the error propagate.
	_ = event.Broker.ProduceWithCorrelation(topic, tradeInBytes, event.CorrelationID, event.Broker.Producer)
	return event, nil
}
