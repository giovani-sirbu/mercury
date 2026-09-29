// Package crashguard is the CrashGuard flag's overlay on the ladder: the
// slow-decline hold on deep trades (ApplyToHold) and the capitulation
// override that lets a shallow dump take one extra fill
// (ApplyCapitulationOverride). It matches regime hold reasons by their text.
package crashguard

import (
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// ApplyToHold is the CrashGuard flag's overlay: while sophos reads a slow
// decline, a deep long ladder stops committing capital the way the decline
// would otherwise draw it in — one depth after another at the top of every
// small bounce. Its reason replaces whatever held. It does not release a
// profit hold and does not flatten. The caller owns the flag — this runs only
// under params.CrashGuard, so UseAI cannot arm the guard by itself.
func ApplyToHold(event events.Events, position string, ai aggragates.AIIndicators, hold string) string {
	if reason := holdReason(event, position, ai); reason != "" {
		return reason
	}
	return hold
}

// holdReason is the slow-decline hold on a long stopLoss transition from
// DeRiskMinDepth filled entries:
//
//   - slow decline with free fall: every stopLoss transition holds — arming
//     the next depth and the trailing re-anchor alike. The price has left
//     every support of the window behind and there is no level to buy at;
//   - slow decline alone: only the ARMING of the next depth holds, and only
//     while the tick is above the level SlowDeclineDepthFactor steps down
//     (slowDeclineArmLevel). Once the price pays that distance the depth arms
//     as it always would.
//
// Inverse ladders are never held: the verdict reads a falling market, which
// is the side an inverse ladder is not trapped on.
func holdReason(event events.Events, position string, ai aggragates.AIIndicators) string {
	if event.Trade.Inverse || position != "stopLoss" || !ai.SlowDecline {
		return ""
	}
	filled := ladder.CountFilledEntries(event.Trade)
	if filled < DeRiskMinDepth {
		return ""
	}
	if ai.FreeFall {
		return FreeFallHoldReason
	}
	if event.Params.OldPosition != "buy" {
		return ""
	}
	level, ok := slowDeclineArmLevel(event.Trade, event.Params.OldPositionPrice, filled)
	if !ok || event.Trade.PositionPrice <= level {
		return ""
	}
	return slowDeclineParkedReason(event.Trade, level)
}
