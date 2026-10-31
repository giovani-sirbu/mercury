package smartsell

import (
	"github.com/giovani-sirbu/mercury/strategies/gates/smartsell/quietdecline"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// quietDeclineEligible is the part of the watch the fills do not decide: quietdecline.Enabled and a
// long spot ladder.
func quietDeclineEligible(trade aggragates.Trades) bool {
	return quietDecline && !trade.Inverse && trade.Strategy.TradeType != aggragates.Futures
}

// quietDeclineWatched is whether the quiet decline watches a trade: quietDeclineEligible and at
// least quietdecline.ArmDepth filled entries.
func quietDeclineWatched(trade aggragates.Trades) bool {
	return quietDeclineEligible(trade) && ladder.CountFilledEntries(trade) >= quietdecline.ArmDepth
}

// quietDeclineGoesPending is the reading that makes a watched ladder pending on the quiet decline:
// not pending, a depth bought within the fill window (recentFill), and the last closed bar reading
// for it (quietDeclineReadsForTheLadder) or the look-back marking it (quietDeclinePendsRecently).
func quietDeclineGoesPending(st state, block aggragates.SmartSellIndicators) ([]string, bool) {
	if !st.quietDeclineWatched || st.slowDeclinePending || !recentFill(st.lastFill(), block) {
		return nil, false
	}
	if quietDeclineReadsForTheLadder(st.lastFill(), block) {
		return block.QuietDeclineExitReasons, true
	}
	if quietDeclinePendsRecently(st, block) {
		return block.QuietDeclineRecentReasons, true
	}
	return nil, false
}

// quietDeclineReadsForTheLadder is quietdecline.ReadsForTheLadder on the newest fill's stamp.
func quietDeclineReadsForTheLadder(newest entryFill, block aggragates.SmartSellIndicators) bool {
	return quietdecline.ReadsForTheLadder(newest.At, block)
}

// quietDeclineConfirms is quietdecline.Confirms on the new fill's stamp.
func quietDeclineConfirms(newest entryFill, block aggragates.SmartSellIndicators) ([]string, bool) {
	return quietdecline.Confirms(newest.At, block)
}
