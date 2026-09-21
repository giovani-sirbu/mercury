package quantities

import (
	"github.com/giovani-sirbu/mercury/helpers"
	"github.com/giovani-sirbu/mercury/trades/fees"
	"strings"

	"github.com/giovani-sirbu/mercury/events"
)

// SimulatedClose is the close Sell would submit: the quantity it would send,
// the lot-size remainder it would keep as Trades.Dust, and the history type of
// that close.
//
// Both halves are mirrored because GetProfit reads both. Sell floors the
// closeable quantity to the pair's lot size and assigns what the floor cut to
// Dust, which GetProfit credits at the position price — so a gate that
// simulates the close with the floored quantity alone values that remainder at
// zero where the close itself values it at market. The gap is one lot step as
// a share of the position: immaterial on a full-sized position, decisive on
// one sized near the pair's minimum notional, where it can put every price the
// trade could close at under the gate's threshold.
//
// The quantity itself is the same arithmetic as Sell: gross entry quantity
// minus exits minus the commissions the exchange already took from the
// received asset. HasProfit/AcceptLoss/ParentTradeHasProfit simulate the close
// with THIS quantity, which is what makes charging one closing-leg fee on top
// (GetFees) a faithful estimate: the opening legs' commissions are embodied in
// the smaller quantity, the closing leg is charged once. The old estimators
// simulated with the gross quantity and compensated with `fees * 2`; gross
// quantity plus one leg undercharged the round trip.
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

	// Sell abandons a close it cannot submit before it reaches its own dust
	// assignment, so a non-positive quantity carries none here either.
	// Crediting the whole remainder would let a gate pass a close that puts no
	// order on the book.
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
