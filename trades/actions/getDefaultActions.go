package actions

import "github.com/giovani-sirbu/mercury/events"

// GetDefaultActions get default trades events functions
func GetDefaultActions() map[string]func(events.Events) (events.Events, error) {
	var newActions = make(map[string]func(events.Events) (events.Events, error))
	newActions["updateTrade"] = UpdateTrade
	newActions["cancelPendingOrder"] = CancelPendingOrder
	newActions["hasFunds"] = HasFunds
	newActions["buy"] = Buy
	newActions["sell"] = Sell
	newActions["hasProfit"] = HasProfit
	newActions["duplicateTrade"] = DuplicateTrade
	return newActions
}
