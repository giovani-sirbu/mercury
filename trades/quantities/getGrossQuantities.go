package quantities

import (
	"strings"

	"github.com/giovani-sirbu/mercury/events"
)

// GetGrossQuantities returns the raw aggregated buy and sell quantities from a
// trade's history. Quantities are summed in whatever unit TradesHistory.Quantity
// is stored in (base units for spot fills; the futures adapter populates inverse
// records consistently with the rest of the inverse-arithmetic pipeline).
//
// Comparison on the Type field is case-insensitive — history records originate
// from multiple producers (Binance order responses use uppercase "BUY"/"SELL",
// some internal adjustments use lowercase) so accepting both is intentional.
//
// Unlike GetQuantities, this function:
//   - does NOT apply lot-size rounding (callers subtract fees first)
//   - does NOT multiply by per-record price for inverse trades (callers that
//     need quote-side totals call GetQuantityInQuote separately)
//
// This is the trade-execution primitive: a clean modernized replacement for the
// legacy trades.GetQuantitiesOld that accepts an events.Events for API
// consistency with GetQuantities and GetFees.
func GetGrossQuantities(event events.Events) (buyTotal, sellTotal float64) {
	for _, data := range event.Trade.History {
		if strings.ToLower(data.Type) == "buy" {
			buyTotal += data.Quantity
		} else {
			sellTotal += data.Quantity
		}
	}

	return buyTotal, sellTotal
}
