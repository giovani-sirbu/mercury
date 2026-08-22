package strategies

import (
	"fmt"
	"sync"
	"testing"

	"github.com/Knetic/govaluate"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// spotLogic mirrors the engines' GetLogic() literals closely enough to drive
// every branch GetPosition can take.
func spotLogic() map[string]string {
	return map[string]string{
		"new":        "'buy'",
		"buy":        "percentage <= -tradePercentage-tolerance ? 'stopLoss' : (percentage >= tradePercentage+tolerance ? 'takeProfit' : '')",
		"sell":       "percentage < -tradePercentage-tolerance ? 'update_buy' : ''",
		"takeProfit": "percentage < -tradePercentage-tolerance ? 'update_buy' : (percentage < -tolerance ? 'sell' : (percentage > trailingTakeProfit ? 'update_takeProfit' : ''))",
		"stopLoss":   "percentage > tradePercentage+tolerance ? 'update_buy' : (percentage > tolerance ? 'buy' : (percentage < -trailingTakeProfit-tolerance ? 'update_stopLoss' : ''))",
		"impasse":    "'impasse'",
	}
}

func testStrategy(positionType string) Strategy {
	return Strategy{
		Position: Position{Type: positionType, Price: 100},
		Logic:    spotLogic(),
		Settings: []aggragates.StrategySettings{
			{Percentage: 2, Tolerance: 0.25, TrailingTakeProfit: 1, TakeLossPercentage: 10},
		},
	}
}

// uncachedPosition is GetPosition as it was before the cache: parse per call.
// The cached path must answer identically for every state and percentage.
func uncachedPosition(S Strategy, percentage float64) string {
	if len(S.Settings) < 1 {
		return ""
	}
	expression, _ := govaluate.NewEvaluableExpression(S.Logic[S.Position.Type])
	parameters := map[string]interface{}{
		"percentage":         percentage,
		"tradePercentage":    S.Settings[0].Percentage,
		"tolerance":          S.Settings[0].Tolerance,
		"trailingTakeProfit": S.Settings[0].TrailingTakeProfit,
		"takeLossPercentage": S.Settings[0].TakeLossPercentage,
	}
	result, _ := expression.Evaluate(parameters)
	return fmt.Sprintf("%s", result)
}

func TestLogicExpressionCacheMatchesUncachedParse(t *testing.T) {
	percentages := []float64{-50, -5, -2.25, -2, -1, -0.25, 0, 0.25, 1, 2, 2.25, 5, 50}

	for state := range spotLogic() {
		strategy := testStrategy(state)
		for _, percentage := range percentages {
			got := strategy.GetPosition(percentage)
			want := uncachedPosition(strategy, percentage)
			if got != want {
				t.Fatalf("state %q at %v: cached gave %q, per-call parse gives %q",
					state, percentage, got, want)
			}
		}
	}
}

// The cache is process-wide and read from every trade goroutine, so a shared
// compiled expression must survive concurrent evaluation.
func TestLogicExpressionCacheIsRaceFree(t *testing.T) {
	const goroutines = 32
	strategy := testStrategy("buy")

	var wg sync.WaitGroup
	results := make([]string, goroutines)
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(slot int) {
			defer wg.Done()
			results[slot] = strategy.GetPosition(-5)
		}(i)
	}
	wg.Wait()

	for slot, got := range results {
		if got != "stopLoss" {
			t.Fatalf("goroutine %d saw %q, want stopLoss", slot, got)
		}
	}
}

// A logic string govaluate cannot parse must behave exactly as before the
// cache: the discarded parse error leaves a nil expression and evaluating it
// panics. Pinned so the cache is never blamed for a pre-existing contract.
func TestLogicExpressionCachePreservesMalformedContract(t *testing.T) {
	strategy := testStrategy("broken")
	strategy.Logic["broken"] = "this is ( not valid"

	defer func() {
		if recover() == nil {
			t.Fatal("malformed logic must still panic, as it did before the cache")
		}
	}()

	strategy.GetPosition(1)
}
