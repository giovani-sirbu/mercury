package actions

import (
	"fmt"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func HasProfit(event events.Events) (events.Events, error) {
	simulateHistory := event.Trade.History
	feeInBase, feeInQuote := CalculateFees(event.Trade.History, event.Trade.Symbol)
	buyQty, sellQty := trades.GetQuantities(event.Trade.History)
	quantity := buyQty - sellQty
	historyType := "sell"

	if event.Trade.Inverse {
		buyQuantity := trades.GetQuantityInQuote(event.Trade.History, "BUY")
		sellQuantity := trades.GetQuantityInQuote(event.Trade.History, "SELL")
		quantity = (buyQuantity - sellQuantity) / event.Trade.PositionPrice
		quantity = ToFixed(quantity, int(event.Trade.StrategyPair.TradeFilters.LotSize))
		historyType = "buy"
	}

	simulateHistory = append(simulateHistory, aggragates.TradesHistory{Type: historyType, Quantity: quantity, Price: event.Trade.PositionPrice})
	sellTotal, buyTotal := GetProfit(simulateHistory)
	profit := sellTotal - buyTotal
	fee := feeInQuote

	if event.Trade.Inverse {
		fee = feeInBase
		sellTotal, buyTotal = GetProfitInBase(simulateHistory)
		profit = buyTotal - sellTotal
	}

	event.Trade.Profit = profit
	minProfit := CalculateMinProfit(event.Trade)
	if profit-fee < minProfit {
		msg := fmt.Sprintf("profit(%f) is smaller than min profit(%f) for symbol %s | trade_id: %d | user_id: %d", profit-fee, minProfit, event.Trade.Symbol, event.Trade.ID, event.Trade.UserID)
		return event, fmt.Errorf(msg)
	}

	event.Params.Profit = profit - fee

	return event, nil
}
