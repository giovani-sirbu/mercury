package dynamicparams

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// RaiseBy is settings raised by an opened row's amounts: a COPY in which
// every row's percentage is raised by points and its depths by depths, and no
// other field moves. When nothing is raised — no rows, or both amounts zero —
// it is settings itself, the very slice.
//
// settings is never written. It is the trade's stored rows, which the
// backtest shares with its run's snapshot and writes back to memory after
// every tick, so a raise written into it would compound from tick to tick and
// be saved with the trade. The copy is a fresh array: nothing written to it
// reaches settings either.
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

// raisesAnyRow reports whether RaiseBy changes settings: there is a row, and
// an amount that is not zero.
func raisesAnyRow(settings []aggragates.StrategySettings, points float64, depths int) bool {
	return len(settings) > 0 && (points != 0 || depths != 0)
}
