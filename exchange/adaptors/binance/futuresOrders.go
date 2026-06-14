package binanceAdaptor

import (
	"context"
	"strconv"
	"strings"

	"github.com/adshao/go-binance/v2/common"
	"github.com/adshao/go-binance/v2/futures"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
	"github.com/jinzhu/copier"
)

// CreateFutureOrder places an order on the futures market. reduceOnly must
// be set for closing orders so binance rejects the request if it would open
// a new position (e.g. a trailing stop that accidentally flips direction).
func (e Binance) CreateFutureOrder(sideType string, orderType string, symbol string, quantity string, price string, reduceOnly bool) (aggregates.CreateOrderResponse, *common.APIError) {
	var order aggregates.CreateOrderResponse

	client, initErr := InitFuturesExchange(e)
	if initErr != nil {
		return order, initErr
	}
	formattedSymbol := strings.Replace(symbol, "/", "", 1)

	orderResponse := client.NewCreateOrderService().
		Symbol(formattedSymbol).
		Side(futures.SideType(sideType)).
		Type(futures.OrderType(orderType)).
		Quantity(quantity).
		ReduceOnly(reduceOnly)

	if orderType == string(futures.OrderTypeLimit) {
		orderResponse.Price(price).TimeInForce(futures.TimeInForceTypeGTC)
	} else if orderType == string(futures.OrderTypeStopMarket) {
		orderResponse.StopPrice(price)
	}

	response, err := orderResponse.Do(context.Background())
	copier.Copy(&order, &response)
	return order, ApiError(err)
}

// ListOrders returns the currently open futures orders for a single symbol.
func (e Binance) ListOrders(symbol string) ([]aggregates.FuturesOrder, *common.APIError) {
	var orders []aggregates.FuturesOrder
	client, initErr := InitFuturesExchange(e)
	if initErr != nil {
		return nil, initErr
	}
	formattedSymbol := strings.Replace(symbol, "/", "", 1)
	responseOrders, err := client.NewListOpenOrdersService().Symbol(formattedSymbol).Do(context.Background())
	copier.Copy(&orders, &responseOrders)
	return orders, ApiError(err)
}

// GetOrderById returns a single futures order by id.
func (e Binance) GetOrderById(symbol string, orderID int64) (aggregates.FuturesOrder, *common.APIError) {
	var order aggregates.FuturesOrder
	client, initErr := InitFuturesExchange(e)
	if initErr != nil {
		return aggregates.FuturesOrder{}, initErr
	}
	formattedSymbol := strings.Replace(symbol, "/", "", 1)
	responseOrder, err := client.NewGetOrderService().
		Symbol(formattedSymbol).
		OrderID(orderID).
		Do(context.Background())

	copier.Copy(&order, &responseOrder)
	return order, ApiError(err)
}

// CancelOrders cancels a single futures order by id despite the plural name
// (preserved for API compatibility with mercury callers).
func (e Binance) CancelOrders(symbol string, orderId int64) (aggregates.CancelFuturesOrderResponse, *common.APIError) {
	var cancelOrder aggregates.CancelFuturesOrderResponse
	client, initErr := InitFuturesExchange(e)
	if initErr != nil {
		return cancelOrder, initErr
	}
	formattedSymbol := strings.Replace(symbol, "/", "", 1)
	orderResponse, err := client.NewCancelOrderService().
		Symbol(formattedSymbol).
		OrderID(orderId).
		Do(context.Background())

	copier.Copy(&cancelOrder, &orderResponse)
	return cancelOrder, ApiError(err)
}

// ModifyFuturesOrderPrice replaces a STOP_MARKET order's trigger price by
// cancelling the original and placing a new one. Binance has no in-place
// modify for STOP_MARKET, so this sequence (cancel -> validate vs mark price
// -> derive ReduceOnly from current position -> recreate) is deliberate.
//
// The validation step guards against accidentally placing a stop on the
// wrong side of the current price, which binance would accept and trigger
// instantly.
func (e Binance) ModifyFuturesOrderPrice(symbol string, orderId int64, newPrice string) (aggregates.CreateOrderResponse, *common.APIError) {
	var updateResponse aggregates.CreateOrderResponse

	client, initErr := InitFuturesExchange(e)
	if initErr != nil {
		return updateResponse, initErr
	}

	formattedSymbol := strings.Replace(symbol, "/", "", 1)

	// 1. Cancel the existing order.
	cancelResp, err := client.NewCancelOrderService().
		Symbol(formattedSymbol).
		OrderID(orderId).
		Do(context.Background())
	if err != nil {
		return updateResponse, ApiError(err)
	}

	stopPrice, _ := strconv.ParseFloat(newPrice, 64)

	// 2. Get current mark price.
	markPriceResp, err := client.NewPremiumIndexService().
		Symbol(formattedSymbol).
		Do(context.Background())
	if err != nil {
		return updateResponse, ApiError(err)
	}
	currPrice, _ := strconv.ParseFloat(markPriceResp[0].MarkPrice, 64)

	// 3. Validate stop price relative to current price.
	if cancelResp.Side == futures.SideTypeSell && stopPrice >= currPrice {
		return updateResponse, &common.APIError{Message: "Invalid stop: sell stop must be below current price"}
	}
	if cancelResp.Side == futures.SideTypeBuy && stopPrice <= currPrice {
		return updateResponse, &common.APIError{Message: "Invalid stop: buy stop must be above current price"}
	}

	// 4. Check position before setting ReduceOnly.
	positions, err := client.NewGetPositionRiskService().Symbol(formattedSymbol).Do(context.Background())
	if err != nil {
		return updateResponse, ApiError(err)
	}
	reduceOnly := false
	for _, p := range positions {
		qty, _ := strconv.ParseFloat(p.PositionAmt, 64)
		if qty != 0 {
			reduceOnly = true
			break
		}
	}

	// 5. Place a new STOP_MARKET order with validated params.
	newOrder, err := client.NewCreateOrderService().
		Symbol(formattedSymbol).
		Side(cancelResp.Side).
		Type(futures.OrderTypeStopMarket).
		Quantity(cancelResp.OrigQuantity).
		StopPrice(newPrice).
		ReduceOnly(reduceOnly).
		WorkingType(futures.WorkingTypeMarkPrice).
		Do(context.Background())
	if err != nil {
		return updateResponse, ApiError(err)
	}

	copier.Copy(&updateResponse, &newOrder)
	return updateResponse, nil
}
