package ladder

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// DepthOf is the wallet view of one trade: what it is, how deep its ladder
// has filled, how deep it may fill, and what the depths it has left still
// cost in the asset it spends.
//
// Every surface that builds that view maps through this one helper — both
// sisyphus engines from their own memory, agora from the database for hermes
// — so a replay, a paper run and production cannot disagree about what a
// depth is or what one is worth. The alternative, each surface counting
// entries its own way, is how a gate that holds on one engine silently lets
// the same ladder through on another.
func DepthOf(trade aggragates.Trades) aggragates.LadderDepth {
	// Counted and read once, then handed to the cost: the wallet view is
	// built for every ladder of a wallet on every gated tick, and folding the
	// same history once per field is the kind of cost that only shows up as a
	// slow engine.
	filled := CountFilledEntries(trade)
	ceiling := ConfiguredDepths(trade)

	asset, remainingCost := remainingCostAt(trade, filled, ceiling)

	return aggragates.LadderDepth{
		TradeID:       trade.ID,
		Symbol:        trade.Symbol,
		Depth:         filled,
		MaxDepth:      ceiling,
		Asset:         asset,
		RemainingCost: remainingCost,
	}
}
