package actions

import (
	"fmt"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func ParentTradeHasProfit(event events.Events) (events.Events, error) {
	// get trade quantities and history type
	quantity, historyType := GetQuantities(event)

	// simulate sell event to calculate profit & get trade profit
	trade := event.Trade
	trade.History = append(trade.History, aggragates.TradesHistory{
		Type:     historyType,
		Quantity: quantity,
		Price:    trade.PositionPrice,
	})

	// get gross profit
	profit := GetProfit(trade)

	// return event fees & multiply by 2 to simulate total fees for the sell event
	fees := GetFees(event)
	fees *= 2

	// subtract fees and return net profit
	profit -= fees

	// gen children profits
	var childrenProfit float64
	client, _ := event.Exchange.Client()
	for index, childrenTrade := range event.ChildrenTrades {
		childrenPrice, priceErr := client.GetPrice(childrenTrade.Symbol)

		if priceErr != nil || childrenPrice == 0 {
			return events.Events{}, fmt.Errorf("failed to get children price")
		}

		childrenPrice = ToFixed(childrenPrice, int(childrenTrade.StrategyPair.TradeFilters.PriceFilter))

		childrenTrade.PositionPrice = childrenPrice
		event.ChildrenTrades[index].PositionPrice = childrenPrice
		newEvent := events.Events{Trade: childrenTrade, Events: event.Events, EventsNames: []string{"hasProfit"}}
		newEvent, _ = event.Events["hasProfit"](newEvent)
		childrenProfit += newEvent.Trade.Profit
	}

	// sum children profit to total trade profit
	profit += childrenProfit * event.Trade.PositionPrice

	if profit < 0 {
		msg := fmt.Sprintf("profit: %f is smaller then min profit for symbol %s, trade id %d, user id %d", profit, event.Trade.Symbol, event.Trade.ID, event.Trade.UserID)
		return events.Events{}, fmt.Errorf(msg)
	}

	return event, nil
}
