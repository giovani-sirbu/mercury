package actions

import (
	"fmt"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/log"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/fees"
	"github.com/giovani-sirbu/mercury/trades/profit"
	"github.com/giovani-sirbu/mercury/trades/quantities"
)

// HasProfit refuses a close that would book under CalculateMinProfit: it
// values the close SimulatedClose says Sell would submit, at
// SimulatedClosePrice, through GetProfit, less one closing leg's fee and the
// opening commissions the quantities do not embody, and assigns the net
// profit to the trade.
func HasProfit(event events.Events) (events.Events, error) {
	quantity, dust, historyType := quantities.SimulatedClose(event)

	trade := event.Trade

	// Sell keeps the remainder as Dust and GetProfit credits it at the position
	// price; set on the copy, never on event.Trade, since a gate may not close.
	trade.Dust = dust

	trade.History = append(trade.History, aggragates.TradesHistory{
		Type:     historyType,
		Quantity: quantity,
		Price:    profit.SimulatedClosePrice(event.Trade),
	})

	netProfit := profit.GetProfit(trade)

	// One closing leg's fee, which the simulated fill lacks.
	closingFees := fees.GetFees(event)

	// The opening commissions the smaller quantity does not embody.
	openingFees := fees.UnembodiedOpeningFees(event)

	netProfit -= closingFees + openingFees

	event.Trade.Profit = netProfit
	event.Params.Profit = netProfit

	minProfit := profit.CalculateMinProfit(event.Trade)

	log.Debug(fmt.Sprintf("hasProfit(TradeID:#%d): PositionPrice(%f), minProfit(%f), fees(%f), netProfit(%f)", event.Trade.ID, event.Trade.PositionPrice, minProfit, closingFees, netProfit))

	if netProfit < minProfit {
		msg := fmt.Sprintf("profit(%f) is smaller than min profit(%f) for symbol %s | trade_id: %d | user_id: %d", netProfit, minProfit, event.Trade.Symbol, event.Trade.ID, event.Trade.UserID)
		return event, fmt.Errorf(msg)
	}

	log.Debug("")

	return event, nil
}
