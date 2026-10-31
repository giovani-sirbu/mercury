// Package patterns is the UsePatterns flag's gates: the chart-pattern holds on a trailing buy and a
// take profit (PositionHold) and the fibonacci wait for a better price (fibonacciTrailingBuyHold).
package patterns

import (
	"fmt"

	"github.com/giovani-sirbu/mercury/helpers"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// HoldMinScore is the detector score a chart pattern needs to hold a depth.
const HoldMinScore = 60.0

// patternInFavor is a scored pattern whose detectors match the trade's direction.
func patternInFavor(ai aggragates.AIIndicators, inverse bool) bool {
	want := "long"
	if inverse {
		want = "short"
	}
	return ai.PatternName != "" && ai.PatternDirection == want && ai.PatternScore >= HoldMinScore
}

func patternTrailingBuyHold(trade aggragates.Trades, ai aggragates.AIIndicators) string {
	if !patternInFavor(ai, trade.Inverse) {
		return ""
	}
	// Detected means the breakout is live on this bar; no price comparison.
	if ai.PatternLevel > 0 && ai.PatternLevelKind != "" {
		return fmt.Sprintf("pattern: %s found (%s %s), preventing the trailing buy",
			patternLabel(ai), ai.PatternLevelKind, helpers.FormatPriceLevel(trade, ai.PatternLevel))
	}
	return fmt.Sprintf("pattern: %s found, preventing the trailing buy", patternLabel(ai))
}

func patternTakeProfitHold(trade aggragates.Trades, ai aggragates.AIIndicators) string {
	if !patternInFavor(ai, trade.Inverse) {
		return ""
	}
	if ai.PatternTakeProfit <= 0 || trade.PositionPrice <= 0 {
		return ""
	}
	targetAhead := trade.PositionPrice < ai.PatternTakeProfit
	if trade.Inverse {
		targetAhead = trade.PositionPrice > ai.PatternTakeProfit
	}
	if !targetAhead {
		return ""
	}
	return fmt.Sprintf("pattern: %s in play, riding to target %s",
		patternLabel(ai), helpers.FormatPriceLevel(trade, ai.PatternTakeProfit))
}

// patternLabel is the display name sophos serves, else the detector id.
func patternLabel(ai aggragates.AIIndicators) string {
	if ai.PatternDisplayName != "" {
		return ai.PatternDisplayName
	}
	return ai.PatternName
}
