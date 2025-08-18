package actions

import (
	"fmt"
	"github.com/adshao/go-binance/v2/futures"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"strings"
)

func CreateFuturesOrders(event events.Events) (events.Events, error) {
	// Init futures client
	client, clientErr := event.Exchange.FuturesClient()
	if clientErr != nil {
		return events.Events{}, clientErr
	}
	// Store leverage value
	leverage := int(event.Trade.StrategyPair.StrategySettings[0].Leverage)
	// Store stop loss value
	stopLoss := float64(event.Trade.StrategyPair.StrategySettings[0].StopLoss) * 0.01

	// Set leverage value to the exchange symbol
	_, err := client.SetSymbolLeverage(event.Trade.Symbol, leverage)

	if err != nil {
		return events.Events{}, err
	}

	// Store price
	price := event.Trade.PositionPrice

	// Fetch lot size and price precision values from exchange
	lotSize, priceFilter, precisionErr := GetPrecision(event)

	if precisionErr != nil {
		return events.Events{}, precisionErr
	}

	// Set a initial amount value
	usdAmount := 2.0
	if len(event.Trade.History) > 0 {
		// TODO: take user assets and use 10%
		usdAmount = event.Trade.History[0].Quantity
	} else {
		return events.Events{}, fmt.Errorf("history is mandatory in v1")
	}

	// Calculate position value by the usd value
	positionValue := usdAmount * float64(leverage)
	qty := positionValue / event.Trade.PositionPrice

	qty = ToFixed(qty, lotSize)

	quantityStr := fmt.Sprintf("%.*f", event.Trade.StrategyPair.TradeFilters.LotSize, qty)

	adjustment := event.Trade.StrategyPair.StrategySettings[0].PriceAdjustment * 0.01

	var orderSide string
	var oppositeSide string
	var stopPrice float64

	// Decide market direction by AI action
	if event.Params.AIIndicators.AIAction == "LONG" {
		orderSide = string(futures.SideTypeBuy)
		oppositeSide = string(futures.SideTypeSell)
		stopPrice = price * (1 - stopLoss/float64(leverage))
		price = price * (1 + adjustment)

	} else { // TODO use else if for not alow hold actions
		orderSide = string(futures.SideTypeSell)
		oppositeSide = string(futures.SideTypeBuy)
		stopPrice = price * (1 + stopLoss/float64(leverage))
		price = price * (1 - adjustment)

	}

	entryPriceStr := fmt.Sprintf("%.*f", priceFilter, price)

	// Create main Order
	order, createErr := client.CreateFuturesOrder(orderSide, string(futures.OrderTypeLimit), event.Trade.Symbol, quantityStr, entryPriceStr, false)

	if createErr != nil {
		return events.Events{}, createErr
	}

	// Update history
	if len(event.Trade.History) > 0 {
		event.Trade.History = []aggragates.TradesHistory{{Price: price, Quantity: usdAmount, Type: orderSide, OrderId: order.OrderID, Status: "CREATED"}}
	} else {
		event.Trade.History = append(event.Trade.History, aggragates.TradesHistory{Price: price, Quantity: usdAmount, Type: orderSide, OrderId: order.OrderID, Status: "CREATED"})
	}

	stopPriceStr := fmt.Sprintf("%.*f", priceFilter, stopPrice)
	// Create Stop loss order
	createOrder, createStopLossErr := client.CreateFuturesOrder(oppositeSide, string(futures.OrderTypeStopMarket), event.Trade.Symbol, quantityStr, stopPriceStr, true)

	// Set stop loss order into pending order
	event.Trade.PendingOrder = createOrder.OrderID
	event.Trade.PositionType = strings.ToLower(orderSide)

	if createStopLossErr != nil {
		return events.Events{}, createStopLossErr
	}

	return event, nil
}
