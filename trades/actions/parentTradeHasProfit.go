package actions

import (
	"fmt"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func ParentTradeHasProfit(event events.Events) (events.Events, error) {
	simulateHistory := event.Trade.History
	_, fee := CalculateFees(event.Trade.History, event.Trade.Symbol)
	buyQty, _ := trades.GetQuantities(event.Trade.History)
	quantity := buyQty
	historyType := "sell"

	simulateHistory = append(simulateHistory, aggragates.TradesHistory{Type: historyType, Quantity: quantity, Price: event.Trade.PositionPrice})
	sellTotal, buyTotal := GetProfit(simulateHistory)
	profit := sellTotal - buyTotal

	var childrenProfit float64

	for index, childrenTrade := range event.ChildrenTrades {
		client, _ := event.Exchange.Client()
		childrenPrice, _ := client.GetPrice(childrenTrade.Symbol)
		childrenTrade.PositionPrice = childrenPrice
		newEvent := events.Events{Trade: childrenTrade, Events: event.Events, EventsNames: []string{"hasProfit"}, TradeSettings: event.ChildrenTradeSettings[index]}
		newEvent, _ = HasProfit(newEvent)
		childrenProfit = childrenProfit + newEvent.Trade.Profit
	}

	if profit-fee+(childrenProfit*event.Trade.PositionPrice) < 0 {
		msg := fmt.Sprintf("profit: %f is smaller then min profit", profit-fee-(childrenProfit*event.Trade.PositionPrice))
		return events.Events{}, fmt.Errorf(msg)
	}

	return event, nil
}
