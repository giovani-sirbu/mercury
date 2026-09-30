package cooldown

import (
	"time"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates"
)

// depthSpacingActivations rebuilds, from the trade's strategy events, when the
// depth-spacing gate activated for each depth: the earliest stamp of a
// depth-spacing event that names the depth. The events are the only state, the
// way firstFillState rebuilds the first-fill gate, and they reach the gate on
// every engine. Events of any other gate or param are never read, the depth
// priority's among them: its document carries a depth of its own.
//
// A later event for the same depth is gates.SaveHoldLog writing the standing
// hold again once its row ages out, so it is the same activation and never a
// new one. The earliest stamp is the first event the trade carries, the order
// the engines keep, and stays the activation however a rehydrated trade orders
// them. The step and the hold an event carries are not read: they are derived,
// and trusting them would let an old event overrule the rule as it stands. An
// event with no stamp, one whose document cannot be read, one of a kind this
// fold does not know and one that names no depth carries nothing to measure
// and is skipped. A hold row without its event is no activation: the row is
// the text an operator reads.
func depthSpacingActivations(spacing []aggragates.TradesStrategyEvents) map[int]time.Time {
	activations := make(map[int]time.Time)
	for _, event := range spacing {
		if event.Param != aggragates.StrategyParamCooldown || event.Gate != GateDepthSpacing {
			continue
		}
		if event.CreatedAt.IsZero() {
			continue
		}

		var data DepthSpacingEvent
		if err := event.DecodeData(&data); err != nil || data.Event != gates.EventHeld || data.Depth < 1 {
			continue
		}

		at := event.CreatedAt.UTC()
		if first, seen := activations[data.Depth]; !seen || at.Before(first) {
			activations[data.Depth] = at
		}
	}

	return activations
}
