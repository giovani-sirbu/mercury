package strategies

import (
	"fmt"
	"sync"

	"github.com/Knetic/govaluate"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// logicExpressions caches compiled logic by its source string. GetPosition
// runs for every live trade on every price print — millions of times in a
// replay — and re-parsed the same fixed string on each one, which cost around
// 25x the evaluation itself and was the dominant allocation source of a run.
// The strings come from the engines' own GetLogic() literals, so the cache is
// bounded (a dozen entries) and never sees user input.
var logicExpressions sync.Map

// logicExpression returns the compiled form of logic, parsing each distinct
// string at most once. A string govaluate cannot parse caches as a nil
// expression, which keeps the existing contract byte for byte: the parse
// error was already discarded here, and evaluating a nil expression panics.
func logicExpression(logic string) *govaluate.EvaluableExpression {
	if cached, ok := logicExpressions.Load(logic); ok {
		expression, _ := cached.(*govaluate.EvaluableExpression)
		return expression
	}

	expression, _ := govaluate.NewEvaluableExpression(logic)
	logicExpressions.Store(logic, expression)

	return expression
}

type (
	Position struct {
		Type  string
		Price float64
	}
	Strategy struct {
		Type     string
		Position Position
		Price    float64
		Settings []aggragates.StrategySettings
		Logic    map[string]string
		Depth    int32
	}
)

// GetPosition get the new position based on a strategy logic
func (S Strategy) GetPosition(percentage float64) string {
	if len(S.Settings) < 1 {
		return ""
	}

	expression := logicExpression(S.Logic[S.Position.Type])

	// Row-per-depth contract: the held depth's own row when it exists, the
	// base row 0 otherwise — so a single-row ladder applies to every depth.
	settingsIndex := int(S.Depth)
	if settingsIndex < 0 || settingsIndex >= len(S.Settings) {
		settingsIndex = 0
	}

	parameters := make(map[string]interface{}, 8)
	parameters["percentage"] = percentage
	parameters["tradePercentage"] = S.Settings[settingsIndex].Percentage
	parameters["tolerance"] = S.Settings[settingsIndex].Tolerance
	parameters["trailingTakeProfit"] = S.Settings[settingsIndex].TrailingTakeProfit
	parameters["takeLossPercentage"] = S.Settings[settingsIndex].TakeLossPercentage

	result, _ := expression.Evaluate(parameters)
	newPosition := fmt.Sprintf("%s", result)

	return newPosition
}

// GetPercentage get percentage between old and new price
func (S Strategy) GetPercentage(price float64) float64 {
	return ((price - S.Position.Price) / price) * 100
}
