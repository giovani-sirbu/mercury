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

// AcceptLoss calculates the trade net profit like HasProfit but accepts a negative
// result, so a trade in take loss mode can sell below break even
func AcceptLoss(event events.Events) (events.Events, error) {
	// the quantity the close would actually submit (net of embodied fees)
	quantity, historyType := quantities.SimulatedCloseQuantity(event)

	// simulate sell event to calculate profit & get trade profit
	trade := event.Trade
	trade.History = append(trade.History, aggragates.TradesHistory{
		Type:     historyType,
		Quantity: quantity,
		Price:    profit.SimulatedClosePrice(event.Trade),
	})

	// get gross profit
	netProfit := profit.GetProfit(trade)

	// One leg's worth estimates the closing fee the simulated fill lacks, plus
	// whatever opening commission the fill quantities do not embody (see
	// HasProfit and fees.UnembodiedOpeningFees).
	closingFees := fees.GetFees(event)
	openingFees := fees.UnembodiedOpeningFees(event)

	// subtract fees and return net profit
	netProfit -= closingFees + openingFees

	// assign net profit to trade
	event.Trade.Profit = netProfit
	event.Params.Profit = netProfit

	if netProfit < 0 {
		log.Info(fmt.Sprintf("acceptLoss(TradeID:#%d): selling with accepted loss(%f) for symbol %s | user_id: %d", event.Trade.ID, netProfit, event.Trade.Symbol, event.Trade.UserID), "AcceptLoss", "")
	}

	return event, nil
}
