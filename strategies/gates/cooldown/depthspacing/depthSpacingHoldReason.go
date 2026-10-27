package depthspacing

import (
	"fmt"
	"strconv"
	"time"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/strategies/gates"
	"github.com/giovani-sirbu/mercury/strategies/gates/cooldown"
	"github.com/giovani-sirbu/mercury/strategies/gates/smarttakeloss"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// depthSpacingState is the fold's result: when the next depth may arm (zero when the history could
// not be read, the gate open), the step of the cascade and the capped hold it earned.
type depthSpacingState struct {
	eligibleFrom time.Time
	step         int
	hold         time.Duration
}

// DepthSpacingHold is the gate on an open position, stopLoss only: a refusal while the hold the last
// fast depth earned stands, unless the price release, a triggered smart take loss gate
// (disabledWhenActiveSTL) or a latched early take profit (disabledWhenEarlyTakeProfit) lifts
// it; inert on unstamped fills, open without a tick clock.
func DepthSpacingHold(event events.Events, position string) gates.Hold {
	if position != "stopLoss" {
		return gates.Hold{}
	}

	// Unknown tick clocks never hold.
	now := event.TickTime()
	if now.IsZero() {
		return gates.Hold{}
	}

	fills := depthFills(event.Trade)
	state := depthSpacingEligibleFrom(event.Trade.StrategyEvents, fills, now)
	if state.eligibleFrom.IsZero() || !now.UTC().Before(state.eligibleFrom) {
		return gates.Hold{}
	}

	// The market may already have paid the hold (depthSpacingPriceReleased).
	if depthSpacingPriceReleased(event.Trade, event.Trade.PositionPrice, fills[len(fills)-1].Price, state.step) {
		return gates.Hold{}
	}
	if disabledWhenActiveSTL && smarttakeloss.Triggered(event.Trade) {
		return gates.Hold{}
	}
	if disabledWhenEarlyTakeProfit && smarttakeloss.EarlyTakeProfitLatched(event.Trade) {
		return gates.Hold{}
	}

	// Byte-stable while the hold stands (SaveHoldLog deduplicates on it): the depth is len(fills), not
	// the step; the release price is frozen with the last fill and the remaining time is left out.
	data := cooldown.DepthSpacingEvent{
		Event: gates.EventHeld,
		Depth: len(fills),
		Step:  state.step,
		Hold:  state.hold,
	}
	if release, ok := depthSpacingReleasePrice(event.Trade, fills[len(fills)-1].Price, state.step); ok {
		data.Release = release
	}

	return gates.Hold{
		Reason: depthSpacingHoldMessage(data),
		Param:  aggragates.StrategyParamCooldown,
		Gate:   cooldown.GateDepthSpacing,
		Data:   data,
	}
}

// depthSpacingHoldMessage is the hold's row text, formatted from the event beside it; it names the
// release price only when the event carries one.
func depthSpacingHoldMessage(data cooldown.DepthSpacingEvent) string {
	if data.Release <= 0 {
		return fmt.Sprintf(
			"cooldown: depths too close (depth %d, step %d), next add parked for %s",
			data.Depth, data.Step, data.Hold,
		)
	}
	return fmt.Sprintf(
		"cooldown: depths too close (depth %d, step %d), next add parked for %s or until %s",
		data.Depth, data.Step, data.Hold, strconv.FormatFloat(data.Release, 'f', -1, 64),
	)
}
