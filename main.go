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
	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 10, Price: 4.252, Type: "BUY"})
	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 20, Price: 4.348, Type: "BUY"})
	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 40, Price: 4.446, Type: "BUY"})
	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 80, Price: 4.63, Type: "BUY"})
	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 160, Price: 4.735, Type: "BUY"})
	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 250.4, Price: 4.74, Type: "SELL"})
	//tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 150.4, Price: 4.956, Type: "SELL"})
	//tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 300.8, Price: 5.092, Type: "SELL"})
	//tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 601.59, Price: 4.952, Type: "BUY"})

	childrenTrade := aggragates.Trades{}
	childrenTrade.History = tradesHistory
	childrenTrade.PositionPrice = 4.952
	childrenTrade.Inverse = false
	childrenTrade.Symbol = "ATOM/USDT"

	newEvent := events.Events{Trade: childrenTrade, Events: event.Events, EventsNames: []string{"buy"}, TradeSettings: aggragates.TradeSettings{LotSize: 3}}
	newEvent2, err := actions.Buy(newEvent)
	fmt.Println(newEvent2.Trade.Profit, err)
}
