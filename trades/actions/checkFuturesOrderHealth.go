package actions

import (
	"fmt"
	"github.com/adshao/go-binance/v2/futures"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"
)

// IntervalToMinutes converts a Binance interval string (e.g. "15m", "1h") into minutes.
// Returns -1 if the format is invalid.
func IntervalToMinutes(interval string) int {
	if len(interval) < 2 {
		return -1
	}

	// Split numeric part and unit part
	numPart := interval[:len(interval)-1]
	unit := interval[len(interval)-1:]

	value, err := strconv.Atoi(numPart)
	if err != nil || value <= 0 {
		return -1
	}

	switch strings.ToLower(unit) {
	case "m": // minutes
		return value
	case "h": // hours → minutes
		return value * 60
	case "d": // days → minutes
		return value * 60 * 24
	case "w": // weeks → minutes
		return value * 60 * 24 * 7
	default:
		return -1
	}
}

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
			klineInterval := IntervalToMinutes(event.Trade.StrategyPair.StrategySettings[0].KeepAliveInterval)

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
			fmt.Println("CheckFuturesOrderHealth, pending order 0", oppositeSide, string(futures.OrderTypeStopMarket), event.Trade.Symbol, quantityStr, stopPriceStr)
			if createOrderErr != nil {
				return events.Events{}, createOrderErr
			}
			event.Trade.PendingOrder = createOrder.OrderID
			newEvent, newError := event.Events["updateTrade"](event)
			return newEvent, newError
		} else {
			stopLossOrder, _ := client.GetOrderById(event.Trade.Symbol, event.Trade.PendingOrder)
			orderClosedStatuses := []string{string(futures.OrderStatusTypeExpired), string(futures.OrderStatusTypeCanceled)}
			if slices.Contains(orderClosedStatuses, stopLossOrder.Status) {
				createOrder, createOrderErr := client.CreateFuturesOrder(oppositeSide, string(futures.OrderTypeStopMarket), event.Trade.Symbol, quantityStr, stopPriceStr, true)
				fmt.Println("CheckFuturesOrderHealth", oppositeSide, string(futures.OrderTypeStopMarket), event.Trade.Symbol, quantityStr, stopPriceStr)
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
