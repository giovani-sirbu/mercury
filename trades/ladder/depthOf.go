package ladder

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// DepthOf is the wallet view of one trade: what it is, how deep its ladder
// has filled, how deep it may fill, and what the depths it has left still
// cost in the asset it spends — priced from its position, which is what the
// wallet is kept for, and from its last fill, which is what level ladders are
// ranked on.
//
// The ceiling and both costs are measured on the rows the ladder TRADES
// (tradedSettings): a ladder that opened with a raise reads the raised rows,
// the ones its first entry was sized for, so it is not full at the stored
// ceiling and keeps the wallet for the depths its opened row added. The trade
// carries that raise in its own logs, which is why the view needs nothing but
// the trade — the two sisyphus engines from their own memory and agora from
// the database read it alike.
//
// Every surface that builds that view maps through this one helper — both
// sisyphus engines from their own memory, agora from the database for hermes
// — so a replay, a paper run and production cannot disagree about what a
// depth is or what one is worth. The alternative, each surface counting
// entries its own way, is how a gate that holds on one engine silently lets
// the same ladder through on another.
func DepthOf(trade aggragates.Trades) aggragates.LadderDepth {
	// Counted and read once, then handed to both costs: the wallet view is
	// built for every ladder of a wallet on every gated tick, and folding the
	// same history, or the same logs, once per field is the kind of cost that
	// only shows up as a slow engine.
	settings := tradedSettings(trade)
	filled := CountFilledEntries(trade)
	ceiling := ceilingOf(settings, filled)

	asset, remainingCost := remainingCostAt(trade, settings, filled, ceiling)

	return aggragates.LadderDepth{
		TradeID:              trade.ID,
		Symbol:               trade.Symbol,
		Depth:                filled,
		MaxDepth:             ceiling,
		Asset:                asset,
		RemainingCost:        remainingCost,
		PlannedRemainingCost: plannedRemainingCostAt(trade, settings, filled, ceiling),
	}
}
