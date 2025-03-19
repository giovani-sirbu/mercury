package actions

import (
	"fmt"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
	"github.com/giovani-sirbu/mercury/trades"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
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

func GetFundsQuantities(event events.Events) (float64, float64, string, error) {
	client, _ := event.Exchange.Client()
	assets, assetsErr := client.GetUserAssets() // Get user balance
	sellAction := false
	if assetsErr != nil {
		return 0, 0, "", assetsErr
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
	var neededQuantity float64

	if event.Trade.PositionType == "sell" || event.Trade.PositionType == "takeProfit" || event.Trade.PositionType == "sellParent" {
		sellAction = true
	}

	if sellAction {
		buyQty, sellQty := trades.GetQuantities(event.Trade.History)
		assetSymbol = pairSymbols[0]
		neededQuantity = buyQty - sellQty

		feeInBase, feeInQuote := CalculateFees(event.Trade.History, event.Trade.Symbol)
		neededQuantity -= feeInBase

		if event.Trade.Inverse {
			neededQuantity = (sellQty - buyQty - feeInQuote) * event.Trade.PositionPrice
			assetSymbol = pairSymbols[1]
		}
	} else {
		assetSymbol = pairSymbols[1]
		quantityType := "BUY"
		if event.Trade.Inverse {
			quantityType = "SELL"
			assetSymbol = pairSymbols[0]
		}
		neededQuantity = trades.GetLatestQuantityByHistory(event.Trade.History, quantityType) * multiplier
		if !event.Trade.Inverse {
			neededQuantity *= event.Trade.PositionPrice
		}
	}

	remainedQuantity := GetAssetBudget(assets, assetSymbol)

	if !event.Trade.Inverse {
		remainedQuantity = remainedQuantity - aggragates.FindUsedAmount(event.Params.InverseUsedAmount, assetSymbol)
		neededQuantity = ToFixed(neededQuantity, int(event.Trade.StrategyPair.TradeFilters.LotSize))
	}

	return remainedQuantity, neededQuantity, assetSymbol, nil
}

func HasFunds(event events.Events) (events.Events, error) {
	remainedQuantity, neededQuantity, assetSymbol, err := GetFundsQuantities(event)

	if err != nil {
		return events.Events{}, err
	}

	if remainedQuantity < neededQuantity {
		// If nou enough funds update to impasse and return
		msg := fmt.Sprintf("%s to %s failed for %f %s. Available quantity: %f", event.Params.OldPosition, event.Trade.PositionType, neededQuantity, assetSymbol, remainedQuantity)
		if event.Trade.Strategy.Params.Impasse && event.Trade.ParentID == 0 {
			usedAmount := GetUsedQuantities(event) * event.Trade.PositionPrice
			_, hasFundsError := trades.CalculateInitialBid(usedAmount, event.Trade, 0)
			if hasFundsError == nil {
				event.Trade.PositionType = "impasse"
			}
		}
		return SaveError(event, fmt.Errorf(msg))
	}

	return event, nil
}
