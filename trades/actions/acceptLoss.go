package actions

import (
	"fmt"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/log"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// AcceptLoss calculates the trade net profit like HasProfit but accepts a negative
// result, so a trade in take loss mode can sell below break even
func AcceptLoss(event events.Events) (events.Events, error) {
	// get trade quantities and history type
	quantity, historyType := GetQuantities(event)

	// simulate sell event to calculate profit & get trade profit
	trade := event.Trade
	trade.History = append(trade.History, aggragates.TradesHistory{
		Type:     historyType,
		Quantity: quantity,
		Price:    subtractToleranceFromPrice(event.Trade), // deduct strategy settings tolerance from position price to simulate unrealised PnL
	})

	// get gross profit
	profit := GetProfit(trade)

	// return event fees & multiply by 2 to simulate total fees for the sell event
	fees := GetFees(event)
	fees *= 2

	// subtract fees and return net profit
	profit -= fees

	// assign net profit to trade
	event.Trade.Profit = profit
	event.Params.Profit = profit

	if profit < 0 {
		log.Info(fmt.Sprintf("acceptLoss(TradeID:#%d): selling with accepted loss(%f) for symbol %s | user_id: %d", event.Trade.ID, profit, event.Trade.Symbol, event.Trade.UserID), "AcceptLoss", "")
	}

	return event, nil
}
