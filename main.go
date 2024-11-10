package main

import (
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/actions"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func main() {
	var tradesHistory []aggragates.TradesHistory
	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 10, Price: 9.038, Type: "SELL"})
	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 20, Price: 9.223, Type: "SELL"})
	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 40, Price: 9.412, Type: "SELL"})
	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 80, Price: 9.604, Type: "SELL"})
	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 160, Price: 9.8, Type: "SELL"})
	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 320, Price: 10, Type: "SELL"})
	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 20, Price: 9.8, Type: "BUY"})
	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 620, Price: 10.2, Type: "SELL"})
	//tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 150.4, Price: 4.956, Type: "SELL"})
	//tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 300.8, Price: 5.092, Type: "SELL"})
	//tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 601.59, Price: 4.952, Type: "BUY"})

	childrenTrade := aggragates.Trades{}
	childrenTrade.History = tradesHistory
	childrenTrade.PositionPrice = 10
	childrenTrade.Inverse = true
	childrenTrade.Symbol = "ATOM/USDT"

	defaultActions := actions.GetDefaultActions() // Use trades logic default actions
	newEvent := events.Events{Trade: childrenTrade, Events: defaultActions, EventsNames: []string{"hasProfit", "buy"}, TradeSettings: aggragates.TradeSettings{LotSize: 3}}
	newEvent.Run()
}
