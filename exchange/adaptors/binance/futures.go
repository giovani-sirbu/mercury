package binanceAdaptor

import (
	"context"
	"github.com/adshao/go-binance/v2/common"
	"github.com/adshao/go-binance/v2/futures"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
	"github.com/jinzhu/copier"
	"strings"
)

// GetFuturesBinanceActions map all binance actions
func GetFuturesBinanceActions(e aggregates.Exchange) aggregates.FuturesActions {
	var binanceStruct = Binance{e}
	var actions = aggregates.FuturesActions{
		CreateFuturesOrder:     binanceStruct.CreateFutureOrder,
		ListOrders:             binanceStruct.ListOrders,
		CancelOrders:           binanceStruct.CancelOrders,
		GetSymbolPosition:      binanceStruct.GetSymbolPosition,
		SetSymbolLeverage:      binanceStruct.SetSymbolLeverage,
		GetFuturesExchangeInfo: binanceStruct.GetFuturesExchangeInfo,
	}
	return actions
}

// InitFuturesExchange Function to initiate binance client
func InitFuturesExchange(exchange Binance) (*futures.Client, *common.APIError) {
	if exchange.TestNet {
		futures.UseTestnet = true
	} else {
		futures.UseTestnet = false
	}

	var client = futures.NewClient(exchange.ApiKey, exchange.ApiSecret)

	return client, nil
}

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

func (e Binance) GetSymbolPosition(symbol string) ([]aggregates.PositionRisk, *common.APIError) {
	var positions []aggregates.PositionRisk
	client, initErr := InitFuturesExchange(e)
	if initErr != nil {
		return nil, initErr
	}
	formattedSymbol := strings.Replace(symbol, "/", "", 1)
	positionsResponse, err := client.NewGetPositionRiskService().Symbol(formattedSymbol).Do(context.Background())

	copier.Copy(&positions, &positionsResponse)

	return positions, ApiError(err)
}

func (e Binance) SetSymbolLeverage(symbol string, leverage int) (aggregates.SymbolLeverage, *common.APIError) {
	var symbolLeverage aggregates.SymbolLeverage
	client, initErr := InitFuturesExchange(e)
	if initErr != nil {
		return symbolLeverage, initErr
	}

	formattedSymbol := strings.Replace(symbol, "/", "", 1)
	response, err := client.NewChangeLeverageService().
		Symbol(formattedSymbol).
		Leverage(leverage).
		Do(context.Background())

	copier.Copy(&symbolLeverage, &response)

	return symbolLeverage, ApiError(err)
}

func (e Binance) GetFuturesExchangeInfo() (aggregates.ExchangeInfo, *common.APIError) {
	var exchangeInfo aggregates.ExchangeInfo
	client, initErr := InitFuturesExchange(e)
	if initErr != nil {
		return exchangeInfo, initErr
	}
	exchangeInfoResponse, err := client.NewExchangeInfoService().Do(context.Background())
	copier.Copy(&exchangeInfo, &exchangeInfoResponse)
	return exchangeInfo, ApiError(err)
}
