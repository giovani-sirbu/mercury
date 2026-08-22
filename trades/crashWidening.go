package trades

import (
	"math"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// crashWidenMaxPct caps a widened rung. GetInitialBidByDepth returns 0 for a
// percentage at or above 100, and percentage+tolerance must stay well under
// that; 45% per rung already covers a 10.10.2025-style flush within a few
// depths.
const crashWidenMaxPct = 45.0

// WidenedPercentage is the percentage the engines should evaluate for the row
// governing depthIndex while the crash guard decides sizing. With the signal
// off it is exactly the configured value. With it on, the ladder turns
// geometric past the configured rows: the multiplier is 2^(depthIndex−rowIdx),
// where rowIdx is the row SettingsIndexOrBase actually serves — so a depth
// whose own row exists keeps its configured percentage, and every depth that
// falls back to the base row doubles per depth (2% → 4% → 8% → 16% → 32%),
// capped at crashWidenMaxPct. The bool reports whether widening changed the
// value.
func WidenedPercentage(settings []aggragates.StrategySettings, depthIndex int, crashActive bool) (float64, bool) {
	if len(settings) == 0 {
		return 0, false
	}
	rowIdx := SettingsIndexOrBase(settings, depthIndex)
	base := settings[rowIdx].Percentage
	if !crashActive || depthIndex <= rowIdx || base <= 0 {
		return base, false
	}
	widened := base * math.Pow(2, float64(depthIndex-rowIdx))
	if widened > crashWidenMaxPct {
		widened = crashWidenMaxPct
	}
	return widened, widened != base
}

// WidenSettingsForCrash returns the settings slice the engines should hand to
// the strategy for this tick: the input itself while the crash signal is off,
// and a COPY with the governing row's Percentage widened while it is on. The
// input is never mutated — trade state aliases these slices — and nothing is
// persisted, so the ladder reverts by itself on the first tick after the
// signal clears.
func WidenSettingsForCrash(settings []aggragates.StrategySettings, depthIndex int, crashActive bool) []aggragates.StrategySettings {
	widened, changed := WidenedPercentage(settings, depthIndex, crashActive)
	if !changed {
		return settings
	}
	widenedSettings := append([]aggragates.StrategySettings(nil), settings...)
	widenedSettings[SettingsIndexOrBase(settings, depthIndex)].Percentage = widened
	return widenedSettings
}
