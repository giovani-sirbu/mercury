// Package ai is the UseAI flag's gates: the ML route's bullish/bearish veto on the first fill
// (EntryHold) and on an open position (LegacyHoldReason). The verdict parameter is named ai, so a
// caller must not shadow it (shouldHold.go calls it indicators).
package ai

import "github.com/giovani-sirbu/mercury/trades/aggragates"

// EntryHold is the first-fill veto: the ML read against side (aggragates.EntrySide, the verdict's own
// direction on futures, so never the Inverse flag); an empty side holds nothing.
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
