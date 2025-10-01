package actions

import (
	"fmt"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func HasProfit(event events.Events) (events.Events, error) {
	// deduct strategy settings tolerance from position price to simulate unrealised PnL
	event.Trade.PositionPrice = subtractToleranceFromPrice(event.Trade)

	// get trade quantities and history type
	quantity, historyType := GetQuantities(event)

	// simulate sell event to calculate profit & get trade profit
	trade := event.Trade
	trade.History = append(trade.History, aggragates.TradesHistory{
		Type:     historyType,
		Quantity: quantity,
		Price:    trade.PositionPrice,
	})
	profit := GetProfit(trade)

	// return event fees
	fees := GetFees(event)

	// simulate sell fees also which are total buy sees multiply by 2
	fees *= 2

	// subtract fees and return net profit
	profit -= fees

	// assign net profit to trade
	event.Trade.Profit = profit

	// get min profit
	minProfit := CalculateMinProfit(event.Trade)

	// ------ just for debug
	/*
		simulateHistory := event.Trade.History
		feeInBase, feeInQuote := CalculateFeesOld(event)
		buyQty, sellQty := trades.GetQuantitiesOld(event.Trade.History)
		quantityy := buyQty - sellQty
		historyTypee := "sell"

		if event.Trade.Inverse {
			buyQuantity := trades.GetQuantityInQuote(event.Trade.History, "BUY")
			sellQuantity := trades.GetQuantityInQuote(event.Trade.History, "SELL")
			quantityy = (buyQuantity - sellQuantity) / event.Trade.PositionPrice
			quantityy = ToFixed(quantityy, int(event.Trade.StrategyPair.TradeFilters.LotSize))
			historyTypee = "buy"
		}

		simulateHistory = append(simulateHistory, aggragates.TradesHistory{Type: historyTypee, Quantity: quantityy, Price: event.Trade.PositionPrice})
		sellTotal, buyTotal := GetProfitOld(simulateHistory)
		profitt := sellTotal - buyTotal
		fee := feeInQuote

		if event.Trade.Inverse {
			fee = feeInBase
			sellTotal, buyTotal = GetProfitInBase(simulateHistory)
			profitt = buyTotal - sellTotal
		}

		fmt.Println("")
		fmt.Println("getQuantity:", quantity, historyType, "[NEW] vs ", quantityy, historyTypee, "[OLD]")
		fmt.Println("getProfit:", profit, "[NEW] vs ", profitt, "[OLD]")
		fmt.Println("getFees:", fees, "[NEW] vs ", fee, "[OLD]")
	*/
	// ------ just for debug

	if profit < minProfit {
		msg := fmt.Sprintf("profit(%f) is smaller than min profit(%f) for symbol %s | trade_id: %d | user_id: %d", profit, minProfit, event.Trade.Symbol, event.Trade.ID, event.Trade.UserID)
		return event, fmt.Errorf(msg)
	}

	event.Params.Profit = profit

	fmt.Println("")

	return event, nil
}

// deduct strategy settings tolerance from position price to simulate unrealised PnL
func subtractToleranceFromPrice(trade aggragates.Trades) float64 {
	toleranceAmount := trade.PositionPrice * (trade.StrategyPair.StrategySettings[0].Tolerance / 100)
	if trade.Inverse {
		trade.PositionPrice += toleranceAmount
	} else {
		trade.PositionPrice -= toleranceAmount
	}

	return ToFixed(trade.PositionPrice, int(trade.StrategyPair.TradeFilters.PriceFilter))
}
