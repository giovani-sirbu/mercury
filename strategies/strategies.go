package strategies

import (
	"fmt"
	"sync"

	"github.com/Knetic/govaluate"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// logicExpressions caches compiled logic by its source string; the strings
// are the engines' own GetLogic literals, so the cache is bounded.
var logicExpressions sync.Map

// logicExpression is the compiled form of logic, parsed once; a string
// govaluate cannot parse caches as nil, and evaluating nil panics.
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
		// AverageEntryPrice is the ladder's break even (ladder.AverageEntryPrice):
		// the upside of the buy rows reads profitPercentage, the move against
		// it, while Price, the last fill or re-anchor, keeps anchoring
		// percentage for the downside and every armed state. Zero falls back to Price.
		AverageEntryPrice float64
		// RestingOrder is an entry order not yet settled, a resting add or a
		// first entry until it is reconciled; only the buy row reads it.
		RestingOrder bool
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

// GetPosition evaluates the position's logic row on the held depth's
// settings: percentage is the move against Position.Price, profitPercentage
// the move against Position.AverageEntryPrice, both negated by the engines on
// an inverse trade and the latter raised by smarttakeloss.TakeProfitPercentage
// on a pending or latched ladder, and restingOrder is Position.RestingOrder.
// An unknown position type answers nothing.
func (S Strategy) GetPosition(percentage float64, profitPercentage float64) string {
	if len(S.Settings) < 1 {
		return ""
	}

	logic, ok := S.Logic[S.Position.Type]
	if !ok {
		// An unknown position type fails closed instead of evaluating nil.
		return ""
	}
	expression := logicExpression(logic)

	// The held depth's own row, else the base row.
	settingsIndex := int(S.Depth)
	if settingsIndex < 0 || settingsIndex >= len(S.Settings) {
		settingsIndex = 0
	}

	parameters := make(map[string]interface{}, 6)
	parameters["percentage"] = percentage
	parameters["profitPercentage"] = profitPercentage
	parameters["tradePercentage"] = S.Settings[settingsIndex].Percentage
	parameters["tolerance"] = S.Settings[settingsIndex].Tolerance
	parameters["trailingTakeProfit"] = S.Settings[settingsIndex].TrailingTakeProfit
	parameters["restingOrder"] = S.Position.RestingOrder

	result, _ := expression.Evaluate(parameters)
	newPosition := fmt.Sprintf("%s", result)

	return newPosition
}

// GetPercentage is the move from the position price to price, in percent.
func (S Strategy) GetPercentage(price float64) float64 {
	return ((price - S.Position.Price) / price) * 100
}

// GetProfitPercentage is the same move measured against the average entry
// price, the break even; without entries it equals GetPercentage.
func (S Strategy) GetProfitPercentage(price float64) float64 {
	if S.Position.AverageEntryPrice <= 0 {
		return S.GetPercentage(price)
	}

	return ((price - S.Position.AverageEntryPrice) / price) * 100
}
