package ladder

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// MinimumStartingWallet is the smallest wallet, in the asset the ladder spends
// (quote for a long, base for an inverse), that CalculateInitialBid will
// admit — the number to put in front of a user whose trade was refused for
// being too small.
//
// It is derived FROM CalculateInitialBid rather than modelled beside it. The
// figure shown used to come from CalculateMinimumQuantity, which sums the
// depths with its own doubling and a flat 5% margin and applies no wallet
// reserve at all, so it answered a different question from the one the gate
// asks: depositing exactly the amount reported was still refused, and the
// gap grew with the reserve.
//
// GetInitialBidByDepth is linear in the amount, so one probe gives the whole
// curve: the bid a wallet of 1 buys at the shallowest configured ladder —
// which is the easiest to clear, and therefore the one that decides whether
// a trade may start at all — scales straight up to the minimum notional. A
// probe of 1 can never clear MinNotional, so CalculateInitialBid always
// settles on that shallowest ladder and returns its bid alongside the error.
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
