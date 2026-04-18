package main

import (
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/exchange"
	"github.com/giovani-sirbu/mercury/trades/actions"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// ATOM/USDC (inverse: FALSE, w/ BNB)
// ATOM/USDC (inverse: FALSE, NO BNB)
// ATOM/ETH (inverse: FALSE, w/ BNB)
// ATOM/ETH (inverse: FALSE, NO BNB)

// ATOM/USDC (inverse: TRUE, w/ BNB)
// ATOM/USDC (inverse: TRUE, NO BNB)
// ATOM/ETH (inverse: TRUE, w/ BNB)
// ATOM/ETH (inverse: TRUE, NO BNB)

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
		result := trades.CalculateMinimumQuantity(event) // depth, initial amount, percentage (as a decimal)
		fmt.Println("Total needed sum:", result)

		initialBid, err := trades.CalculateInitialBid(560, event, 0)

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

	actions.CalculateProfit(newEvent)

	// LINK/USDC (no inverse w/out BNB)
	/*
		var tradesHistory []aggragates.TradesHistory
		tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 3.62, Price: 24.53, Type: "BUY", Fees: []aggragates.TradesFees{{Asset: "LINK", Fee: 0.003439}}})
		tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 7.24, Price: 23.99, Type: "BUY", Fees: []aggragates.TradesFees{{Asset: "LINK", Fee: 0.006878}}})
		tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 14.48, Price: 23.46, Type: "BUY", Fees: []aggragates.TradesFees{{Asset: "LINK", Fee: 0.013756}}})
		tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 28.96, Price: 22.59, Type: "BUY", Fees: []aggragates.TradesFees{{Asset: "LINK", Fee: 0.02896}}})
		tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 57.92, Price: 22.09, Type: "BUY", Fees: []aggragates.TradesFees{{Asset: "LINK", Fee: 0.055024}}})
		tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 115.84, Price: 21.39, Type: "BUY", Fees: []aggragates.TradesFees{{Asset: "LINK", Fee: 0.11584}}})
		tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 231.68, Price: 20.32, Type: "BUY", Fees: []aggragates.TradesFees{{Asset: "LINK", Fee: 0.220096}}})

		var strategySettings []aggragates.StrategySettings

		childrenTrade := aggragates.Trades{}
		childrenTrade.History = tradesHistory
		childrenTrade.PositionPrice = 21.22
		childrenTrade.Inverse = false
		childrenTrade.Symbol = "LINK/USDC"
		childrenTrade.ProfitAsset = "USDC"
		childrenTrade.PositionType = "sell"
		childrenTrade.StrategyPair = aggragates.StrategiesPairs{
			TradeFilters: aggragates.TradeFilters{MinNotional: 6, LotSize: 2, PriceFilter: 2},
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
	*/

	// ATOM/USDC (inverse w/ BNB)
	/*
		var tradesHistory []aggragates.TradesHistory
		tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 14.18, Price: 4.094, Type: "SELL", Fees: []aggragates.TradesFees{{Asset: "BNB", Fee: 0.0000415}}})
		tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 28.36, Price: 4.187, Type: "SELL", Fees: []aggragates.TradesFees{{Asset: "BNB", Fee: 0.00008333}}})

		var strategySettings []aggragates.StrategySettings

		childrenTrade := aggragates.Trades{}
		childrenTrade.History = tradesHistory
		childrenTrade.PositionPrice = 4.094
		childrenTrade.Inverse = true
		childrenTrade.Symbol = "ATOM/USDC"
		childrenTrade.ProfitAsset = "USDC"
		childrenTrade.PositionType = "sell"
		childrenTrade.StrategyPair = aggragates.StrategiesPairs{
			TradeFilters: aggragates.TradeFilters{MinNotional: 6, LotSize: 2, PriceFilter: 3},
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
	*/

}
