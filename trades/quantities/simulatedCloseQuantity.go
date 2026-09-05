package quantities

import (
	"github.com/giovani-sirbu/mercury/helpers"
	"github.com/giovani-sirbu/mercury/trades/fees"
	"strings"

	"github.com/giovani-sirbu/mercury/events"
)

// SimulatedCloseQuantity is the quantity a close would actually submit, and
// the history type of that close — the same arithmetic as Sell: gross entry
// quantity minus exits minus the commissions the exchange already took from
// the received asset. HasProfit/AcceptLoss/ParentTradeHasProfit simulate the
// close with THIS quantity, which is what makes charging one closing-leg fee
// on top (GetFees) a faithful estimate: the opening legs' commissions are
// embodied in the smaller quantity, the closing leg is charged once. The old
// estimators simulated with the gross quantity and compensated with
// `fees * 2`; gross quantity plus one leg undercharged the round trip.
func SimulatedCloseQuantity(event events.Events) (float64, string) {
	buyQty, sellQty := GetGrossQuantities(event)
	feeInBase, feeInQuote := fees.CalculateFees(event)
	lotSize := int(event.Trade.StrategyPair.TradeFilters.LotSize)

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
		quantity := sellInQuote - buyInQuote - feeInQuote
		if event.Trade.PositionPrice > 0 {
			quantity /= event.Trade.PositionPrice
		}
		quantity -= feeInBase
		return helpers.ToFixed(quantity, lotSize), "buy"
	}

	return helpers.ToFixed(buyQty-sellQty-feeInBase, lotSize), "sell"
}
