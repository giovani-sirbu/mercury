package dynamicparams

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// RaisedSettings is the one call each engine makes per tick: the rows the
// reads raise and true while the flag applies to the trade (Applies) and the
// reads raise a row (Adjust); otherwise the trade's stored rows — the very
// slice — and false, and the engine changes nothing.
//
// On true the engine hands the rows to exactly two places: the
// strategies.Strategy.Settings it computes the position from, and the chain's
// aggragates.Params.EntrySettings, which the first entry of a ladder that
// opens while raised is sized with. Nothing else reads them and nothing
// stores them. The trade keeps its own rows, so every tick raises the
// configured rows afresh, never the rows a tick before it raised.
func RaisedSettings(trade aggragates.Trades, reads aggragates.DynamicParamsIndicators) ([]aggragates.StrategySettings, bool) {
	stored := trade.StrategyPair.StrategySettings
	if !Applies(trade) {
		return stored, false
	}

	increase := Raise(TierOf(reads))
	if len(stored) == 0 || !increase.changesRows() {
		return stored, false
	}

	return raiseRows(stored, increase), true
}
