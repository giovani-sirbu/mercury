// Package helpers holds domain-free utilities (rounding, symbol and interval
// parsing, millisecond clocks) shared by every mercury package and by the
// services. It imports nothing from mercury, so any package may depend on it.
package helpers

import "github.com/shopspring/decimal"

// ToFixed floors num to precision decimals. It never rounds up, so a quantity
// or price fitted to an exchange filter can never exceed the input.
func ToFixed(num float64, precision int) float64 {
	d := decimal.NewFromFloat(num).RoundFloor(int32(precision))
	result, _ := d.Float64()
	return result
}
