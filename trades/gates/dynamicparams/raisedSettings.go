package dynamicparams

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// RaisedSettings is the rows a trade trades, the one call each engine makes
// per tick: while the flag shapes the trade (Applies) and its opened row
// carries amounts that change a row (OpenedRaise), the stored rows raised by
// those amounts (RaiseBy) and true; otherwise the stored rows themselves —
// the very slice — and false, and the engine changes nothing.
//
// The reads play no part. A ladder trades the raise it opened with until it
// closes, whatever the reads say by then, and one that opened without an
// opened row trades its configured rows for life.
//
// On true the engine hands the rows to exactly two places: the
// strategies.Strategy.Settings it computes the position from, and the
// chain's aggragates.Params.EntrySettings, which the first entry of a ladder
// that opens raised is sized with. Nothing else reads them and nothing
// stores them. The trade keeps its own rows, so every tick raises the
// configured rows afresh, never the rows a tick before it raised.
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
