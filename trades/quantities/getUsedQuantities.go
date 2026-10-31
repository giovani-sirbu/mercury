package quantities

import (
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/helpers"
	"github.com/giovani-sirbu/mercury/trades/fees"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// GetUsedQuantities is the base-asset position the trade currently holds:
// gross entries minus exits minus the literal base fees, rounded to the lot
// size. Inverse trades hold quote and are converted back at PositionPrice.
func GetUsedQuantities(event events.Events) float64 {
	buyQty, sellQty := GetGrossQuantities(event)
	// Literal asset fees only: see sell.go for the rationale (BNB and other
	// third-asset fees do not reduce base or quote wallet balances).
	feeInBase, _ := fees.CalculateFees(event)
	quantity := buyQty - sellQty - feeInBase

	if event.Trade.Inverse {
		sellInQuote := ladder.GetQuantityInQuote(event.Trade.History, "BUY")
		buyInQuote := ladder.GetQuantityInQuote(event.Trade.History, "SELL")
		quantity = sellInQuote - buyInQuote
		quantity = quantity / event.Trade.PositionPrice
		quantity = quantity - feeInBase
	}

	quantity = helpers.ToFixed(quantity, int(event.Trade.StrategyPair.TradeFilters.LotSize))
	return quantity
}
