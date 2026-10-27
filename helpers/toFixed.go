// Package helpers holds utilities (rounding, symbol and interval parsing,
// millisecond clocks) shared by the mercury packages and by the services. It
// imports only trades/aggragates from mercury, for FindUsedAmount and
// FormatPriceLevel, so aggragates must never import it.
package helpers

import "github.com/shopspring/decimal"

// ToFixed floors num to precision decimals. It never rounds up, so a quantity
// or price fitted to an exchange filter can never exceed the input.
func ToFixed(num float64, precision int) float64 {
	d := decimal.NewFromFloat(num).RoundFloor(int32(precision))
	result, _ := d.Float64()
	return result
}
