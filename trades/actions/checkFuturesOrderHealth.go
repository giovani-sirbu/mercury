package actions

import (
	"fmt"
	binanceFutures "github.com/adshao/go-binance/v2/futures"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/helpers"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/futures"
	"math"
	"slices"
	"strconv"
	"time"
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

		// If no position open and no open orders close trade and create a new one
		if posAmt == 0 {
			orders, listOrderErr := client.ListOrders(event.Trade.Symbol)
			if listOrderErr != nil {
				return events.Events{}, listOrderErr
			}
			timeSinceUpdated := time.Since(event.Trade.UpdatedAt)
			minutes := int(timeSinceUpdated.Minutes())
			klineInterval := helpers.IntervalToMinutes(event.Trade.StrategyPair.StrategySettings[0].KeepAliveInterval)

			if len(orders) == 0 {
				event.Trade.Status = aggragates.Closed
				newEvent, newError := event.Events["updateTrade"](event)
				return newEvent, newError
			} else if minutes >= klineInterval {
				for _, order := range orders {
					client.CancelOrders(event.Trade.Symbol, order.OrderID)
				}
				event.Trade.Status = aggragates.Closed
				newEvent, newError := event.Events["updateTrade"](event)
				return newEvent, newError
			}
		}

		absQty := math.Abs(posAmt)

		// Decide market direction by AI action
		if event.Trade.PositionType == "buy" {
			oppositeSide = string(binanceFutures.SideTypeSell)
			stopPrice = price * (1 - stopLoss/float64(leverage))

		} else if event.Trade.PositionType == "sell" {
			oppositeSide = string(binanceFutures.SideTypeBuy)
			stopPrice = price * (1 + stopLoss/float64(leverage))
		}

		lotSize, priceFilter, precisionErr := futures.GetPrecision(event)
		if precisionErr != nil {
			return events.Events{}, precisionErr
		}
		stopPriceStr := fmt.Sprintf("%.*f", priceFilter, stopPrice)
		quantityStr := fmt.Sprintf("%.*f", lotSize, absQty)

		if event.Trade.PendingOrder == 0 {
			createOrder, createOrderErr := client.CreateFuturesOrder(oppositeSide, string(binanceFutures.OrderTypeStopMarket), event.Trade.Symbol, quantityStr, stopPriceStr, true)
			fmt.Println("CheckFuturesOrderHealth, pending order 0", oppositeSide, string(binanceFutures.OrderTypeStopMarket), event.Trade.Symbol, quantityStr, stopPriceStr)
			if createOrderErr != nil {
				return events.Events{}, createOrderErr
			}
			event.Trade.PendingOrder = createOrder.OrderID
			newEvent, newError := event.Events["updateTrade"](event)
			return newEvent, newError
		} else {
			stopLossOrder, _ := client.GetOrderById(event.Trade.Symbol, event.Trade.PendingOrder)
			orderClosedStatuses := []string{string(binanceFutures.OrderStatusTypeExpired), string(binanceFutures.OrderStatusTypeCanceled)}
			if slices.Contains(orderClosedStatuses, stopLossOrder.Status) {
				createOrder, createOrderErr := client.CreateFuturesOrder(oppositeSide, string(binanceFutures.OrderTypeStopMarket), event.Trade.Symbol, quantityStr, stopPriceStr, true)
				fmt.Println("CheckFuturesOrderHealth", oppositeSide, string(binanceFutures.OrderTypeStopMarket), event.Trade.Symbol, quantityStr, stopPriceStr)
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
