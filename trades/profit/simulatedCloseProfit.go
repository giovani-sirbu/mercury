package profit

import (
	"slices"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/fees"
	"github.com/giovani-sirbu/mercury/trades/quantities"
)

// SimulatedCloseProfit is the net profit a close of the whole position would
// book now, the figure acceptLoss assigns: the close quantities.SimulatedClose
// says Sell would submit, valued at SimulatedClosePrice through GetProfit,
// less one closing leg's fee (fees.GetFees) and the opening commissions the
// fill quantities do not embody (fees.UnembodiedOpeningFees). Negative for a
// loss. The history is cloned, so the caller's event is never written.
func SimulatedCloseProfit(event events.Events) float64 {
	quantity, dust, historyType := quantities.SimulatedClose(event)

	// Sell keeps the remainder as Dust and GetProfit credits it at the position price.
	trade := event.Trade
	trade.Dust = dust
	trade.History = append(slices.Clone(event.Trade.History), aggragates.TradesHistory{
		Type:     historyType,
		Quantity: quantity,
		Price:    SimulatedClosePrice(event.Trade),
	})

	netProfit := GetProfit(trade)

	// One closing leg plus the opening commissions the quantities do not embody.
	closingFees := fees.GetFees(event)
	openingFees := fees.UnembodiedOpeningFees(event)

	return netProfit - (closingFees + openingFees)
}
