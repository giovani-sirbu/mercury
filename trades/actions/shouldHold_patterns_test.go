package actions

import (
	"github.com/giovani-sirbu/mercury/trades/gates/regime"
	"github.com/giovani-sirbu/mercury/trades/internal/testutil"
	"strings"
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func patternAI(direction string, score float64) aggragates.AIIndicators {
	return aggragates.AIIndicators{
		HasRegimeVerdict:   true,
		AddAllowed:         true,
		Regimes:            map[string]string{"4h": "mixed", "1h": "mixed", "15m": "mixed"},
		PatternName:        "asc_triangle",
		PatternDisplayName: "ascending triangle",
		PatternDirection:   direction,
		PatternScore:       score,
		PatternLevel:       96000,
		PatternLevelKind:   "resistance",
		PatternStopLoss:    94000,
		PatternTakeProfit:  104500,
		PatternInterval:    "15m",
	}
}

func patternEvent(position string, inverse bool, price float64, ai aggragates.AIIndicators) events.Events {
	trade := testutil.NewHoldTrade(position, inverse)
	trade.Strategy.Params.UsePatterns = true
	trade.PositionPrice = price
	side := "BUY"
	if inverse {
		side = "SELL"
	}
	trade.History = []aggragates.TradesHistory{{Type: side, Quantity: 1, Price: 100000, OrderId: 1}}
	return events.Events{
		Trade: trade,
		Events: map[string]func(events.Events) (events.Events, error){
			"updateTrade": testutil.NopUpdateTrade,
		},
		Params: aggragates.Params{OldPosition: "active", AIIndicators: ai},
	}
}

func TestPatternHoldPreventsStopLossOnBullishPattern(t *testing.T) {
	held, err := ShouldHold(patternEvent("stopLoss", false, 100000, patternAI("long", 71)))
	if err == nil {
		t.Fatal("a bullish pattern must prevent the long add")
	}
	want := "Hold stopLoss: pattern: ascending triangle found (resistance 96000.0000), preventing stopLoss"
	if held.Trade.Logs[0].Message != want {
		t.Errorf("got %q, want %q", held.Trade.Logs[0].Message, want)
	}
}

func TestPatternHoldStopLossPasses(t *testing.T) {
	cases := []struct {
		name    string
		inverse bool
		ai      aggragates.AIIndicators
	}{
		{"score under the floor", false, patternAI("long", 59.9)},
		{"pattern against the trade", false, patternAI("short", 90)},
		{"long pattern on an inverse trade", true, patternAI("long", 90)},
		{"no pattern", false, aggragates.AIIndicators{HasRegimeVerdict: true, AddAllowed: true}},
	}
	for _, c := range cases {
		if held, err := ShouldHold(patternEvent("stopLoss", c.inverse, 100000, c.ai)); err != nil {
			t.Fatalf("%s: must pass, got %v %v", c.name, err, messages(held.Trade.Logs))
		}
	}
}

func TestPatternHoldInverseMirror(t *testing.T) {
	ai := patternAI("short", 80)
	ai.PatternName, ai.PatternDisplayName, ai.PatternLevelKind = "head_shoulders", "head and shoulders", "neckline"
	ai.PatternTakeProfit = 90000
	held, err := ShouldHold(patternEvent("stopLoss", true, 100000, ai))
	if err == nil {
		t.Fatal("a bearish pattern must prevent the inverse add")
	}
	if held.Trade.Logs[0].Message != "Hold stopLoss: pattern: head and shoulders found (neckline 96000.0000), preventing stopLoss" {
		t.Errorf("unexpected message %q", held.Trade.Logs[0].Message)
	}

	riding, err := ShouldHold(patternEvent("takeProfit", true, 100000, ai))
	if err == nil {
		t.Fatal("an inverse exit above the pattern target must ride")
	}
	if riding.Trade.Logs[0].Message != "Hold takeProfit: pattern: head and shoulders in play, riding to target 90000.0000" {
		t.Errorf("unexpected message %q", riding.Trade.Logs[0].Message)
	}
}

func TestPatternHoldNeverOnFirstFill(t *testing.T) {
	for _, direction := range []string{"long", "short"} {
		event := patternEvent("buy", false, 100000, patternAI(direction, 90))
		event.Trade.History = nil
		event.Params.OldPosition = "new"
		if held, err := ShouldHold(event); err != nil || len(held.Trade.Logs) != 0 {
			t.Fatalf("%s pattern must not touch the first fill, got %v %v", direction, err, messages(held.Trade.Logs))
		}
	}
}

func TestPatternHoldTakeProfitRidesToTarget(t *testing.T) {
	held, err := ShouldHold(patternEvent("takeProfit", false, 100000, patternAI("long", 71)))
	if err == nil {
		t.Fatal("a long exit below the pattern target must ride")
	}
	if held.Trade.Logs[0].Message != "Hold takeProfit: pattern: ascending triangle in play, riding to target 104500.0000" {
		t.Errorf("unexpected message %q", held.Trade.Logs[0].Message)
	}

	if _, err := ShouldHold(patternEvent("takeProfit", false, 104500, patternAI("long", 71))); err != nil {
		t.Fatalf("at the target the exit must execute, got %v", err)
	}
	noTarget := patternAI("long", 71)
	noTarget.PatternTakeProfit = 0
	if _, err := ShouldHold(patternEvent("takeProfit", false, 100000, noTarget)); err != nil {
		t.Fatalf("without a target the exit must execute, got %v", err)
	}
	if _, err := ShouldHold(patternEvent("takeProfit", false, 100000, patternAI("short", 90))); err != nil {
		t.Fatalf("a pattern against the trade must release the exit, got %v", err)
	}
}

func TestPatternHoldIsOwnedByUsePatterns(t *testing.T) {
	event := patternEvent("stopLoss", false, 100000, patternAI("long", 90))
	event.Trade.Strategy.Params.UsePatterns = false
	if held, err := ShouldHold(event); err != nil || len(held.Trade.Logs) != 0 {
		t.Fatalf("with UsePatterns off the pattern must not hold, got %v %v", err, messages(held.Trade.Logs))
	}
}

// The regime lens runs first: when both would hold, the regime wording is
// the one on record.
func TestPatternHoldYieldsToRegime(t *testing.T) {
	ai := patternAI("long", 90)
	ai.AddAllowed = false
	ai.Regimes = map[string]string{"4h": regime.DownPersist, "1h": "mixed", "15m": "mixed"}
	event := patternEvent("stopLoss", false, 100000, ai)
	event.Trade.Strategy.Params.RegimeHold = true
	held, err := ShouldHold(event)
	if err == nil {
		t.Fatal("expected a hold")
	}
	if !strings.HasPrefix(held.Trade.Logs[0].Message, "Hold stopLoss: regime: add not allowed") {
		t.Errorf("regime must keep its wording over the pattern, got %q", held.Trade.Logs[0].Message)
	}
}

func TestPatternHoldMessageFallbacks(t *testing.T) {
	ai := patternAI("long", 71)
	ai.PatternDisplayName = ""
	ai.PatternLevel = 0
	held, err := ShouldHold(patternEvent("stopLoss", false, 100000, ai))
	if err == nil {
		t.Fatal("expected a hold")
	}
	if held.Trade.Logs[0].Message != "Hold stopLoss: pattern: asc_triangle found, preventing stopLoss" {
		t.Errorf("unexpected message %q", held.Trade.Logs[0].Message)
	}

	withFilter := patternEvent("stopLoss", false, 100000, patternAI("long", 71))
	withFilter.Trade.StrategyPair.TradeFilters = aggragates.TradeFilters{PriceFilter: 2}
	held, err = ShouldHold(withFilter)
	if err == nil {
		t.Fatal("expected a hold")
	}
	if !strings.Contains(held.Trade.Logs[0].Message, "(resistance 96000.00)") {
		t.Errorf("the level must use the pair's price precision, got %q", held.Trade.Logs[0].Message)
	}
}
