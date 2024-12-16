package actions

import (
	"fmt"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
	"github.com/giovani-sirbu/mercury/log"
	"github.com/giovani-sirbu/mercury/trades"
	"strconv"
	"strings"
)

func GetAssetBudget(assets []aggregates.UserAssetRecord, assetSymbol string) float64 {
	var remainedQuantity float64 // Init needed quantity

	// Check if account has remaining balance for pair
	for _, balance := range assets {
		if balance.Asset == assetSymbol {
			floatQuantity, _ := strconv.ParseFloat(balance.Free, 64)
			remainedQuantity = floatQuantity
		}
	}

	return remainedQuantity
}

func GetUsedQuantities(event events.Events) float64 {
	buyQuantity, sellQuantity := trades.GetQuantities(event.Trade.History)
	feeInBase, _ := CalculateFees(event.Trade.History, event.Trade.Symbol)
	quantity := buyQuantity - sellQuantity - feeInBase

	if event.Trade.Inverse {
		sellQuantity = trades.GetQuantityInQuote(event.Trade.History, "BUY")
		buyQuantity = trades.GetQuantityInQuote(event.Trade.History, "SELL")
		quantity = sellQuantity - buyQuantity
		quantity = quantity / event.Trade.PositionPrice
		quantity = quantity - feeInBase
	}

	quantity = ToFixed(quantity, int(event.Trade.StrategyPair.TradeFilters.LotSize))
	return quantity
}

func HasFunds(event events.Events) (events.Events, error) {
	client, _ := event.Exchange.Client()
	assets, assetsErr := client.GetUserAssets() // Get user balance
	sellAction := false
	if assetsErr != nil {
		return events.Events{}, assetsErr
	}

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

	pairSymbols := strings.Split(event.Trade.Symbol, "/")
	multiplier := strategySettings[settingsIndex].Multiplier

	var assetSymbol string
	var quantity float64

	if event.Trade.PositionType == "sell" {
		sellAction = true
	}

	if sellAction {
		buyQty, sellQty := trades.GetQuantities(event.Trade.History)
		assetSymbol = pairSymbols[0]
		quantity = buyQty - sellQty
		if event.Trade.Inverse {
			quantity = (buyQty - sellQty) * event.Trade.PositionPrice
			assetSymbol = pairSymbols[1]
		}
	} else {
		assetSymbol = pairSymbols[1]
		quantityType := "BUY"
		if event.Trade.Inverse {
			quantityType = "SELL"
		}
		quantity = trades.GetLatestQuantityByHistory(event.Trade.History, quantityType) * multiplier
		if event.Trade.Inverse {
			assetSymbol = pairSymbols[0]
		}
	}

	remainedQuantity := GetAssetBudget(assets, assetSymbol)
	neededQuantity := quantity * event.Trade.PositionPrice

	if event.Trade.Inverse {
		neededQuantity = quantity
	}

	if remainedQuantity < neededQuantity {
		// If nou enough funds update to impasse and return
		msg := fmt.Sprintf("Failed to %s %f %s. Available quantity: %f", event.Trade.PositionType, quantity*event.Trade.PositionPrice, assetSymbol, remainedQuantity)
		debugErrorMsg := fmt.Sprintf("Insufficient funds for #%d to buy %s, available qty: %f, necessary qty: %f", event.Trade.UserID, event.Trade.Symbol, remainedQuantity, quantity*event.Trade.PositionPrice)
		log.Debug(debugErrorMsg)
		if event.Trade.Strategy.Params.Impasse && event.Trade.ParentID == 0 {
			usedAmount := GetUsedQuantities(event)
			_, hasFundsError := trades.CalculateInitialBid(usedAmount, event.Trade, event.Trade.StrategyPair.StrategySettings[0])
			if hasFundsError == nil {
				event.Trade.PositionType = "impasse"
			}
		}
		return SaveError(event, fmt.Errorf(msg))
	}

	return event, nil
}
