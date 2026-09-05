package regime

import (
	"fmt"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// profitHoldReason defers a profitable close while the trigger timeframe
// still moves in the trade's favor: uptrend-persist for a long sell,
// downtrend-persist for an inverse buyback. Empty means sell now. Only from
// ProfitHoldMinDepth up, never while another trade of the wallet is
// funds-blocked (the close IS the capital the ladder waits for), and 4h
// against (C.4) still releases.
func profitHoldReason(event events.Events, ai aggragates.AIIndicators) string {
	// THE RATCHET IS NOT THE ARMING, and the engines cannot tell them apart
	// for us: both rewrite Trade.PositionType to "takeProfit" before the chain
	// runs, so this gate sees the same value on the tick that ARMS the exit and
	// on every trailing re-anchor after it.
	//
	// Holding the arming is the whole point: the exit is not placed, so the
	// trade stays in position and rides the trend. Holding the re-anchor is the
	// opposite: it freezes the exit anchor while the trend runs, so the
	// eventual sell lands lower than the trail would have given. The `sell`
	// chain carries no shouldHold, so the hold cannot even defer the close —
	// it only breaks the ratchet. The engine already trails on its own
	// (`percentage > trailingTakeProfit ? 'update_takeProfit'`), which is
	// exactly what this would block.
	//
	// Params.OldPosition is the position the trade held before the logic ran,
	// and every engine sets it. Normalised, "takeProfit" there means the exit
	// is already armed and this tick is only moving its anchor.
	if gates.PositionType(event.Params.OldPosition) == "takeProfit" {
		return ""
	}

	if ladder.CountFilledEntries(event.Trade) < ProfitHoldMinDepth {
		return ""
	}
	if event.Params.PortfolioBlocked {
		return ""
	}

	label := ai.Regimes[profitHoldTimeframe]
	favorable := UpPersist
	if event.Trade.Inverse {
		favorable = DownPersist
	}
	if label != favorable {
		return ""
	}

	// 15m-in-favor vs 4h-against is disagreement, not a hold (C.4).
	if fourHourAgainstLong(ai, event.Trade.Inverse) {
		return ""
	}

	return fmt.Sprintf("regime: rides the trend (%s %s)", profitHoldTimeframe, label)
}

// fourHourAgainstLong is C.4 disagreement for a long profit-exit: 4h is
// sliding or in a down-shock while 15m still reads as a ride. Inverse is
// never "against" on a dump — the flush moves in its favor.
func fourHourAgainstLong(ai aggragates.AIIndicators, inverse bool) bool {
	if inverse {
		return false
	}
	switch ai.Regimes["4h"] {
	case DownPersist, ShockDown:
		return true
	}
	return false
}
