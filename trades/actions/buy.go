package actions

import (
	"fmt"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
	"github.com/giovani-sirbu/mercury/trades"
	"strconv"
	"strings"
)

func Buy(event events.Events) (events.Events, error) {
	client, clientError := event.Exchange.Client()
	if clientError != nil {
		return events.Events{}, clientError
	}

	quantityType := "BUY"
	if event.Trade.Inverse {
		quantityType = "SELL"
	}

	quantity := trades.GetLatestQuantityByHistory(event.Trade.History, quantityType)
	buyQty, sellQty := trades.GetQuantities(event.Trade.History)

	historyCount := len(event.Trade.History)
	strategySettings := event.Trade.StrategyPair.StrategySettings
	var settingsIndex int

	if historyCount > len(strategySettings) {
		settingsIndex = len(strategySettings) - 1
	} else {
		settingsIndex = historyCount - 1
	}

	if historyCount == 0 {
		settingsIndex = 0
	}

	multiplier := strategySettings[settingsIndex].Multiplier
	depths := strategySettings[settingsIndex].Depths
	if event.Trade.ParentID != 0 {
		depths = strategySettings[settingsIndex].ImpasseDepth
	}
	pairInitialBid := strategySettings[settingsIndex].InitialBid
	minNotion := event.Trade.StrategyPair.TradeFilters.MinNotional / event.Trade.PositionPrice

	if quantity == 0 {
		var initialBid float64
		if pairInitialBid > 0 {
			initialBid = pairInitialBid
			quantity = minNotion * initialBid
		} else {
			assets, assetsErr := client.GetUserAssets() // Get user balance
			if assetsErr != nil {
				return SaveError(event, assetsErr)
			}
			pairSymbols := strings.Split(event.Trade.Symbol, "/")
			assetSymbol := pairSymbols[1]

			if event.Trade.Inverse {
				assetSymbol = pairSymbols[0]
			}

			amount := GetAssetBudget(assets, assetSymbol)

			quantity = trades.GetInitialBid(amount, depths, multiplier)
			minNotionQuantity := quantity
			if event.Trade.Inverse {
				minNotionQuantity = quantity * event.Trade.PositionPrice
			} else {
				quantity = quantity / event.Trade.PositionPrice
			}

			if minNotionQuantity < event.Trade.StrategyPair.TradeFilters.MinNotional {
				return SaveError(event, fmt.Errorf("not enough funds to start logic"))
			}
		}
		multiplier = 1
	}

	priceInString := strconv.FormatFloat(event.Trade.PositionPrice, 'f', -1, 64)
	quantity = quantity * multiplier
	if event.Trade.Inverse {
		quantity = quantity - buyQty
	} else {
		quantity = quantity - sellQty
	}

	quantity = ToFixed(quantity, int(event.Trade.StrategyPair.TradeFilters.LotSize))

	event.Params.Quantity = quantity

	var response aggregates.CreateOrderResponse
	var err error

	if historyCount > 0 {
		if event.Trade.Inverse {
			response, err = client.Sell(event.Trade.Symbol, quantity, priceInString)
		} else {
			response, err = client.Buy(event.Trade.Symbol, quantity, priceInString)
		}
	} else {
		if event.Trade.Inverse {
			response, err = client.MarketSell(event.Trade.Symbol, quantity)
		} else {
			response, err = client.MarketBuy(event.Trade.Symbol, quantity)
		}
	}

	event.Trade.PendingOrder = response.OrderID

	if err != nil {
		return SaveError(event, err)
	}
	return event, nil
}
