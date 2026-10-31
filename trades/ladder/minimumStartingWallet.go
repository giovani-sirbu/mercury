package ladder

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// MinimumStartingWallet is the smallest wallet, in the asset the ladder
// spends, that CalculateInitialBid admits: the figure shown to a user whose
// trade was refused as too small. It is derived from CalculateInitialBid
// itself: GetInitialBidByDepth is linear in the amount, so a probe of 1
// settles on the shallowest configured ladder, the easiest to clear, and the
// minimum notional over that bid is the wallet that clears it.
func MinimumStartingWallet(trade aggragates.Trades) float64 {
	unitBid, _ := CalculateInitialBid(1, trade, 0)

	unitInQuote := unitBid
	if trade.Inverse {
		unitInQuote *= trade.PositionPrice
	}
	if unitInQuote <= 0 {
		return 0
	}

	return trade.StrategyPair.TradeFilters.MinNotional / unitInQuote
}
