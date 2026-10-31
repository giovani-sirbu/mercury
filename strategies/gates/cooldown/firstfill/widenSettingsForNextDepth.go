package firstfill

import (
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// WidenSettingsForNextDepth is a copy of settings whose base row carries double the step, for one
// GetPosition call on a tick where NextDepthDoubled holds; the input is never mutated and sizing
// reads the trade's own settings.
func WidenSettingsForNextDepth(settings []aggragates.StrategySettings) []aggragates.StrategySettings {
	if len(settings) == 0 {
		return settings
	}
	widened := append([]aggragates.StrategySettings(nil), settings...)
	widened[ladder.SettingsIndexOrBase(settings, 0)].Percentage *= 2
	return widened
}
