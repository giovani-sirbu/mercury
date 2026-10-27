package ladder

import (
	"github.com/giovani-sirbu/mercury/strategies/gates/dynamicparams"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// tradedSettings is the rows the trade's ladder TRADES, which every reading
// of its depths is measured against: the stored rows, or the raised rows when
// the DynamicParams flag shapes the trade and its opened event raises them
// (dynamicparams.RaisedSettings). The raise is read off the trade's strategy
// events, never its log text, so the engine and every wallet view read the
// same rows. The sizing readers (CalculateInitialBid, NextEntryCost) are not
// routed through here: the engines hand them the rows the entry is placed on.
func tradedSettings(trade aggragates.Trades) []aggragates.StrategySettings {
	rows, _ := dynamicparams.RaisedSettings(trade)

	return rows
}
