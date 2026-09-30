package ladder

import (
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/gates/dynamicparams"
)

// tradedSettings is the rows the trade's ladder TRADES, which is what every
// reading of its depths is measured against: the stored rows, unless the
// DynamicParams flag shapes the trade and its opened row raises them
// (dynamicparams.RaisedSettings), and then the raised rows.
//
// A ladder that opened raised has a first entry sized for the raised depths
// and places its adds up to the raised ceiling, so its ceiling, the reserve
// for the depths it has left and the price step of each of them are the raised
// rows' — measured against the stored rows it would read as full at the depth
// its own grid was planned to go past, and reserve nothing for its largest
// entries. The raise is read off the trade's own logs, so the engine that
// ticks the ladder and every surface that builds the wallet view from the
// trade (sisyphus from memory, agora from the database) read the same rows.
//
// A trade without an opened row, one the flag does not shape, an inverse
// ladder and an impasse child answer the stored slice itself, unchanged.
//
// The sizing readers — SettingsIndexOrBase, CalculateInitialBid,
// GetInitialBidByDepth and NextEntryCost — are not routed through here: the
// engines hand them the rows the entry is placed on.
func tradedSettings(trade aggragates.Trades) []aggragates.StrategySettings {
	rows, _ := dynamicparams.RaisedSettings(trade)

	return rows
}
