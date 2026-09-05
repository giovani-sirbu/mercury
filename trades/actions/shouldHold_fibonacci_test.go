package actions

import (
	"testing"

	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
)

// A structural pattern outranks the fibonacci wait, and fibonacci only
// runs under UsePatterns on a stopLoss.
func TestFibonacciHoldPrecedenceAndOwnership(t *testing.T) {
	ai := testutil.FibAI()
	ai.PatternName, ai.PatternDisplayName, ai.PatternDirection, ai.PatternScore = "bull_flag", "bull flag", "long", 80
	event := patternEvent("stopLoss", false, 106.5, ai)
	event.Trade.StrategyPair.TradeFilters = aggragates.TradeFilters{PriceFilter: 2}
	held, err := ShouldHold(event)
	if err == nil || held.Trade.Logs[0].Message != "Hold stopLoss: pattern: bull flag found, preventing stopLoss" {
		t.Fatalf("the pattern must outrank the fibonacci wait, got %v %v", err, messages(held.Trade.Logs))
	}

	fibOnly := patternEvent("stopLoss", false, 106.5, testutil.FibAI())
	fibOnly.Trade.StrategyPair.TradeFilters = aggragates.TradeFilters{PriceFilter: 2}
	held, err = ShouldHold(fibOnly)
	if err == nil || held.Trade.Logs[0].Message != "Hold stopLoss: fibonacci: waiting for a better price (next level 106.18)" {
		t.Fatalf("fibonacci must hold the rung, got %v %v", err, messages(held.Trade.Logs))
	}

	off := patternEvent("stopLoss", false, 106.5, testutil.FibAI())
	off.Trade.Strategy.Params.UsePatterns = false
	if held, err := ShouldHold(off); err != nil || len(held.Trade.Logs) != 0 {
		t.Fatalf("fibonacci is UsePatterns' alone, got %v %v", err, messages(held.Trade.Logs))
	}

	exit := patternEvent("takeProfit", false, 106.5, testutil.FibAI())
	if held, err := ShouldHold(exit); err != nil || len(held.Trade.Logs) != 0 {
		t.Fatalf("fibonacci never touches an exit, got %v %v", err, messages(held.Trade.Logs))
	}
}
