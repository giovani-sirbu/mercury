package dynamicparams

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// Adjust is the rows the reads raise: a COPY of settings in which every row's
// percentage is raised by BearPercentagePoints and/or its depths by
// BearDepths, as Raise(TierOf(reads)) asks, and no other field moves. When
// nothing is raised — no rows, IncreaseNone, or an increase whose amounts are
// zero — it is settings itself, the very slice.
//
// settings is never written. It is the trade's stored rows, which the
// backtest shares with its run's snapshot and writes back to memory after
// every tick, so a raise written into it would compound from tick to tick and
// be saved with the trade. The copy is a fresh array: nothing written to it
// reaches settings either.
func Adjust(settings []aggragates.StrategySettings, reads aggragates.DynamicParamsIndicators) []aggragates.StrategySettings {
	return raiseRows(settings, Raise(TierOf(reads)))
}

// raiseRows is Adjust for an increase already read.
func raiseRows(settings []aggragates.StrategySettings, increase Increase) []aggragates.StrategySettings {
	if len(settings) == 0 || !increase.changesRows() {
		return settings
	}

	raised := append([]aggragates.StrategySettings(nil), settings...)
	for index := range raised {
		if increase.raisesPercentage() {
			raised[index].Percentage += BearPercentagePoints
		}
		if increase.raisesDepths() {
			raised[index].Depths += float64(BearDepths)
		}
	}

	return raised
}
