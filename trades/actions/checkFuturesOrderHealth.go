package actions

import (
	"fmt"
	"github.com/adshao/go-binance/v2/futures"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"math"
	"slices"
	"strconv"
)

func CheckFuturesOrderHealth(event events.Events) (events.Events, error) {
	// Init futures client
	client, clientError := event.Exchange.FuturesClient()
	if clientError != nil {
		return events.Events{}, clientError
	}
	// Fetch the active position
	positions, positionsErr := client.GetSymbolPosition(event.Trade.Symbol)
	if positionsErr != nil {
		return events.Events{}, positionsErr
	}

	// If no position open and no open orders close trade and create a new one
	if len(positions) == 0 {
		orders, listOrderErr := client.ListOrders(event.Trade.Symbol)
		if listOrderErr != nil {
			return events.Events{}, listOrderErr
		}
		if len(orders) == 0 {
			event.Trade.Status = aggragates.Closed
			newEvent, newError := event.Events["updateTrade"](event)
			return newEvent, newError
		}
	}

	// Trade has active position on symbol and no pending order should create stop loss order
	// Trade has active position on symbol and pending order is canceled, filled or expired should create stop loss order
	var closeThePositionErr error
	for _, p := range positions {
		var oppositeSide string
		var stopPrice float64
		leverage := int(event.Trade.StrategyPair.StrategySettings[0].Leverage)
		stopLoss := float64(event.Trade.StrategyPair.StrategySettings[0].StopLoss) * 0.01
		price := event.Params.OldPositionPrice
		posAmt, _ := strconv.ParseFloat(p.PositionAmt, 64)

		absQty := math.Abs(posAmt)

		// Decide market direction by AI action
		if event.Trade.PositionType == "buy" {
			oppositeSide = string(futures.SideTypeSell)
			stopPrice = price * (1 - stopLoss/float64(leverage))

		} else if event.Trade.PositionType == "sell" {
			oppositeSide = string(futures.SideTypeBuy)
			stopPrice = price * (1 + stopLoss/float64(leverage))
		}

		lotSize, priceFilter, precisionErr := GetPrecision(event)
		if precisionErr != nil {
			return events.Events{}, precisionErr
		}
		stopPriceStr := fmt.Sprintf("%.*f", priceFilter, stopPrice)
		quantityStr := fmt.Sprintf("%.*f", lotSize, absQty)

		if event.Trade.PendingOrder == 0 {
			createOrder, createOrderErr := client.CreateFuturesOrder(oppositeSide, string(futures.OrderTypeStopMarket), event.Trade.Symbol, quantityStr, stopPriceStr, true)
			if createOrderErr != nil {
				return events.Events{}, createOrderErr
			}
			event.Trade.PendingOrder = createOrder.OrderID
			newEvent, newError := event.Events["updateTrade"](event)
			return newEvent, newError
		} else {
			stopLossOrder, _ := client.GetOrderById(event.Trade.Symbol, event.Trade.PendingOrder)
			orderClosedStatuses := []string{string(futures.OrderStatusTypeFilled), string(futures.OrderStatusTypeExpired), string(futures.OrderStatusTypeCanceled)}
			if slices.Contains(orderClosedStatuses, stopLossOrder.Status) {
				createOrder, createOrderErr := client.CreateFuturesOrder(oppositeSide, string(futures.OrderTypeStopMarket), event.Trade.Symbol, quantityStr, stopPriceStr, true)
				if createOrderErr != nil {
					return events.Events{}, createOrderErr
				}
				event.Trade.PendingOrder = createOrder.OrderID
				newEvent, newError := event.Events["updateTrade"](event)
				return newEvent, newError
			}
		}
	}
	if closeThePositionErr != nil {
		return events.Events{}, closeThePositionErr
	}

	return event, nil
}
