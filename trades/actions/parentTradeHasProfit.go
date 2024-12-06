package actions

import (
	"fmt"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/log"
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
		childrenPrice, priceErr := client.GetPrice(childrenTrade.Symbol)

		if priceErr != nil || childrenPrice == 0 {
			return events.Events{}, fmt.Errorf("failed to get children price")
		}

		childrenPrice = ToFixed(childrenPrice, int(childrenTrade.StrategyPair.TradeFilters.PriceFilter))

		childrenTrade.PositionPrice = childrenPrice
		event.ChildrenTrades[index].PositionPrice = childrenPrice
		newEvent := events.Events{Trade: childrenTrade, Events: event.Events, EventsNames: []string{"hasProfit"}}
		newEvent, _ = event.Events["hasProfit"](newEvent)
		childrenProfit = childrenProfit + newEvent.Trade.Profit
	}

	msg := fmt.Sprintf("Profit info: parent profit: %f, childrens profit: %f, fee: %f", profit, childrenProfit, fee)
	log.Info(msg, "parentHasProfit", "events")
	profit = profit - fee + (childrenProfit * event.Trade.PositionPrice)

	if profit < 0 {
		msg := fmt.Sprintf("profit: %f is smaller then min profit for symbol %s, trade id %d, user id %d", profit, event.Trade.Symbol, event.Trade.ID, event.Trade.UserID)
		return events.Events{}, fmt.Errorf(msg)
	}

	return event, nil
}
