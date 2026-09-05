// Package ai is the UseAI flag's gates: the ML route's bullish/bearish veto
// on the first fill and on an open position. Every gate here takes the
// verdict as `ai aggragates.AIIndicators`, so a caller that imports this
// package must not name a local `ai` — shouldHold.go calls it `indicators`.
package ai

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// EntryHold is the UseAI flag's first-fill veto: the ML route's
// bullish/bearish read against the trade's direction.
func EntryHold(inverse bool, ai aggragates.AIIndicators) string {
	isBearishSignal := ai.AIMarketBearish || ai.AIAction == aggragates.ActionShort
	isBullishSignal := ai.AIMarketBullish || ai.AIAction == aggragates.ActionLong
	if !inverse && isBearishSignal {
		return "AI market is bearish"
	}
	if inverse && isBullishSignal {
		return "AI market is bullish"
	}
	return ""
}
