package actions

import (
	"fmt"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/log"
	"github.com/giovani-sirbu/mercury/trades/profit"
)

// AcceptLoss calculates the trade net profit like HasProfit but accepts a negative
// result, so a trade in take loss mode can sell below break even
func AcceptLoss(event events.Events) (events.Events, error) {
	// the close simulation — quantity, lot-size dust, closing price, fees — is
	// profit.SimulatedCloseProfit's, the one figure the smart take loss's loss
	// cap reads too
	netProfit := profit.SimulatedCloseProfit(event)

	// assign net profit to trade
	event.Trade.Profit = netProfit
	event.Params.Profit = netProfit

	if netProfit < 0 {
		log.Info(fmt.Sprintf("acceptLoss(TradeID:#%d): selling with accepted loss(%f) for symbol %s | user_id: %d", event.Trade.ID, netProfit, event.Trade.Symbol, event.Trade.UserID), "AcceptLoss", "")
	}

	return event, nil
}
