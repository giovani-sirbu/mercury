package smarttakeloss

import (
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates"
)

// SlowDeclineEntryHoldReason is the reason a first fill is held with while
// the quiet slow-decline verdict stands. It is human-readable text,
// byte-stable for cp and the notification filter, and it is constant on
// purpose: gates.SaveHoldLog collapses an identical reason tick to tick, so
// the hold writes one row, not one per tick.
const SlowDeclineEntryHoldReason = "smartTakeLoss: quiet slow decline, first fill held"

// EntryHold is the smart take loss's first-fill hold: no new ladder opens on a
// pair while sophos reads a quiet slow decline on it — the decline the exit
// sells a long ladder out of. It holds a parent only (an impasse child
// belongs to the impasse chain), a long entry only (side is
// aggragates.EntrySide, the direction the entry would take), and only while
// the verdict stands; the zero Hold otherwise. A refusal names its row's text
// (SlowDeclineEntryHoldReason) and the held event that goes beside it, filed
// under GateEntryHold, which gates.SaveHoldLog writes. It reads the verdict
// alone — its leg on and still down sophos' SlowDeclineMinLegFallPct from its
// high close, and its vote passing with the smoothness read over sophos' own
// span: a pair with no ladder has no fill to count the smoothness from, so it
// reads the span sophos reads for itself. While sophos' SlowDeclineSmcTrend
// is on, the verdict is served only with the SMC trend dashboard bearish on
// every timeframe sophos' SmcTrendTimeframes names, so the hold asks that as
// well. The bar among the last closed ones on which the verdict stood
// (SlowDeclineRecentAt) holds nothing: it reads for a ladder by that ladder's
// newest fill (slowDeclineReadsRecently), and a pair with no ladder has none.
// The flag comes first, as in Armed: a verdict fetched for another flag's
// sake holds nothing. While QuietSlowDeclineExit is off it holds nothing
// either.
func EntryHold(trade aggragates.Trades, side string, ai aggragates.AIIndicators) gates.Hold {
	if !quietSlowDeclineExit || !trade.Strategy.Params.SmartTakeLoss || trade.ParentID != 0 {
		return gates.Hold{}
	}
	if side != aggragates.SideLong || !ai.SmartTakeLoss.SlowDeclineExit {
		return gates.Hold{}
	}

	return gates.Hold{
		Reason: SlowDeclineEntryHoldReason,
		Param:  aggragates.StrategyParamSmartTakeLoss,
		Gate:   GateEntryHold,
		Data:   EventData{Event: gates.EventHeld},
	}
}
