package cooldown

import (
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// The first-fill hold is priced with the ladder's own arithmetic, so a held
// entry passes through the same levels the ladder would have taken it
// through once filled. The engines measure
//
//	percentage = (price − anchor) / price × 100
//
// with the sign flipped on an inverse trade, and the transitions are the
// same expressions in all three engines (strategies.Strategy.GetPosition,
// getLogic):
//
//	buy      → stopLoss          percentage <= −(p + t)    the depth arms
//	stopLoss → buy               percentage >  t           the bounce fills it
//	stopLoss → update_stopLoss   percentage <  −(tr + t)   the anchor trails
//
// Solved for the price on a long, from the reference R and the armed anchor A:
//
//	up(R)     = R / (1 − p/100)          the hold called the wrong direction
//	arm(R)    = R / (1 + (p + t)/100)    the entry arms, as BUY_TO_STOPLOSS
//	bounce(A) = A / (1 − t/100)          the entry fills, as STOPLOSS_TO_BUY
//	trail(A)  = A / (1 + (tr + t)/100)   a full step lower: the anchor follows
//
// An inverse ladder sells first, so every sign flips and every comparison
// mirrors: up sits below R, arm above it, and the anchor trails the high.
type firstFillLevels struct {
	percentage float64 // p, the ladder step
	tolerance  float64 // t
	trailing   float64 // tr, the row's trailingTakeProfit
	inverse    bool    // a spot short side: the inverse ladder
}

// firstFillLevelsFrom reads the levels off the pair's base row — the row the
// first depth is priced from. It fails OPEN, the posture of
// depthSpacingReleasePrice: no row, no step, or a step that would put a
// level at or below zero means the hold cannot be priced, so nothing
// activates — never a panic, never a zero level releasing every tick.
func firstFillLevelsFrom(trade aggragates.Trades, side string) (firstFillLevels, bool) {
	settings := trade.StrategyPair.StrategySettings
	if len(settings) == 0 {
		return firstFillLevels{}, false
	}
	row := settings[ladder.SettingsIndexOrBase(settings, 0)]
	levels := firstFillLevels{
		percentage: row.Percentage,
		tolerance:  row.Tolerance,
		trailing:   row.TrailingTakeProfit,
		inverse:    side == aggragates.SideShort,
	}
	if levels.percentage <= 0 {
		return firstFillLevels{}, false
	}
	// On an inverse ladder the arm divides by 1 − (p + t)/100 and the trail
	// by 1 − (tr + t)/100; a long divides by their mirrors, which are always
	// positive. Both distances are refused on both sides, so a strategy is
	// priced the same whichever way it enters.
	if (levels.percentage+levels.tolerance)/100 >= 1 || (levels.trailing+levels.tolerance)/100 >= 1 {
		return firstFillLevels{}, false
	}
	return levels, true
}

func (l firstFillLevels) up(reference float64) float64 {
	if l.inverse {
		return reference / (1 + l.percentage/100)
	}
	return reference / (1 - l.percentage/100)
}

func (l firstFillLevels) arm(reference float64) float64 {
	if l.inverse {
		return reference / (1 - (l.percentage+l.tolerance)/100)
	}
	return reference / (1 + (l.percentage+l.tolerance)/100)
}

func (l firstFillLevels) bounce(anchor float64) float64 {
	if l.inverse {
		return anchor / (1 + l.tolerance/100)
	}
	return anchor / (1 - l.tolerance/100)
}

func (l firstFillLevels) trail(anchor float64) float64 {
	if l.inverse {
		return anchor / (1 - (l.trailing+l.tolerance)/100)
	}
	return anchor / (1 + (l.trailing+l.tolerance)/100)
}

// The comparisons are named in the long frame and mirror on an inverse
// ladder, so FirstFillHold reads the same for both sides. Inclusive where the
// ladder is inclusive (the arm's `<=`, and the release through the
// reference), strict where it is strict (the bounce's `>`, the trail's `<`).

func (l firstFillLevels) atOrAbove(price, level float64) bool {
	if l.inverse {
		return price <= level
	}
	return price >= level
}

func (l firstFillLevels) above(price, level float64) bool {
	if l.inverse {
		return price < level
	}
	return price > level
}

func (l firstFillLevels) atOrBelow(price, level float64) bool {
	if l.inverse {
		return price >= level
	}
	return price <= level
}

func (l firstFillLevels) below(price, level float64) bool {
	if l.inverse {
		return price > level
	}
	return price < level
}
