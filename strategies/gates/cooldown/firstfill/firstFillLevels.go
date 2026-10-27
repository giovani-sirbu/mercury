package firstfill

import (
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// firstFillLevels prices the hold with the ladder's own arithmetic (strategies.Strategy.GetPosition),
// from the reference R and the armed anchor A on a long; an inverse ladder mirrors every level:
//
//	up(R)     = R / (1 − p/100)          the hold called the wrong direction
//	arm(R)    = R / (1 + (p + t)/100)    the entry arms, as BUY_TO_STOPLOSS
//	bounce(A) = A / (1 − t/100)          the entry fills, as STOPLOSS_TO_BUY
//	trail(A)  = A / (1 + (tr + t)/100)   the anchor follows a full step lower
type firstFillLevels struct {
	percentage float64 // p, the ladder step
	tolerance  float64 // t
	trailing   float64 // tr, the row's trailingTakeProfit
	inverse    bool    // a spot short side: the inverse ladder
}

// firstFillLevelsFrom reads the levels off the pair's base row; it fails open on no row, no step or
// a step that would put a level at or below zero.
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
	// Both distances are refused on both sides, so a strategy prices the same whichever way it enters.
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

// The comparisons are named in the long frame and mirror on an inverse ladder, inclusive where the
// ladder is inclusive (arm, release) and strict where it is strict (bounce, trail).

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
