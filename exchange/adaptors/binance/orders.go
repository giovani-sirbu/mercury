package binanceAdaptor

import (
	"context"
	"fmt"
	"strings"

	"github.com/adshao/go-binance/v2"
	"github.com/adshao/go-binance/v2/common"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
	"github.com/jinzhu/copier"
)

// Buy submits a LIMIT buy order. The caller is responsible for computing
// a price that respects the pair's tick size — binance will reject orders
// that do not align.
func (e Binance) Buy(symbol string, quantity float64, price string) (aggregates.CreateOrderResponse, *common.APIError) {
	var actionResult aggregates.CreateOrderResponse
	result, err := e.binanceCreateOrder(binance.SideTypeBuy, binance.OrderTypeLimit, symbol, quantity, price)
	copier.Copy(&actionResult, &result)
	return actionResult, err
}

// Sell submits a LIMIT sell order. Same tick-size constraint as Buy.
func (e Binance) Sell(symbol string, quantity float64, price string) (aggregates.CreateOrderResponse, *common.APIError) {
	var actionResult aggregates.CreateOrderResponse
	result, err := e.binanceCreateOrder(binance.SideTypeSell, binance.OrderTypeLimit, symbol, quantity, price)
	copier.Copy(&actionResult, &result)
	return actionResult, err
}

// MarketBuy submits a MARKET buy — executes at the current best ask.
func (e Binance) MarketBuy(symbol string, quantity float64) (aggregates.CreateOrderResponse, *common.APIError) {
	var actionResult aggregates.CreateOrderResponse
	result, err := e.binanceCreateOrder(binance.SideTypeBuy, binance.OrderTypeMarket, symbol, quantity, "")
	copier.Copy(&actionResult, &result)
	return actionResult, err
}

// MarketSell submits a MARKET sell — executes at the current best bid.
func (e Binance) MarketSell(symbol string, quantity float64) (aggregates.CreateOrderResponse, *common.APIError) {
	var actionResult aggregates.CreateOrderResponse
	result, err := e.binanceCreateOrder(binance.SideTypeSell, binance.OrderTypeMarket, symbol, quantity, "")
	copier.Copy(&actionResult, &result)
	return actionResult, err
}

// GetOrder fetches an order by id. Returns the zero Order if not found.
func (e Binance) GetOrder(orderId int64, symbol string) (aggregates.Order, *common.APIError) {
	formattedSymbol := strings.Replace(symbol, "/", "", 1)
	client, initErr := InitExchange(e)
	if initErr != nil {
		return aggregates.Order{}, initErr
	}
	order, err := client.NewGetOrderService().Symbol(formattedSymbol).
		OrderID(orderId).Do(context.Background())
	if err != nil {
		return aggregates.Order{}, ApiError(err)
	}
	var orderResult aggregates.Order
	copier.Copy(&orderResult, &order)
	return orderResult, nil
}

// CancelOrder cancels an open order by id.
func (e Binance) CancelOrder(orderId int64, symbol string) (aggregates.CancelOrderResponse, *common.APIError) {
	formattedSymbol := strings.Replace(symbol, "/", "", 1)
	client, initErr := InitExchange(e)
	if initErr != nil {
		return aggregates.CancelOrderResponse{}, initErr
	}
	order, err := client.NewCancelOrderService().Symbol(formattedSymbol).
		OrderID(orderId).Do(context.Background())
	if err != nil {
		return aggregates.CancelOrderResponse{}, ApiError(err)
	}

	var orderResult aggregates.CancelOrderResponse
	copier.Copy(&orderResult, &order)
	return orderResult, nil
}

// GetTrades returns the fills associated with a given order id.
func (e Binance) GetTrades(orderId int64, symbol string) ([]aggregates.Trade, *common.APIError) {
	formattedSymbol := strings.Replace(symbol, "/", "", 1)
	client, initErr := InitExchange(e)
	if initErr != nil {
		return nil, initErr
	}
	order, err := client.NewListTradesService().Symbol(formattedSymbol).
		OrderId(orderId).Do(context.Background())
	if err != nil {
		return nil, ApiError(err)
	}
	var orderResult []aggregates.Trade
	copier.Copy(&orderResult, &order)
	return orderResult, nil
}

// binanceCreateOrder is the shared order-placement primitive. Internal-only;
// the exported Buy/Sell/MarketBuy/MarketSell helpers above pick the right
// side + type combination before calling it.
func (e Binance) binanceCreateOrder(sideType binance.SideType, orderType binance.OrderType, symbol string, quantity float64, price string) (*binance.CreateOrderResponse, *common.APIError) {
	formattedSymbol := strings.Replace(symbol, "/", "", 1)
	client, initErr := InitExchange(e)
	if initErr != nil {
		return nil, initErr
	}

	stringQuantity := fmt.Sprintf("%f", quantity)

	var order *binance.CreateOrderResponse
	var err error

	if orderType == binance.OrderTypeMarket {
		order, err = client.NewCreateOrderService().NewOrderRespType("FULL").Symbol(formattedSymbol).
			Side(sideType).Type(binance.OrderTypeMarket).Quantity(stringQuantity).Do(context.Background())
	}

	if orderType == binance.OrderTypeLimit {
		order, err = client.NewCreateOrderService().Symbol(formattedSymbol).
			Side(sideType).Type(binance.OrderTypeLimit).
			TimeInForce(binance.TimeInForceTypeGTC).Quantity(stringQuantity).
			Price(price).Do(context.Background())
	}

	return order, ApiError(err)
}
