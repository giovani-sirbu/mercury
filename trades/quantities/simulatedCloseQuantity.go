package quantities

import (
	"github.com/giovani-sirbu/mercury/helpers"
	"github.com/giovani-sirbu/mercury/trades/fees"
	"strings"

	"github.com/giovani-sirbu/mercury/events"
)

// SimulatedClose is the close Sell would submit: the quantity it would send,
// the lot-size remainder it would keep as Trades.Dust, and the history type
// of that close. Both halves mirror Sell because GetProfit reads both: Sell
// floors the closeable quantity to the lot size and credits the remainder as
// dust at the position price, and a gate that simulated the floored quantity
// alone would value that remainder at zero, decisive on a position sized near
// the minimum notional. The quantity is Sell's arithmetic, gross entries minus
// exits minus the commissions the exchange took from the received asset, so
// charging one closing leg on top (GetFees) is a faithful round trip.
func SimulatedClose(event events.Events) (quantity, dust float64, historyType string) {
	buyQty, sellQty := GetGrossQuantities(event)
	feeInBase, feeInQuote := fees.CalculateFees(event)
	lotSize := int(event.Trade.StrategyPair.TradeFilters.LotSize)

	beforeLotSize := buyQty - sellQty - feeInBase
	historyType = "sell"

	if event.Trade.Inverse {
		// Inverse entries SELL base for quote and the close BUYs it back with
		// the quote still held, net of the quote commissions the entries paid.
		var sellInQuote, buyInQuote float64
		for _, row := range event.Trade.History {
			if strings.ToLower(row.Type) == "buy" {
				buyInQuote += row.Quantity * row.Price
			} else {
				sellInQuote += row.Quantity * row.Price
			}
		}
		beforeLotSize = sellInQuote - buyInQuote - feeInQuote
		if event.Trade.PositionPrice > 0 {
			beforeLotSize /= event.Trade.PositionPrice
		}
		beforeLotSize -= feeInBase
		historyType = "buy"
	}

	quantity = helpers.ToFixed(beforeLotSize, lotSize)

	// A close Sell cannot submit carries no dust either.
	if quantity <= 0 {
		return quantity, 0, historyType
	}

	if beforeLotSize > quantity {
		dust = beforeLotSize - quantity
	}

	return quantity, dust, historyType
}

// SimulatedCloseQuantity is SimulatedClose without the dust, for the callers
// that need only the order the close would place.
func SimulatedCloseQuantity(event events.Events) (float64, string) {
	quantity, _, historyType := SimulatedClose(event)
	return quantity, historyType
}
