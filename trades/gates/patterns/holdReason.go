// Package patterns is the UsePatterns flag's gates: the chart-pattern holds
// on a stop loss and a take profit, and the fibonacci "wait for a better
// price" on a stop loss. Fibonacci and the pattern text share one package
// because each needs the other's helpers.
package patterns

import (
	"fmt"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// HoldMinScore is the detector score a chart pattern needs before it
// may hold a rung. Detector scores average three parts, one of which is a
// constant 1, so 60 means the two real geometry parts sum to at least 0.8.
const HoldMinScore = 60.0

// HoldReason is the UsePatterns flag's gate on an open position.
// It reads the 15m chart-pattern verdict sophos serves on /patterns:
//
//	stopLoss   a pattern IN the trade's direction (bullish for a long) says
//	           "price should bounce": do not average down into it. When no
//	           pattern holds, the fibonacci retracement may still ask for a
//	           better rung price.
//	takeProfit the same pattern with its measured target still ahead keeps
//	           the exit deferred; anything else — and any pattern AGAINST
//	           the trade — releases the close, which is the sell.
//
// Never on the first fill: that is the cooldown's. A pattern is never
// bypassed by capitulation (its reasons carry no regime prefix): "price
// should bounce" and "take one extra fill on the reclaim" contradict.
func HoldReason(event events.Events, position string, ai aggragates.AIIndicators) string {
	if event.Params.OldPosition == "new" {
		return ""
	}
	switch position {
	case "stopLoss":
		if reason := patternStopLossHold(event.Trade, ai); reason != "" {
			return reason
		}
		return fibonacciStopLossHold(event.Trade, ai)
	case "takeProfit":
		return patternTakeProfitHold(event.Trade, ai)
	}
	return ""
}

// patternInFavor: a scored pattern whose detector set matches the trade's
// direction (long detectors for a spot long, short detectors for inverse).
func patternInFavor(ai aggragates.AIIndicators, inverse bool) bool {
	want := "long"
	if inverse {
		want = "short"
	}
	return ai.PatternName != "" && ai.PatternDirection == want && ai.PatternScore >= HoldMinScore
}

func patternStopLossHold(trade aggragates.Trades, ai aggragates.AIIndicators) string {
	if !patternInFavor(ai, trade.Inverse) {
		return ""
	}
	// No price comparison: every detector requires the last close beyond
	// its level, so "detected" means the breakout is live on this bar.
	if ai.PatternLevel > 0 && ai.PatternLevelKind != "" {
		return fmt.Sprintf("pattern: %s found (%s %s), preventing stopLoss",
			patternLabel(ai), ai.PatternLevelKind, formatPriceLevel(trade, ai.PatternLevel))
	}
	return fmt.Sprintf("pattern: %s found, preventing stopLoss", patternLabel(ai))
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
		patternLabel(ai), formatPriceLevel(trade, ai.PatternTakeProfit))
}

// patternLabel is the human name sophos serves, falling back to the
// detector id for an older payload.
func patternLabel(ai aggragates.AIIndicators) string {
	if ai.PatternDisplayName != "" {
		return ai.PatternDisplayName
	}
	return ai.PatternName
}
