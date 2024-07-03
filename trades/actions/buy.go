package actions

import (
	"encoding/json"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/exchange"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
	"github.com/giovani-sirbu/mercury/trades"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"strconv"
	"strings"
)

func Buy(event events.Events) (events.Events, error) {
	exchangeInit := exchange.Exchange{Name: event.Exchange.Name, ApiKey: event.Exchange.ApiKey, ApiSecret: event.Exchange.ApiSecret, TestNet: event.Exchange.TestNet}
	client, clientError := exchangeInit.Client()
	if clientError != nil {
		return events.Events{}, clientError
	}
	quantity := trades.GetQuantityByHistory(event.Trade.History, event.Trade.Inverse)

	historyCount := len(event.Trade.History)
	settings := []byte(event.Trade.Strategy.Params)
	var StrategySettings []aggragates.StrategyParams
	var settingsIndex int

	json.Unmarshal(settings, &StrategySettings)

	if historyCount > len(StrategySettings) {
		settingsIndex = len(StrategySettings) - 1
	} else {
		settingsIndex = historyCount - 1
	}

	if historyCount == 0 {
		settingsIndex = 0
	}

	multiplier := StrategySettings[settingsIndex].Multiplier
	depths := StrategySettings[settingsIndex].Depths
	pairInitialBid := StrategySettings[settingsIndex].InitialBid

	if quantity == 0 {
		multiplier = 1
		var initialBid float64
		if pairInitialBid > 0 {
			initialBid = pairInitialBid
			quantity = (event.TradeSettings.MinNotion / event.Trade.PositionPrice) * initialBid
		} else {
			assets, assetsErr := client.GetUserAssets() // Get user balance
			if assetsErr != nil {
				return events.Events{}, assetsErr
			}
			pairSymbols := strings.Split(event.Trade.Symbol, "/")
			assetSymbol := pairSymbols[1]

			if event.Trade.Inverse {
				assetSymbol = pairSymbols[0]
			}

			amount := GetAssetBudget(assets, assetSymbol)

			quantity = trades.GetInitialBid(amount, depths, multiplier)
		}

	}

	priceInString := strconv.FormatFloat(event.Trade.PositionPrice, 'f', -1, 64)
	quantity = ToFixed(quantity*multiplier, event.TradeSettings.LotSize)

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
		return events.Events{}, err
	}
	return event, nil
}
