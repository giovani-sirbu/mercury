package main

import (
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/actions"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func main() {
	/*
		result := trades.CalculateMinimumQuantity(7, 5, 2.5) // depth, initial amount, percentage (as a decimal)
		fmt.Println("Total needed sum:", result)

		event := aggragates.Trades{
			Inverse:       false,
			PositionPrice: 10,
			StrategyPair: aggragates.StrategiesPairs{
				TradeFilters: aggragates.TradeFilters{
					MinNotional: 5,
				},
				StrategySettings: []aggragates.StrategySettings{
					{
						Percentage: 2.5,
						Multiplier: 2,
						MinDepths:  7,
						Depths:     7,
					},
				},
			},
		}
		initialBid, err := trades.CalculateInitialBid(560, event, 0)

		fmt.Println(initialBid, err)
		return
	*/

	var tradesHistory []aggragates.TradesHistory
	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 3.97, Price: 5.029, Type: "SELL"})
	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 7.94, Price: 5.158, Type: "SELL"})
	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 15.88, Price: 5.281, Type: "SELL"})
	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 31.76, Price: 5.426, Type: "SELL"})
	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 63.52, Price: 5.619, Type: "SELL"})
	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 127.04, Price: 5.762, Type: "SELL"})
	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 254.08, Price: 5.937, Type: "SELL"})
	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 508.16, Price: 6.146, Type: "SELL"})
	// tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 1016.46, Price: 5.94, Type: "BUY"})
	//tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 150.4, Price: 4.956, Type: "SELL"})
	//tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 300.8, Price: 5.092, Type: "SELL"})
	//tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 601.59, Price: 4.952, Type: "BUY"})

	childrenTrade := aggragates.Trades{}
	childrenTrade.History = tradesHistory
	childrenTrade.PositionPrice = 6.315
	childrenTrade.Inverse = true
	childrenTrade.Symbol = "ATOM/USDT"
	childrenTrade.PositionType = "buy"

	defaultActions := actions.GetDefaultActions() // Use trades logic default actions
	newEvent := events.Events{Trade: childrenTrade, Events: defaultActions, EventsNames: []string{"hasFunds"}}
	newEvent.Run()
}
