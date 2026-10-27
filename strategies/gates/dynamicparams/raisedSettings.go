package dynamicparams

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// RaisedSettings is the rows a trade trades, rebuilt per tick: the stored rows raised by its opened
// event's amounts (OpenedRaise, RaiseBy) and true while the flag shapes the trade (Applies), else
// the stored rows and false; nothing stores the raised rows.
func RaisedSettings(trade aggragates.Trades) ([]aggragates.StrategySettings, bool) {
	stored := trade.StrategyPair.StrategySettings
	if !Applies(trade) {
		return stored, false
	}

	points, depths, opened := OpenedRaise(trade)
	if !opened || !raisesAnyRow(stored, points, depths) {
		return stored, false
	}

	return RaiseBy(stored, points, depths), true
}
