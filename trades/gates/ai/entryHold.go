// Package ai is the UseAI flag's gates: the ML route's bullish/bearish veto
// on the first fill and on an open position. Every gate here takes the
// verdict as `ai aggragates.AIIndicators`, so a caller that imports this
// package must not name a local `ai` — shouldHold.go calls it `indicators`.
package ai

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// EntryHold is the UseAI flag's first-fill veto: the ML route's
// bullish/bearish read against the direction the entry would take.
//
// `side` is aggragates.EntrySide, not the Inverse flag. The two agree on spot
// and disagree on futures, where the verdict IS the direction: taking Inverse
// here read a futures SHORT as bearish and vetoed the entry the same verdict
// was asking for.
//
// An empty side is no direction to judge, so nothing is held. The engines
// refuse a directionless futures entry before the chain is built.
func EntryHold(side string, ai aggragates.AIIndicators) string {
	isBearishSignal := ai.AIMarketBearish || ai.AIAction == aggragates.ActionShort
	isBullishSignal := ai.AIMarketBullish || ai.AIAction == aggragates.ActionLong

	switch side {
	case aggragates.SideLong:
		if isBearishSignal {
			return "AI market is bearish"
		}
	case aggragates.SideShort:
		if isBullishSignal {
			return "AI market is bullish"
		}
	}

	return ""
}
