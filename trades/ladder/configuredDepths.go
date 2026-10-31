package ladder

import (
	"math"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// ConfiguredDepths is how many entries the ladder may fill: the floored Depths
// of the row the NEXT fill reads, on the rows the ladder trades
// (tradedSettings), so a ladder that opened raised keeps its raised ceiling.
// A trade whose pair carries no settings answers 0, read as unknown, never as full.
func ConfiguredDepths(trade aggragates.Trades) int {
	return ceilingOf(tradedSettings(trade), CountFilledEntries(trade))
}

// ceilingOf is ConfiguredDepths for a caller that already holds the traded rows and the filled count.
func ceilingOf(settings []aggragates.StrategySettings, filled int) int {
	if len(settings) == 0 {
		return 0
	}

	row := SettingsIndexOrBase(settings, filled)

	return int(math.Floor(settings[row].Depths))
}
