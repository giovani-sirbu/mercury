package strategies

import (
	"fmt"
	"github.com/Knetic/govaluate"
)

type Position struct {
	Type  string
	Price float64
}

type Settings struct {
	Tolerance          float64 `bson:"tolerance" json:"tolerance"`
	TrailingTakeProfit float64 `bson:"trailingTakeProfit" json:"trailingTakeProfit"`
	Percentage         float64 `bson:"percentage" json:"percentage"`
}

type Strategy struct {
	Type     string
	Position Position
	Price    float64
	Settings []Settings
	Logic    map[string]string
	Depth    int32
}

func (S Strategy) GetPosition(percentage float64) string {
	if len(S.Settings) < 1 {
		return ""
	}

	fmt.Println(S.Logic[S.Position.Type])
	expression, _ := govaluate.NewEvaluableExpression(S.Logic[S.Position.Type])

	parameters := make(map[string]interface{}, 8)
	parameters["percentage"] = percentage
	parameters["tradePercentage"] = S.Settings[S.Depth].Percentage
	parameters["tolerance"] = S.Settings[S.Depth].Tolerance
	parameters["trailingTakeProfit"] = S.Settings[S.Depth].TrailingTakeProfit

	fmt.Println(parameters)
	result, _ := expression.Evaluate(parameters)
	newPosition := fmt.Sprintf("%s", result)

	return newPosition
}

func (S Strategy) GetPercentage(price float64) float64 {
	return ((price - S.Position.Price) / price) * 100
}
