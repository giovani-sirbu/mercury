package tradelog

import (
	"fmt"
	"time"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/strategies/gates"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// holdRelogAfter is how long one hold row stands for a hold still in force
// before the same reason is written again, so a long blockade reads as one.
const holdRelogAfter = 24 * time.Hour

// SaveHoldLog records a hold as an INFO row, restores the previous position
// and stops the chain with events.ErrTradeHeld. A hold that names a strategy
// param writes its strategy event beside the row, under the same tick clock.
// The dedupe is on the full message: a repeat of the same reason is written
// again only once the previous row is older than holdRelogAfter, and a
// collapsed hold writes neither row nor event and says so with
// events.ErrHoldNotPersisted, since nothing then releases the trade lock.
func SaveHoldLog(event events.Events, position string, hold gates.Hold) (events.Events, error) {
	message := fmt.Sprintf("Hold %s: %s", position, hold.Reason)
	// Wrapped so the chain stops while the runner keeps quiet: the row is the record.
	err := fmt.Errorf("%w: %s %s", events.ErrTradeHeld, position, hold.Reason)

	now := event.TickTime()
	if holdLoggedWithin(event.Trade.Logs, message, now) {
		// Nothing is written and updateTrade never runs: the caller releases its own lock.
		return event, fmt.Errorf("%w (%w)", err, events.ErrHoldNotPersisted)
	}

	// Reset price and position so only the hold entry is persisted.
	price := event.Trade.PositionPrice
	event.Trade.PositionType = event.Params.OldPosition
	event.Trade.PositionPrice = event.Params.OldPositionPrice
	event.Params.PreventInfoLog = true

	event.Trade.Logs = append(event.Trade.Logs, aggragates.TradesLogs{
		Percentage: event.Params.Percentage,
		Message:    message,
		Type:       aggragates.LOG_INFO,
		Price:      price,
		Quantity:   event.Params.Quantity,
		TradeID:    event.Trade.ID,
		// The tick clock, so the age test works on replayed tape.
		CreatedAt: now,
		UpdatedAt: now,
	})
	if hold.Param != "" {
		event.Trade.StrategyEvents = append(
			event.Trade.StrategyEvents,
			aggragates.NewStrategyEvent(event.Trade.ID, hold.Param, hold.Gate, hold.Data, now),
		)
	}

	newEvent, _ := event.Events["updateTrade"](event)

	return newEvent, err
}

// holdLoggedWithin reports whether this exact message already stands in the
// log within the last holdRelogAfter: the scan runs backwards and stops at the
// first row old enough to be written again, so alternating reasons collapse
// to one row each per window. Without a clock it is the plain "same message" rule.
func holdLoggedWithin(logs []aggragates.TradesLogs, message string, now time.Time) bool {
	for i := len(logs) - 1; i >= 0; i-- {
		if holdRowExpired(logs[i].CreatedAt, now) {
			return false
		}
		if logs[i].Message == message {
			return true
		}
	}
	return false
}

// holdRowExpired is true when a hold row is old enough to be written again.
// Unknown clocks (zero on either side) never expire: without a tick time the
// dedup stays the plain "same message" rule.
func holdRowExpired(rowAt, now time.Time) bool {
	if rowAt.IsZero() || now.IsZero() {
		return false
	}
	return now.Sub(rowAt) >= holdRelogAfter
}
