package dynamicparams

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// RaiseBy is a copy of settings with every row's percentage raised by points and its depths by
// depths, or settings itself when nothing is raised. settings is never written: the backtest
// shares it with its snapshot, so a raise written into it would compound tick to tick.
func RaiseBy(settings []aggragates.StrategySettings, points float64, depths int) []aggragates.StrategySettings {
	if !raisesAnyRow(settings, points, depths) {
		return settings
	}

	raised := append([]aggragates.StrategySettings(nil), settings...)
	for index := range raised {
		if points != 0 {
			raised[index].Percentage += points
		}
		if depths != 0 {
			raised[index].Depths += float64(depths)
		}
	}

	return raised
}

// raisesAnyRow is whether RaiseBy changes settings: a row, and an amount that is not zero.
func raisesAnyRow(settings []aggragates.StrategySettings, points float64, depths int) bool {
	return len(settings) > 0 && (points != 0 || depths != 0)
}
