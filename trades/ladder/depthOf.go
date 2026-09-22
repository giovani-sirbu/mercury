package ladder

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// DepthOf is the wallet view of one trade: what it is, how deep its ladder
// has filled and how deep it may fill.
//
// Every surface that builds that view maps through this one helper — both
// sisyphus engines from their own memory, agora from the database for hermes
// — so a replay, a paper run and production cannot disagree about what a
// depth is. The alternative, each surface counting entries its own way, is
// how a gate that holds on one engine silently lets the same ladder through
// on another.
func DepthOf(trade aggragates.Trades) aggragates.LadderDepth {
	return aggragates.LadderDepth{
		TradeID:  trade.ID,
		Symbol:   trade.Symbol,
		Depth:    CountFilledEntries(trade),
		MaxDepth: ConfiguredDepths(trade),
	}
}
