// Package regime is the RegimeHold flag's gates over sophos' multi-timeframe
// regime labels: the shock hold, the add veto and the profit hold.
package regime

import (
	"fmt"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// HoldReason is the RegimeHold flag's gate on an open position.
// Empty means the chain may proceed. Never called on the first fill.
func HoldReason(event events.Events, position string, ai aggragates.AIIndicators) string {
	inverse := event.Trade.Inverse

	switch position {
	case "takeProfit":
		return profitHoldReason(event, ai)
	case "stopLoss":
		filledEntries := ladder.CountFilledEntries(event.Trade)
		// The shock veto is depth-aware, reads only the trigger timeframe and
		// only the direction the add would buy into: shallow rungs are cheap,
		// a spike is when the grid gets its best fills, and a shock moving in
		// the trade's favor is no reason to park it.
		if filledEntries >= ShockHoldMinDepth && ShockBlocks(ai.Regimes[ShockHoldTimeframe], inverse) {
			return fmt.Sprintf(ShockHoldPrefix+" (%s %s, depth %d)", ShockHoldTimeframe, ai.Regimes[ShockHoldTimeframe], filledEntries)
		}
		if !inverse && !ai.AddAllowed {
			return fmt.Sprintf(AddVetoPrefix+" (%s)", regimeDetail(ai))
		}
		if inverse {
			// A rising shock is the same uptrend this veto exists for, just
			// violent enough that the shock label outranks upPersist on the
			// wire — without ShockBlocks here, a vertical squeeze slipped the
			// veto that saved run 74's rally blow-ups precisely because it
			// was too fast. A falling shock stays tradeable: it moves in the
			// inverse trade's favor.
			for _, timeframe := range addVetoTimeframes {
				if ai.Regimes[timeframe] == UpPersist || ShockBlocks(ai.Regimes[timeframe], true) {
					return fmt.Sprintf(InverseAddVetoPrefix+" (%s %s)", timeframe, ai.Regimes[timeframe])
				}
			}
		}
	}

	return ""
}
