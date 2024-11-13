package strategies

import (
	"fmt"
	"github.com/Knetic/govaluate"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

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

	expression, _ := govaluate.NewEvaluableExpression(S.Logic[S.Position.Type])

	parameters := make(map[string]interface{}, 8)
	parameters["percentage"] = percentage
	parameters["tradePercentage"] = S.Settings[S.Depth].Percentage
	parameters["tolerance"] = S.Settings[S.Depth].Tolerance
	parameters["trailingTakeProfit"] = S.Settings[S.Depth].TrailingTakeProfit

	result, _ := expression.Evaluate(parameters)
	newPosition := fmt.Sprintf("%s", result)

	return newPosition
}

// GetPercentage get percentage between old and new price
func (S Strategy) GetPercentage(price float64) float64 {
	return ((price - S.Position.Price) / price) * 100
}
