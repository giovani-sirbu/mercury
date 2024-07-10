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
	buyQty, _ := trades.GetQuantities(event.Trade.History)
	quantity := buyQty
	historyType := "sell"

	if event.Trade.Inverse {
		quantity = trades.GetQuantityInQuote(event.Trade.History, "BUY")
		quantity = quantity / event.Trade.PositionPrice
		quantity = ToFixed(quantity, event.TradeSettings.LotSize)
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

	if profit-fee < 0 {
		msg := fmt.Sprintf("profit: %f is smaller then min profit", profit-fee)
		return event, fmt.Errorf(msg)
	}

	event.Params.Profit = profit - fee

	return event, nil
}
