// Package gates is the plumbing every strategy-flag gate shares: the INFO
// hold row a gate writes on the trade, the position normalization the gates
// key on, and the futures first-fill veto. It imports no gate family; the
// families live in the sub-packages and actions.ShouldHold orders them.
package gates

import (
	"fmt"
	"time"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// holdRelogAfter is how long one hold row may stand for a hold that is still
// in force before the same reason is written again. A row per day keeps a
// three-week blockade readable as three weeks instead of one line dated the
// first tick (run 97: one "4h downtrend-persist" row covered 25 simulated
// days), while a flip-flopping gate still collapses within the day.
const holdRelogAfter = 24 * time.Hour

// SaveHoldLog records a hold as an INFO entry in the trade logs, restores the
// previous position and stops the action chain.
//
// Deduplication is on the FULL message, not on the "Hold <position>:" prefix:
// with the prefix, every later stopLoss hold of a different reason (capitulation
// after regime, shock after capitulation) was silently dropped, and a cooldown
// entry hold hid the regime entry veto behind it. A repeat of the same reason
// is written again once the previous row is older than holdRelogAfter on the
// tick clock, so the duration of a hold is on record too.
func SaveHoldLog(event events.Events, position string, reason string) (events.Events, error) {
	message := fmt.Sprintf("Hold %s: %s", position, reason)
	// Wrapped so the chain still stops here while the runner keeps quiet about
	// it: the INFO entry below is the record of this decision.
	err := fmt.Errorf("%w: %s %s", events.ErrTradeHeld, position, reason)

	now := event.TickTime()
	if holdLoggedWithin(event.Trade.Logs, message, now) {
		return event, err
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
		// Stamped with the tick clock when the engine provides one, so the
		// age test above works on replayed tape; a persisting engine that
		// stamps its own time overwrites it.
		CreatedAt: now,
		UpdatedAt: now,
	})

	newEvent, _ := event.Events["updateTrade"](event)

	return newEvent, err
}

// holdLoggedWithin reports whether this exact message already stands in the
// trade's log within the last holdRelogAfter. Logs are chronological, so the
// scan runs backwards and stops at the first row old enough to be written
// again. Comparing only with the LAST row let two reasons that alternate
// tick by tick (A-B-A-B) write on every tick — run 98: 18 rows a day on one
// trade; the window collapses that to one row per reason per day. Without a
// clock (rows never expire) this is the plain "same message anywhere" rule.
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
