package actions

import (
	"fmt"
	"github.com/adshao/go-binance/v2/futures"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"strconv"
)

const UpdateStopLossStatus = "UPDATED_STOP_LOSS"

func UpdateStopLossOrder(event events.Events) (events.Events, error) {
	// Init futures client
	client, clientError := event.Exchange.FuturesClient()
	// Store leverage value
	leverage := int(event.Trade.StrategyPair.StrategySettings[0].Leverage)
	// Store stop loss value
	stopLoss := float64(event.Trade.StrategyPair.StrategySettings[0].StopLoss) * 0.01
	trailingTakeProfit := event.Trade.StrategyPair.StrategySettings[0].TrailingTakeProfit
	takeProfitPercentage := event.Trade.StrategyPair.StrategySettings[0].Percentage
	tolerance := event.Trade.StrategyPair.StrategySettings[0].Tolerance

	if trailingTakeProfit > 0 {
		stopLoss = trailingTakeProfit
	}

	lastHistoryStatus := event.Trade.History[len(event.Trade.History)-1].Status

	if takeProfitPercentage > 0 && lastHistoryStatus != UpdateStopLossStatus {
		stopLoss = takeProfitPercentage - tolerance
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

	fmt.Println("UpdateStopLossOrder", event.Trade.Symbol, event.Trade.PositionType, price, stopPriceStr)

	event.Trade.PendingOrder = createOrder.OrderID

	if createStopLossErr != nil {
		_, healthError := CheckFuturesOrderHealth(event)
		if healthError != nil {
			fmt.Println("update stop loss check health error", healthError)
		}
		fmt.Println("update stop loss order error", string(futures.OrderTypeStopMarket), event.Trade.Symbol, stopPriceStr, true)
		return events.Events{}, createStopLossErr
	}

	orderQty, _ := strconv.ParseFloat(createOrder.OrigQuantity, 64)
	event.Trade.History = append(event.Trade.History, aggragates.TradesHistory{
		Price:    price,
		Quantity: orderQty,
		Type:     createOrder.Side,
		OrderId:  createOrder.OrderID,
		Status:   UpdateStopLossStatus,
	})

	return event, nil
}
