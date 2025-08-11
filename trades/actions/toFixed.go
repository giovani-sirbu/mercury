package actions

import "github.com/shopspring/decimal"

func ToFixed(num float64, precision int) float64 {
	d := decimal.NewFromFloat(num).RoundFloor(int32(precision))
	result, _ := d.Float64()
	return result
}
