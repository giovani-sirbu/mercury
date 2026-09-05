package cooldown

import (
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// WidenSettingsForNextDepth returns the settings slice the engines hand to
// the strategy on a tick where NextDepthDoubled holds: a COPY whose base row
// carries double the step, so the second depth arms at −(2p + t) instead of
// −(p + t). The input is never mutated — trade state aliases these slices —
// and nothing is persisted: the copy lives for one GetPosition call, exactly
// like crashguard.WidenSettingsForCrash, and the ladder reverts by itself
// the moment the second fill lands (NextDepthDoubled is false from two fills
// on). Empty settings come back as they are.
//
// The base row, through ladder.SettingsIndexOrBase(settings, 0), is the row
// the reference levels were priced from (firstFillLevelsFrom) — the same p
// the entered row promised to double. Sizing is untouched:
// ladder.CalculateInitialBid reads the settings on the trade, not this copy.
func WidenSettingsForNextDepth(settings []aggragates.StrategySettings) []aggragates.StrategySettings {
	if len(settings) == 0 {
		return settings
	}
	widened := append([]aggragates.StrategySettings(nil), settings...)
	widened[ladder.SettingsIndexOrBase(settings, 0)].Percentage *= 2
	return widened
}
