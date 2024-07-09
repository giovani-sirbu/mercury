package main

import (
	"fmt"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades"
	"github.com/giovani-sirbu/mercury/trades/actions"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func main() {
	fmt.Println(trades.GetInitialBid(1000, 5, 2))
	event := events.Events{}
	var tradesHistory []aggragates.TradesHistory
	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 0.908, Price: 0.002206})
	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 1.816, Price: 0.002248})
	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 3.632, Price: 0.002292})

	childrenTrade := aggragates.Trades{}
	childrenTrade.History = tradesHistory
	childrenTrade.PositionPrice = 0
	childrenTrade.Inverse = true
	childrenTrade.Symbol = "ATOM/ETH"

	newEvent := events.Events{Trade: childrenTrade, Events: event.Events, EventsNames: []string{"hasProfit"}, TradeSettings: aggragates.TradeSettings{LotSize: 3}}
	newEvent2, err := actions.HasProfit(newEvent)
	fmt.Println(newEvent2.Trade.Profit, err)
}
