package crashguard

import (
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// SlowDeclineDepthFactor is how many ladder steps under the anchor the next
// depth arms while a slow decline stands without free fall: the entry the
// ladder arms at −(p + t) is held until −(SlowDeclineDepthFactor·p + t).
//
// It is a hold with a price release, not a wider settings row.
// strategies.Strategy.GetPosition binds one percentage to every transition of
// the row, so widening the row would move the take profit by the same factor;
// holding the arming leaves the row, and the take profit, exactly as
// configured.
const SlowDeclineDepthFactor = 4

// SlowDeclineHoldPrefix opens every crash-guard hold reason.
const SlowDeclineHoldPrefix = "crash-guard: slow decline"

// FreeFallHoldReason is the hold while a slow decline has left every support
// of sophos' free-fall window behind: no new depth and no re-anchor.
const FreeFallHoldReason = SlowDeclineHoldPrefix + " with no support below, no new depth"

// slowDeclineArmLevel is the price at which the held depth may arm: the
// anchor the ladder measures from, SlowDeclineDepthFactor steps down. The
// engines measure
//
//	percentage = (price − anchor) / price × 100
//
// so the arm at −(p + t) solves to anchor / (1 + (p + t)/100), and the level
// here is the same expression with SlowDeclineDepthFactor·p. p and t come from
// the settings row strategies.GetPosition binds for the held depth
// (ladder.SettingsIndexOrBase of the last filled entry). It fails OPEN: no
// anchor, no row or no step means the level cannot be priced and nothing is
// held.
func slowDeclineArmLevel(trade aggragates.Trades, anchor float64, filled int) (float64, bool) {
	settings := trade.StrategyPair.StrategySettings
	if anchor <= 0 || len(settings) == 0 {
		return 0, false
	}
	row := settings[ladder.SettingsIndexOrBase(settings, filled-1)]
	if row.Percentage <= 0 {
		return 0, false
	}
	distance := (SlowDeclineDepthFactor*row.Percentage + row.Tolerance) / 100
	if distance <= 0 {
		return 0, false
	}
	return anchor / (1 + distance), true
}

// slowDeclineParkedReason names the level the held depth waits for. The
// anchor and the settings row are both frozen while the hold stands — a held
// arming leaves the position on `buy` at its old price — so the text is
// byte-identical tick to tick and gates.SaveHoldLog collapses it.
func slowDeclineParkedReason(trade aggragates.Trades, level float64) string {
	return SlowDeclineHoldPrefix + ", next depth parked until " + gates.FormatPriceLevel(trade, level)
}
