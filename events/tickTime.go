package events

import (
	"time"

	"github.com/giovani-sirbu/mercury/helpers"
)

// TickTime is the tick clock: Timestamp (seconds, millis, micros or nanos)
// when the engine stamps one, else the trade's own stamp (sisyphus keeps
// simulated time in UpdatedAt), else zero. A zero clock means "unknown", and
// every age-based rule fails open on it.
func (e Events) TickTime() time.Time {
	if e.Timestamp > 0 {
		return time.UnixMilli(helpers.UnixMillis(e.Timestamp)).UTC()
	}
	return e.Trade.UpdatedAt
}

// TickMillis is the tick clock as Unix milliseconds, 0 when no clock is known.
func (e Events) TickMillis() int64 {
	if e.Timestamp > 0 {
		return helpers.UnixMillis(e.Timestamp)
	}
	if !e.Trade.UpdatedAt.IsZero() {
		return e.Trade.UpdatedAt.UnixMilli()
	}
	return 0
}
