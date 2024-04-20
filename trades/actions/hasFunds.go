package actions

import (
	"fmt"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/exchange"
	"github.com/giovani-sirbu/mercury/trades"
	"strconv"
	"strings"
)

func HasFunds(event events.Events) (events.Events, error) {
	if event.Exchange.TestNet {
		return event, nil
	}
	exchangeInit := exchange.Exchange{Name: event.Exchange.Name, ApiKey: event.Exchange.ApiKey, ApiSecret: event.Exchange.ApiSecret, TestNet: event.Exchange.TestNet}
	client, _ := exchangeInit.Client()
	assets, assetsErr := client.GetUserAssets() // Get user balance

	if assetsErr != nil {
		return events.Events{}, assetsErr
	}

	var remainedQuantity float64 // Init needed quantity

	// Check if account has remaining balance for pair
	for _, balance := range assets {
		pairSymbols := strings.Split(event.Trade.Symbol, "/")
		if balance.Asset == pairSymbols[1] {
			floatQuantity, _ := strconv.ParseFloat(balance.Free, 64)
			remainedQuantity = floatQuantity
		}
	}

	quantity := trades.GetQuantityByHistory(event.Trade.History, event.Trade.Inverse)
	if remainedQuantity < quantity*event.Trade.PositionPrice {
		// If nou enough funds log and return
		msg := fmt.Sprintf("Not enough funds to buy, available qty: %f, necessary qty: %f", remainedQuantity, quantity*event.Trade.PositionPrice)
		return events.Events{}, fmt.Errorf(msg)
	}

	return event, nil
}
