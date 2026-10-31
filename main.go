package main

import (
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/exchange"
	"github.com/giovani-sirbu/mercury/trades/actions"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/profit"
)

func main() {
	/*
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
		result := ladder.CalculateMinimumQuantity(event) // depth, initial amount, percentage (as a decimal)
		fmt.Println("Total needed sum:", result)

		initialBid, err := ladder.CalculateInitialBid(560, event, 0)

		fmt.Println(initialBid, err)
		return
	*/

	// ATOM/ETH (inverse TRUE with BNB)
	var tradesHistory []aggragates.TradesHistory
	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 14.181, Price: 0.000994, Type: "SELL", Fees: []aggragates.TradesFees{{Asset: "BNB", Fee: 0.00004712}}})
	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 28.362, Price: 0.001011, Type: "SELL", Fees: []aggragates.TradesFees{{Asset: "BNB", Fee: 0.00008988}}})
	var strategySettings []aggragates.StrategySettings

	childrenTrade := aggragates.Trades{}
	childrenTrade.History = tradesHistory
	childrenTrade.PositionPrice = 0.000986
	childrenTrade.Inverse = true
	childrenTrade.Symbol = "ATOM/ETH"
	childrenTrade.ProfitAsset = "ATOM"
	childrenTrade.PositionType = "buy"
	childrenTrade.StrategyPair = aggragates.StrategiesPairs{
		TradeFilters: aggragates.TradeFilters{MinNotional: 0.001, LotSize: 3, PriceFilter: 6},
		StrategySettings: append(strategySettings, aggragates.StrategySettings{
			Percentage: 2.25,
			Tolerance:  0.25,
		}),
	}
	defaultActions := actions.GetDefaultActions() // Use trades logic default actions
	newEvent := events.Events{
		Trade: childrenTrade,
		Exchange: exchange.Exchange{
			Name:      "binance",
			ApiKey:    "",
			ApiSecret: "",
		},
		Events:      defaultActions,
		EventsNames: []string{"hasProfit"},
	}
	newEvent.Run()

	profit.CalculateProfit(newEvent)

}
