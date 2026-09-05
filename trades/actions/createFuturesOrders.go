package actions

import (
	"fmt"
	binanceFutures "github.com/adshao/go-binance/v2/futures"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/helpers"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/futures"
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
	lotSize, priceFilter, precisionErr := futures.GetPrecision(event)

	if precisionErr != nil {
		return events.Events{}, precisionErr
	}

	// set default margin percentage amount
	marginPercentage := event.Trade.StrategyPair.StrategySettings[0].MarginPercentage
	if marginPercentage == 0 {
		marginPercentage = 10 // default 10%
	}
	usdAmount := event.Trade.Exchange.Balance
	usdAmount *= marginPercentage / 100
	if usdAmount <= 0 {
		return events.Events{}, fmt.Errorf("failed to set margin percentage amount")
	}

	// Calculate position value by the usd value
	positionValue := usdAmount * float64(leverage)
	qty := positionValue / event.Trade.PositionPrice

	qty = helpers.ToFixed(qty, lotSize)

	// Format with the futures precision just fetched, not the pair's spot lot
	// size: the rounding on the line above already uses it, and the two
	// disagree whenever the same symbol trades at different precision on the
	// two venues — printing more decimals than the futures filter allows is
	// rejected, printing fewer silently changes the size.
	quantityStr := fmt.Sprintf("%.*f", lotSize, qty)

	adjustment := event.Trade.StrategyPair.StrategySettings[0].PriceAdjustment * 0.01

	var orderSide string
	var oppositeSide string
	var stopPrice float64

	// Decide market direction by AI action
	if event.Params.AIIndicators.AIAction == "LONG" {
		orderSide = string(binanceFutures.SideTypeBuy)
		oppositeSide = string(binanceFutures.SideTypeSell)
		stopPrice = price * (1 - stopLoss/float64(leverage))
		price = price * (1 + adjustment)

	} else if event.Params.AIIndicators.AIAction == "SHORT" {
		orderSide = string(binanceFutures.SideTypeSell)
		oppositeSide = string(binanceFutures.SideTypeBuy)
		stopPrice = price * (1 + stopLoss/float64(leverage))
		price = price * (1 - adjustment)

	}

	// Anything else — HOLD, an empty verdict, a sophos outage — leaves both
	// sides empty and the stop price at zero, and the two calls below then
	// submit an order with no side. There is no direction to trade here, so
	// the chain stops instead.
	if orderSide == "" {
		return events.Events{}, fmt.Errorf("no futures direction for %s: AI action is %q", event.Trade.Symbol, event.Params.AIIndicators.AIAction)
	}

	entryPriceStr := fmt.Sprintf("%.*f", priceFilter, price)

	// Create main Order
	order, createErr := client.CreateFuturesOrder(orderSide, string(binanceFutures.OrderTypeLimit), event.Trade.Symbol, quantityStr, entryPriceStr, false)
	fmt.Println("CreateFuturesOrders: Create order::", orderSide, string(binanceFutures.OrderTypeLimit), event.Trade.Symbol, quantityStr, entryPriceStr)

	if createErr != nil {
		fmt.Println("create main order error", orderSide, string(binanceFutures.OrderTypeLimit), event.Trade.Symbol, quantityStr, entryPriceStr, false)
		return events.Events{}, createErr
	}

	// Update history
	event.Trade.History = append(event.Trade.History, aggragates.TradesHistory{
		Price:    price,
		Quantity: usdAmount / price,
		Type:     orderSide,
		OrderId:  order.OrderID,
		Status:   "CREATED",
	})

	stopPriceStr := fmt.Sprintf("%.*f", priceFilter, stopPrice)
	// Create Stop loss order
	createOrder, createStopLossErr := client.CreateFuturesOrder(oppositeSide, string(binanceFutures.OrderTypeStopMarket), event.Trade.Symbol, quantityStr, stopPriceStr, true)
	fmt.Println("CreateFuturesOrders: Create stop loss order::", oppositeSide, string(binanceFutures.OrderTypeStopMarket), event.Trade.Symbol, quantityStr, stopPriceStr)

	// Set stop loss order into pending order
	event.Trade.PendingOrder = createOrder.OrderID
	event.Trade.PositionType = strings.ToLower(orderSide)

	if createStopLossErr != nil {
		fmt.Println("create stop loss order error", oppositeSide, string(binanceFutures.OrderTypeStopMarket), event.Trade.Symbol, quantityStr, stopPriceStr, true)
		return events.Events{}, createStopLossErr
	}

	return event, nil
}
