package smarttakeloss

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// sellTargetReached is the exit an active trade watches on every tick: the
// resistance line through the last two lower highs, projected to now, or
// the upper Bollinger band. Both are read on the smart take loss window
// sophos serves, so the band spans BBPeriod bars of that window's Interval
// and two line anchors are never closer than their lookback + 1 bars of it —
// SwingLookback for the highs, SupportSwingLookback for the lows. The band
// is the leg that actually closes these trades; on an inverse ladder the
// support line through the last two higher lows or the lower band. Touching
// the level (>=, <=) sells; the engines place the sellLoss limit at the tick
// price.
//
// The line is ignored while its projection sits at or under the last fill
// (at or over it on an inverse ladder). At the bottom of the window a line
// through two older lower highs can already run below the price, and a line
// that re-anchors after the activation can project UNDER a depth that has
// not filled yet — selling before that depth lands, under the price the
// ladder was about to pay. The band leg stands on its own; with no line
// served it is the only target.
func sellTargetReached(trade aggragates.Trades, st state, price float64, nowMs int64, ai aggragates.SmartTakeLossIndicators) (string, bool) {
	lastFill := st.lastFill().Price
	if lastFill <= 0 {
		return "", false
	}
	if trade.Inverse {
		if level, ok := projectLine(ai.Support, nowMs); ok && level < lastFill && price <= level {
			return reasonSupportLine, true
		}
		if ai.LowerBB > 0 && price <= ai.LowerBB {
			return reasonLowerBand, true
		}
		return "", false
	}
	if level, ok := projectLine(ai.Resistance, nowMs); ok && level > lastFill && price >= level {
		return reasonResistanceLine, true
	}
	if ai.UpperBB > 0 && price >= ai.UpperBB {
		return reasonUpperBand, true
	}
	return "", false
}
