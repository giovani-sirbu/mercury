package dynamicparams

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// Applies reports whether the flag shapes a trade's rows: DynamicParams on, a
// long ladder, a spot strategy and a parent — the ladders the smart take
// loss's capital protection and slow-decline hold watch. An inverse ladder, a
// futures strategy and an impasse child, which belongs to its parent's
// impasse chain, keep their configured rows.
func Applies(trade aggragates.Trades) bool {
	return trade.Strategy.Params.DynamicParams &&
		!trade.Inverse &&
		trade.Strategy.TradeType != aggragates.Futures &&
		trade.ParentID == 0
}
