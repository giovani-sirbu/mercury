package actions_test

import (
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/actions"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// patternHoldTrade is an open BTC/USDC long two fills deep, quoted with the
// scenario's real market filters (2 price decimals), so the hold messages
// render levels exactly as the pair quotes them.
func patternHoldTrade(positionType string, positionPrice float64, usePatterns bool) aggragates.Trades {
	trade := scenarioBuildTrade(positionType, positionPrice, false)
	trade.Strategy.Params = aggragates.StrategyParams{UsePatterns: usePatterns}
	scenarioAppendHistory(&trade, "BUY", 0.01, 100000, "", 0)
	scenarioAppendHistory(&trade, "BUY", 0.02, 98000, "", 0)
	return trade
}

func patternHoldEvent(trade aggragates.Trades, ai aggragates.AIIndicators) events.Events {
	return events.Events{
		Trade:  trade,
		Events: map[string]func(events.Events) (events.Events, error){"updateTrade": EmptyUpdateTrade},
		Params: aggragates.Params{OldPosition: trade.PositionType, AIIndicators: ai},
	}
}

func ascendingTriangle() aggragates.AIIndicators {
	return aggragates.AIIndicators{
		PatternName:        "asc_triangle",
		PatternDisplayName: "ascending triangle",
		PatternDirection:   "long",
		PatternScore:       71,
		PatternLevel:       96000,
		PatternLevelKind:   "resistance",
		PatternTakeProfit:  104500,
		PatternLens:        "15m",
	}
}

func lastHoldRow(t *testing.T, event events.Events) string {
	t.Helper()
	held, err := actions.ShouldHold(event)
	if err == nil {
		t.Fatal("expected a hold")
	}
	if len(held.Trade.Logs) == 0 {
		t.Fatalf("hold %v wrote no log row", err)
	}
	return held.Trade.Logs[len(held.Trade.Logs)-1].Message
}

// The pattern verdict sophos serves on /patterns reaches the trade log as the
// operator reads it: the human pattern name, the structural level in the
// pair's price format, and the rung it prevented.
func TestPatternHold_AscendingTriangleBlocksTheRung(t *testing.T) {
	msg := lastHoldRow(t, patternHoldEvent(patternHoldTrade("stopLoss", 96500, true), ascendingTriangle()))
	want := "Hold stopLoss: pattern: ascending triangle found (resistance 96000.00), preventing stopLoss"
	if msg != want {
		t.Fatalf("got %q, want %q", msg, want)
	}
}

func TestPatternHold_AscendingTriangleRidesToTarget(t *testing.T) {
	msg := lastHoldRow(t, patternHoldEvent(patternHoldTrade("takeProfit", 100500, true), ascendingTriangle()))
	want := "Hold takeProfit: pattern: ascending triangle in play, riding to target 104500.00"
	if msg != want {
		t.Fatalf("got %q, want %q", msg, want)
	}

	// Past the measured target the exit is released: the release IS the sell.
	reached := patternHoldEvent(patternHoldTrade("takeProfit", 104600, true), ascendingTriangle())
	if _, err := actions.ShouldHold(reached); err != nil {
		t.Fatalf("a target already reached must release the exit, got %v", err)
	}
}

func TestPatternHold_FibonacciWaitsForABetterPrice(t *testing.T) {
	fib := aggragates.AIIndicators{
		FibSwingLow:  100000,
		FibSwingHigh: 110000,
		FibLevels:    []float64{106180, 105000, 103820, 102140},
	}
	msg := lastHoldRow(t, patternHoldEvent(patternHoldTrade("stopLoss", 106500, true), fib))
	want := "Hold stopLoss: fibonacci: waiting for a better price (next level 106180.00)"
	if msg != want {
		t.Fatalf("got %q, want %q", msg, want)
	}

	// Within the tolerance band of the level the rung may arm.
	atLevel := patternHoldEvent(patternHoldTrade("stopLoss", 106300, true), fib)
	if _, err := actions.ShouldHold(atLevel); err != nil {
		t.Fatalf("a price at the level must release the rung, got %v", err)
	}
}

// The whole family answers to the flag: the same verdicts hold nothing when
// usePatterns is off, and never on a first fill.
func TestPatternHold_OwnedByUsePatterns(t *testing.T) {
	off := patternHoldEvent(patternHoldTrade("stopLoss", 96500, false), ascendingTriangle())
	if held, err := actions.ShouldHold(off); err != nil {
		t.Fatalf("usePatterns off must hold nothing, got %q", held.Trade.Logs[len(held.Trade.Logs)-1].Message)
	}

	first := scenarioBuildTrade("buy", 96500, false)
	first.Strategy.Params = aggragates.StrategyParams{UsePatterns: true}
	entry := patternHoldEvent(first, ascendingTriangle())
	entry.Params.OldPosition = "new"
	if held, err := actions.ShouldHold(entry); err != nil {
		t.Fatalf("a pattern must never gate the first fill, got %q", held.Trade.Logs[len(held.Trade.Logs)-1].Message)
	}
}
