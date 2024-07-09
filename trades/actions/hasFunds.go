package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
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

func HasFunds(event events.Events) (events.Events, error) {
	if event.Exchange.TestNet {
		return event, nil
	}

	client, _ := event.Exchange.Client()
	assets, assetsErr := client.GetUserAssets() // Get user balance

	if assetsErr != nil {
		return events.Events{}, assetsErr
	}

	pairSymbols := strings.Split(event.Trade.Symbol, "/")
	assetSymbol := pairSymbols[1]

	if event.Trade.Inverse {
		assetSymbol = pairSymbols[0]
	}

	remainedQuantity := GetAssetBudget(assets, assetSymbol)

	quantity := trades.GetQuantityByHistory(event.Trade.History, event.Trade.Inverse)
	if remainedQuantity < quantity*event.Trade.PositionPrice {
		// If nou enough funds update to impasse and return
		msg := fmt.Sprintf("Not enough funds to buy, available qty: %f, necessary qty: %f", remainedQuantity, quantity*event.Trade.PositionPrice)
		event.Trade.PositionType = "impasse"
		tradeInBytes, _ := json.Marshal(event.Trade)
		topic := "update-trade"
		event.Broker.Producer(topic, context.Background(), nil, tradeInBytes)
		return events.Events{}, fmt.Errorf(msg)
	}

	return event, nil
}
