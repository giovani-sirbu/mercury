package actions

import (
	"encoding/json"
	"fmt"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/helpers"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"strings"
)

func UpdateTrade(event events.Events) (events.Events, error) {
	message := fmt.Sprintf("%s_TO_%s", event.Params.OldPosition, event.Trade.PositionType)

	// prevent duplicate logs
	if len(event.Trade.Logs) > 0 {
		lastError := event.Trade.Logs[len(event.Trade.Logs)-1].Message
		if helpers.RemoveNumbersFromString(lastError) == helpers.RemoveNumbersFromString(message) {
			return event, nil
		}
	}

	// If no error occurs save only info logs
	if !event.Params.PreventInfoLog {
		event.Trade.Logs = append(event.Trade.Logs, aggragates.TradesLogs{
			Percentage: event.Params.Percentage,
			Message:    strings.ToUpper(message),
			Type:       aggragates.LOG_INFO,
			Price:      event.Trade.PositionPrice,
			Quantity:   event.Params.Quantity,
			TradeID:    event.Trade.ID,
		})
	}
	tradeInBytes, _ := json.Marshal(event.Trade)
	topic := "update-trade"
	// DELIBERATE: do NOT propagate the Produce error upward.
	//
	// Returning err here would make events.Run apply LockTradeWithBackOff,
	// which extends the lock by 1–60s. After the lock expires the next WS
	// tick re-enters ManageTrade, finds the trade state still "stale" in
	// Redis (agora never received the update), the strategy decides the
	// same transition again, and the entire action chain — including
	// Buy()/Sell() — runs a SECOND time on Binance. Buy/Sell are not
	// idempotent on the exchange (each call produces a new order; there is
	// no client_order_id wiring through the mercury Binance adaptor), so
	// the cure would be worse than the disease.
	//
	// Current degraded mode on broker outage after Buy: trade is "stuck"
	// because the TryLockTrade 24h lock blocks any re-entry. Recovery
	// requires (a) broker comes back AND (b) the next chain attempt
	// publishes successfully — or (c) manual reconciliation by an
	// operator who reads Binance's order history and re-inserts the
	// missing update-trade row into the outbox.
	//
	// Produce already logs the failure at the Producer level (see
	// produceWithCorrelation), so the outage is visible in logs even
	// though we don't propagate.
	//
	// TODO: idempotent action chain. Persist a per-trade "buy-attempted"
	// flag in Redis keyed by event.Trade.ID + transition; on retry, the
	// Buy() action reads the flag and (if set) calls GetOrder against
	// Binance to confirm + skip duplicate execution. Once that exists,
	// this comment block can go and we can safely propagate the error.
	_ = event.Broker.ProduceWithCorrelation(topic, tradeInBytes, event.CorrelationID, event.Broker.Producer)
	return event, nil
}
