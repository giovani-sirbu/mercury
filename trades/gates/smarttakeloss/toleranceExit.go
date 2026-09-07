package smarttakeloss

import (
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// toleranceExitReached is the last permitted depth's own exit: the price
// fell one tolerance under the last fill (rose one tolerance over it on an
// inverse ladder), the tolerance read from the row of the depth that filled
// last — the same row and the same reference cooldown's depth spacing
// prices its release from. The reference is the last FILL, a persisted
// fact, never the trailed stopLoss anchor. A row without a tolerance never
// exits here.
func toleranceExitReached(trade aggragates.Trades, st state, price float64) bool {
	lastFill := st.lastFill().Price
	settings := trade.StrategyPair.StrategySettings
	if lastFill <= 0 || len(settings) == 0 {
		return false
	}
	row := settings[ladder.SettingsIndexOrBase(settings, len(st.fills)-1)]
	if row.Tolerance <= 0 {
		return false
	}
	discount := row.Tolerance / 100
	if trade.Inverse {
		return price >= lastFill*(1+discount)
	}
	return price <= lastFill*(1-discount)
}

// toleranceReason names the leg in the exit row.
func toleranceReason(inverse bool) string {
	if inverse {
		return reasonToleranceAbove
	}
	return reasonToleranceUnder
}
