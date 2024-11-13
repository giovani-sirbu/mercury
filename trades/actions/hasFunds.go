package actions

import (
	"encoding/json"
	"fmt"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
	"github.com/giovani-sirbu/mercury/log"
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

func HasFunds(event events.Events) (events.Events, error) {
	client, _ := event.Exchange.Client()
	assets, assetsErr := client.GetUserAssets() // Get user balance
	sellAction := false
	if assetsErr != nil {
		return events.Events{}, assetsErr
	}

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

	pairSymbols := strings.Split(event.Trade.Symbol, "/")
	multiplier := StrategySettings[settingsIndex].Multiplier

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

	if remainedQuantity < neededQuantity {
		log.Debug("remainedQuantity: ", remainedQuantity, assetSymbol)
		log.Debug("neededQuantity: ", neededQuantity)

		// If nou enough funds update to impasse and return
		msg := fmt.Sprintf("Not enough funds to buy, available qty: %f, necessary qty: %f", remainedQuantity, quantity*event.Trade.PositionPrice)
		event.Trade.PositionType = "impasse"
		newEvent, err := UpdateTrade(event)
		if err != nil {
			return SaveError(event, err)
		}
		return SaveError(newEvent, fmt.Errorf(msg))
	}

	return event, nil
}
