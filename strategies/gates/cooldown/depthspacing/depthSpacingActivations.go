package depthspacing

import (
	"time"

	"github.com/giovani-sirbu/mercury/strategies/gates"
	"github.com/giovani-sirbu/mercury/strategies/gates/cooldown"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// spacingEvents remembers the decoded depth spacing events read on every print.
var spacingEvents aggragates.EventMemo[cooldown.DepthSpacingEvent]

// depthSpacingActivations is, per depth, the earliest stamp of a depth-spacing held event naming it,
// the gate's activation at that depth; a later event for the same depth is the hold written again,
// the step it carries is never read back, and unstamped or unreadable events are skipped.
func depthSpacingActivations(spacing []aggragates.TradesStrategyEvents) map[int]time.Time {
	activations := make(map[int]time.Time)
	for _, event := range spacing {
		if event.Param != aggragates.StrategyParamCooldown || event.Gate != cooldown.GateDepthSpacing {
			continue
		}
		if event.CreatedAt.IsZero() {
			continue
		}

		data, err := spacingEvents.Decode(event)
		if err != nil || data.Event != gates.EventHeld || data.Depth < 1 {
			continue
		}

		at := event.CreatedAt.UTC()
		if first, seen := activations[data.Depth]; !seen || at.Before(first) {
			activations[data.Depth] = at
		}
	}

	return activations
}
