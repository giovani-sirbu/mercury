package actions

import (
	"github.com/adshao/go-binance/v2/common"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
	"github.com/giovani-sirbu/mercury/helpers"
	"github.com/giovani-sirbu/mercury/trades/funds"
	"github.com/giovani-sirbu/mercury/trades/ladder"
	"github.com/giovani-sirbu/mercury/trades/quantities"
	"github.com/giovani-sirbu/mercury/trades/tradelog"
	"math"
	"strconv"
	"strings"
)

// Buy places the next entry: a market order for the first fill, sized by
// CalculateInitialBid off the wallet's budget, and a limit order at the
// position price for every add, sized from the last entry times the row's
// multiplier. A shortfall HasFunds waived trims the entry to the wallet.
func Buy(event events.Events) (events.Events, error) {
	client, clientError := event.Exchange.Client()
	if clientError != nil {
		return events.Events{}, clientError
	}

	quantityType := "BUY"
	if event.Trade.Inverse {
		quantityType = "SELL"
	}

	quantity := ladder.GetLatestQuantityByHistory(event.Trade.History, quantityType)
	buyQty, sellQty := quantities.GetGrossQuantities(event)

	// A first entry is sized from the rows the engine named for it
	// (Params.EntrySettings through SizingTrade); every other entry reads the trade's own rows.
	sizing := event.Params.SizingTrade(event.Trade)
	strategySettings := sizing.StrategyPair.StrategySettings
	filledEntries := ladder.CountFilledEntries(event.Trade)
	// Entry N reads row N-1; a missing row falls back to the base row.
	settingsIndex := ladder.SettingsIndexOrBase(strategySettings, filledEntries)

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
				return tradelog.SaveError(event, assetsErr)
			}
			pairSymbols := strings.Split(event.Trade.Symbol, "/")
			assetSymbol := pairSymbols[1]

			if event.Trade.Inverse {
				assetSymbol = pairSymbols[0]
			}

			amount := funds.GetAssetBudget(assets, assetSymbol)

			if !event.Trade.Inverse {
				amount = amount - helpers.FindUsedAmount(event.Params.InverseUsedAmount, assetSymbol)
			}

			var err error
			quantity, err = ladder.CalculateInitialBid(amount, sizing, settingsIndex)

			if !event.Trade.Inverse {
				quantity /= event.Trade.PositionPrice
			}

			if err != nil {
				return tradelog.SaveError(event, err)
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

	// HasFunds waived a small shortfall and named the balance: place the entry for what it buys.
	if event.Params.AvailableQuantity > 0 {
		affordableQuantity := event.Params.AvailableQuantity
		if !event.Trade.Inverse {
			affordableQuantity = event.Params.AvailableQuantity / event.Trade.PositionPrice
		}
		quantity = math.Min(quantity, affordableQuantity)
	}

	quantity = helpers.ToFixed(quantity, int(event.Trade.StrategyPair.TradeFilters.LotSize))
	minQuantity := quantities.CalculateMinOrderQty(event.Trade)
	quantity = math.Max(quantity, minQuantity)
	event.Params.Quantity = quantity

	var response aggregates.CreateOrderResponse
	var err *common.APIError

	if filledEntries > 0 {
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
		return tradelog.SaveError(event, err)
	}
	return event, nil
}
