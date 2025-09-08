package actions

import (
	"fmt"
	"github.com/adshao/go-binance/v2/futures"
	"github.com/giovani-sirbu/mercury/events"
)

func UpdateStopLossOrder(event events.Events) (events.Events, error) {
	// Init futures client
	client, clientError := event.Exchange.FuturesClient()
	// Store leverage value
	leverage := int(event.Trade.StrategyPair.StrategySettings[0].Leverage)
	// Store stop loss value
	stopLoss := float64(event.Trade.StrategyPair.StrategySettings[0].StopLoss) * 0.01
	trailingTakeProfit := event.Trade.StrategyPair.StrategySettings[0].TrailingTakeProfit

	if trailingTakeProfit > 0 {
		stopLoss = trailingTakeProfit
	}

	price := event.Trade.PositionPrice

	if clientError != nil {
		return events.Events{}, clientError
	}

	var stopPrice float64

	if event.Trade.PositionType == "sell" {
		stopPrice = price * (1 + stopLoss/float64(leverage))

	} else if event.Trade.PositionType == "buy" {
		stopPrice = price * (1 - stopLoss/float64(leverage))

	}

	_, priceFilter, _ := GetPrecision(event)

	stopPriceStr := fmt.Sprintf("%.*f", priceFilter, stopPrice)

	createOrder, createStopLossErr := client.ModifyFuturesOrderPrice(event.Trade.Symbol, event.Trade.PendingOrder, stopPriceStr)

	event.Trade.PendingOrder = createOrder.OrderID

	if createStopLossErr != nil {
		fmt.Println("update stop loss order error", string(futures.OrderTypeStopMarket), event.Trade.Symbol, stopPriceStr, true)
		return events.Events{}, createStopLossErr
	}

	return event, nil
}
