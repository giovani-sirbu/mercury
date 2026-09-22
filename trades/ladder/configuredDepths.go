package ladder

import (
	"math"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// ConfiguredDepths is how many entries the trade's grid is allowed to fill:
// the Depths of the settings row the NEXT fill would use, floored — Depths is
// configured as a float and a fraction of an entry cannot be placed.
//
// It reads the row of the next fill, not of the last one, because that is the
// row that governs the entry being decided; SettingsIndexOrBase falls back to
// the base row for a depth no row was configured for. A trade whose pair
// carries no settings at all answers 0: no ceiling is known, and every caller
// reads 0 as unknown rather than as a full ladder.
func ConfiguredDepths(trade aggragates.Trades) int {
	settings := trade.StrategyPair.StrategySettings
	if len(settings) == 0 {
		return 0
	}

	row := SettingsIndexOrBase(settings, CountFilledEntries(trade))

	return int(math.Floor(settings[row].Depths))
}
