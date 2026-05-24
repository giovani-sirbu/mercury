package actions

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/adshao/go-binance/v2/common"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
	"github.com/giovani-sirbu/mercury/trades"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
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
	buyQty, sellQty := trades.GetQuantitiesOld(event.Trade.History)

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

			if !event.Trade.Inverse {
				amount = amount - aggragates.FindUsedAmount(event.Params.InverseUsedAmount, assetSymbol)
			}

			var err error
			quantity, err = trades.CalculateInitialBid(amount, event.Trade, settingsIndex)

			if !event.Trade.Inverse {
				quantity /= event.Trade.PositionPrice
			}

			if err != nil {
				return SaveError(event, err)
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

	// get quantity
	quantity = ToFixed(quantity, int(event.Trade.StrategyPair.TradeFilters.LotSize))
	minQuantity := CalculateMinOrderQty(event.Trade)
	quantity = math.Max(quantity, minQuantity)
	event.Params.Quantity = quantity

	var response aggregates.CreateOrderResponse
	var err *common.APIError

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

	// Write the OrderID -> TradeID mapping directly to Redis (no broker
	// dependency). If the downstream updateTrade action cannot publish
	// (messagebus outage), agora's user-data-stream handler still has a way
	// to reconcile the fill: it reads this mapping when its DB query
	// `WHERE pending_order = ?` finds nothing. Closes Gap 1.
	if response.OrderID != 0 && event.Storage != nil {
		mapping := aggragates.BinanceOrderMapping{
			TradeID: event.Trade.ID,
			UserID:  event.Trade.UserID,
			Symbol:  event.Trade.Symbol,
		}
		_ = event.Storage.Set(fmt.Sprintf("binance-order:%d", response.OrderID), mapping, 24*time.Hour)
	}

	if err != nil {
		return SaveError(event, err)
	}
	return event, nil
}

// CalculateMinOrderQty returns the minimum amount based on lotSize (decimal places) and minNotional
func CalculateMinOrderQty(trade aggragates.Trades) float64 {
	if trade.StrategyPair.TradeFilters.MinNotional == 0 ||
		trade.StrategyPair.TradeFilters.LotSize == 0 {
		return 0
	}

	quantity := trade.StrategyPair.TradeFilters.MinNotional

	if !trade.Inverse {
		quantity /= trade.PositionPrice
		quantity += math.Pow(10, -float64(trade.StrategyPair.TradeFilters.LotSize))
	}

	return ToFixed(quantity, int(trade.StrategyPair.TradeFilters.LotSize))
}
