package profit

import (
	"slices"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/fees"
	"github.com/giovani-sirbu/mercury/trades/quantities"
)

// SimulatedCloseProfit is the net profit a close of the whole position would
// book now, the figure acceptLoss assigns to the trade: the close
// quantities.SimulatedClose says Sell would submit, valued at
// SimulatedClosePrice, through GetProfit, less one closing leg's fee
// (fees.GetFees) and the opening commissions the fill quantities do not embody
// (fees.UnembodiedOpeningFees). It is negative for a loss.
//
// The simulated close — its quantity, the lot-size dust it leaves, its row — is
// built on a copy: the history is cloned before the close row is appended, so
// the caller's slice never grows, and the event the caller holds is never
// written. A third-asset fee on the history is priced through the event's wallet
// prices or its exchange, so a caller that cannot price one hands in a history
// without them.
func SimulatedCloseProfit(event events.Events) float64 {
	// the quantity the close would actually submit (net of embodied fees), and
	// the lot-size remainder it would leave behind
	quantity, dust, historyType := quantities.SimulatedClose(event)

	// Sell keeps this remainder as Dust and GetProfit credits it at the
	// position price, so the loss a close accepts and logs carries it too.
	// Without it the cut is reported deeper than the close will book.
	trade := event.Trade
	trade.Dust = dust
	trade.History = append(slices.Clone(event.Trade.History), aggragates.TradesHistory{
		Type:     historyType,
		Quantity: quantity,
		Price:    SimulatedClosePrice(event.Trade),
	})

	// get gross profit
	netProfit := GetProfit(trade)

	// One leg's worth estimates the closing fee the simulated fill lacks, plus
	// whatever opening commission the fill quantities do not embody (see
	// HasProfit and fees.UnembodiedOpeningFees).
	closingFees := fees.GetFees(event)
	openingFees := fees.UnembodiedOpeningFees(event)

	return netProfit - (closingFees + openingFees)
}
