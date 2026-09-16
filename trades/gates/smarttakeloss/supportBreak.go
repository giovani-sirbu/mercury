package smarttakeloss

import (
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// supportBreakReached is the support line read against the tick, from
// ARMING on — it does not wait for the activation, and it does not need it:
// a ladder deep enough to be watched that is holding its support line goes
// on trading normally, adds included, and a ladder whose support gave way
// sells. The line is the one sophos draws on the same 4h window through the
// two most recent LOWER lows (SupportSwingLookback bars each side) — the
// support a falling market holds, drawn like the resistance through two
// lower highs — served with how many of the newest closed bars in a row
// CLOSED under it. Two conditions, both on this tick:
//
//   - at least SupportBreakBars closed bars stayed under the line
//     (ai.SupportBarsUnder, counted by sophos);
//   - the tick price plus one tolerance is still under the line projected to
//     now: price · (1 + tolerance) < level, the tolerance read from the row
//     of the depth that filled last, as the tolerance exit reads it.
//
// A bounce that took the price back to within a tolerance of the line does
// not sell, whatever the count says: the bars closed under it, the price no
// longer is.
//
// The second condition is the failed retest. A price that reached the line
// and bounced — without the bounce reaching a sell target — and then comes
// back under the support it bounced from sells one bar sooner: sophos
// serves that support (the line's value where the price last touched it and
// closed back over, ai.SupportBounceLevel) with how many of the newest
// closed bars stayed under it, and the trade sells once
// SupportBounceBreakBars of them stand and the tick price is under the
// level. No tolerance on this leg: the level already held once.
//
// An inverse ladder mirrors both on the resistance through the two most
// recent HIGHER highs: bars that closed OVER it, price minus one tolerance
// over it, and the resistance it bounced from. The bounce targets — the
// resistance through lower highs, the support through higher lows — are
// never read here. No line, no verdict, no fill or a row without a
// tolerance never sells here.
func supportBreakReached(trade aggragates.Trades, st state, price float64, nowMs int64, ai aggragates.SmartTakeLossIndicators) (string, bool) {
	settings := trade.StrategyPair.StrategySettings
	if price <= 0 || !ai.HasVerdict || len(st.fills) == 0 || len(settings) == 0 {
		return "", false
	}
	row := settings[ladder.SettingsIndexOrBase(settings, len(st.fills)-1)]
	if row.Tolerance <= 0 {
		return "", false
	}
	margin := row.Tolerance / 100
	if trade.Inverse {
		level, ok := projectLine(ai.HigherHighs, nowMs)
		if ok && ai.ResistanceBarsOver >= SupportBreakBars && price*(1-margin) > level {
			return reasonResistanceBreak, true
		}
		if ai.ResistanceBounceLevel > 0 && ai.ResistanceBounceBarsOver >= SupportBounceBreakBars && price > ai.ResistanceBounceLevel {
			return reasonResistanceBounceBreak, true
		}
		return "", false
	}
	level, ok := projectLine(ai.LowerLows, nowMs)
	if ok && ai.SupportBarsUnder >= SupportBreakBars && price*(1+margin) < level {
		return reasonSupportBreak, true
	}
	if ai.SupportBounceLevel > 0 && ai.SupportBounceBarsUnder >= SupportBounceBreakBars && price < ai.SupportBounceLevel {
		return reasonSupportBounceBreak, true
	}
	return "", false
}
