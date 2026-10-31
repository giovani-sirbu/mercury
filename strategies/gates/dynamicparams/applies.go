package dynamicparams

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// Applies is whether the flag shapes a trade's rows: DynamicParams on and a long spot parent ladder.
func Applies(trade aggragates.Trades) bool {
	return trade.Strategy.Params.DynamicParams &&
		!trade.Inverse &&
		trade.Strategy.TradeType != aggragates.Futures &&
		trade.ParentID == 0
}
