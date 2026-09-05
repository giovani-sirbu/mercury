// Package crashguard is the CrashGuard flag's overlay on the ladder: the
// flush park and sticky reclaim on deep trades (ApplyToHold) and the
// capitulation override that lets a shallow dump take one extra fill
// (ApplyCapitulationOverride). It matches hold reasons from regime and from
// smart take loss by their text; it never imports smarttakeloss (which
// imports this package for DeRiskMinDepth).
package crashguard

import (
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/regime"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// DeepHoldReason is the flush park on a deep trade. The capitulation
// override keeps it by its "crash-guard: deep" prefix (keepCapitulationHold).
const DeepHoldReason = "crash-guard: deep trade, no new capital during a flush"

// ApplyToHold is the CrashGuard flag's overlay: a flush parks a deep
// rebuy, replacing whatever held. It does not release a profit hold and does
// not flatten. The caller owns the flag — this runs only under
// params.CrashGuard, so UseAI cannot arm the guard by itself.
func ApplyToHold(event events.Events, position string, ai aggragates.AIIndicators, hold string) string {
	if reason := holdReason(event, position, ai); reason != "" {
		return reason
	}
	return hold
}

func holdReason(event events.Events, position string, ai aggragates.AIIndicators) string {
	if position != "stopLoss" {
		return ""
	}
	filled := ladder.CountFilledEntries(event.Trade)
	if filled < DeRiskMinDepth {
		return ""
	}
	// Direction-blind: this is a cap on committing MORE capital at
	// maximum ladder depth during maximum chaos, not a sale. Inverse
	// still parks — a flush is the wrong moment to add size on either
	// side. sellLoss stays on Smart Take Loss.
	if ai.CrashActive {
		return DeepHoldReason
	}
	if TradeHasCrashArmed(event.Trade) || ai.CrashSticky {
		if fourHourUnreclaimed(ai, event.Trade.Inverse) {
			return "crash-guard: sticky flush, waiting for 4h reclaim"
		}
	}
	return ""
}

// fourHourUnreclaimed is the sticky-ARM release: 4h still against the
// trade. A CLEAR without a reclaim is the run-90 gap that let fills 5–8
// through. A MISSING regime verdict (sophos outage, empty cache) does not
// park: every other AI gate degrades open on a missing verdict, and failing
// closed here parked every previously-armed deep trade indefinitely, with
// no way back but the verdict returning.
func fourHourUnreclaimed(ai aggragates.AIIndicators, inverse bool) bool {
	if !ai.HasRegimeVerdict {
		return false
	}
	label := ai.Regimes["4h"]
	if inverse {
		return label == regime.UpPersist || regime.ShockBlocks(label, true)
	}
	return label == regime.DownPersist || regime.ShockBlocks(label, false)
}
