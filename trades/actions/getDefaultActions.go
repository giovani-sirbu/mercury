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
	newActions["sellAll"] = SellAll
	newActions["hasProfit"] = HasProfit
	newActions["createChildrenTrades"] = CreateChildrenTrades
	newActions["parentTradeHasProfit"] = ParentTradeHasProfit
	newActions["shouldHold"] = ShouldHold
	newActions["hasEnoughFunds"] = HasEnoughFunds
	newActions["regulatePriceChange"] = RegulatePriceChange
	newActions["closeFuturesTrade"] = CloseFuturesTrade
	newActions["checkOldFuturesOrders"] = CheckOldFuturesOrders
	newActions["createFuturesOrders"] = CreateFuturesOrders
	newActions["closeOrKeepALiveTrade"] = CloseOrKeepALiveTrade
	newActions["updateStopLossOrder"] = UpdateStopLossOrder
	return newActions
}
