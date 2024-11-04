package main

import (
	"fmt"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/actions"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func main() {
	event := events.Events{}
	var tradesHistory []aggragates.TradesHistory
	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 2.35, Price: 4.252, Type: "SELL"})
	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 4.7, Price: 4.348, Type: "SELL"})
	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 9.4, Price: 4.446, Type: "SELL"})
	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 18.8, Price: 4.63, Type: "SELL"})
	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 37.6, Price: 4.735, Type: "SELL"})
	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 75.2, Price: 4.85, Type: "SELL"})
	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 30.4, Price: 4.74, Type: "BUY"})
	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 150.4, Price: 4.956, Type: "SELL"})
	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 300.8, Price: 5.092, Type: "SELL"})
	//tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 601.59, Price: 4.952, Type: "BUY"})

	childrenTrade := aggragates.Trades{}
	childrenTrade.History = tradesHistory
	childrenTrade.PositionPrice = 4.952
	childrenTrade.Inverse = true
	childrenTrade.Symbol = "ATOM/USDT"

	newEvent := events.Events{Trade: childrenTrade, Events: event.Events, EventsNames: []string{"hasProfit"}, TradeSettings: aggragates.TradeSettings{LotSize: 3}}
	newEvent2, err := actions.HasProfit(newEvent)
	fmt.Println(newEvent2.Trade.Profit, err)
}
