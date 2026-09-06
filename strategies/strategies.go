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
		// AverageEntryPrice is the ladder's break even: what the position cost
		// per unit of base across every entry fill (ladder.AverageEntryPrice).
		// The upside branches of the `buy` rows read `profitPercentage`, the
		// move measured against it, so a take profit is proposed at break even
		// + percentage + tolerance at every depth. Price — the last fill or the
		// last re-anchor — keeps anchoring `percentage`: the downside (stopLoss
		// spacing) and every armed state. At depth 1 the two coincide, so the
		// first depth behaves exactly as before. Zero (no fills yet) makes
		// profitPercentage fall back to percentage.
		AverageEntryPrice float64
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

// GetPosition get the new position based on a strategy logic. percentage is
// the move against Position.Price, profitPercentage the move against
// Position.AverageEntryPrice (GetPercentage / GetProfitPercentage, both
// negated by the engines for an inverse trade).
func (S Strategy) GetPosition(percentage float64, profitPercentage float64) string {
	if len(S.Settings) < 1 {
		return ""
	}

	logic, ok := S.Logic[S.Position.Type]
	if !ok {
		// A position type this engine's logic map does not know (a state
		// migrated from another engine, a hand-edited row) fails closed:
		// evaluating the nil expression of "" panics, and in hermes that
		// panic ran in an unrecovered goroutine.
		return ""
	}
	expression := logicExpression(logic)

	// Row-per-depth contract: the held depth's own row when it exists, the
	// base row 0 otherwise — so a single-row ladder applies to every depth.
	settingsIndex := int(S.Depth)
	if settingsIndex < 0 || settingsIndex >= len(S.Settings) {
		settingsIndex = 0
	}

	parameters := make(map[string]interface{}, 5)
	parameters["percentage"] = percentage
	parameters["profitPercentage"] = profitPercentage
	parameters["tradePercentage"] = S.Settings[settingsIndex].Percentage
	parameters["tolerance"] = S.Settings[settingsIndex].Tolerance
	parameters["trailingTakeProfit"] = S.Settings[settingsIndex].TrailingTakeProfit

	result, _ := expression.Evaluate(parameters)
	newPosition := fmt.Sprintf("%s", result)

	return newPosition
}

// GetPercentage get percentage between old and new price
func (S Strategy) GetPercentage(price float64) float64 {
	return ((price - S.Position.Price) / price) * 100
}

// GetProfitPercentage is the same metric measured against the average entry
// price — how far the whole position is from its break even. Without entries
// there is no break even yet and the last fill stands in, so the value equals
// GetPercentage.
func (S Strategy) GetProfitPercentage(price float64) float64 {
	if S.Position.AverageEntryPrice <= 0 {
		return S.GetPercentage(price)
	}

	return ((price - S.Position.AverageEntryPrice) / price) * 100
}
