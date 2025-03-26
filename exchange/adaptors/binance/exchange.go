package binanceAdaptor

import (
	"context"
	"fmt"
	"github.com/adshao/go-binance/v2"
	"github.com/adshao/go-binance/v2/common"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
	"github.com/jinzhu/copier"
	"strconv"
	"strings"
)

// Binance structure to init the binance actions
type Binance struct {
	aggregates.Exchange
}

// ApiError converts a given error into an APIError
func ApiError(err error) *common.APIError {
	if err == nil {
		return nil // Return nil if there's no error
	}

	// Check if the error is already an APIError
	if apiErr, ok := err.(*common.APIError); ok {
		return &common.APIError{
			Code:    apiErr.Code,
			Message: apiErr.Message,
		}
	}

	// Check if the error is a Binance API error
	if binanceErr, ok := err.(*common.APIError); ok {
		return &common.APIError{
			Code:    int64(binanceErr.Code),
			Message: binanceErr.Message,
		}
	}

	// Handle other unknown errors as a generic APIError
	return &common.APIError{
		Code:    0, // No specific error code
		Message: err.Error(),
	}
}

// GetBinanceActions map all binance actions
func GetBinanceActions(e aggregates.Exchange) aggregates.Actions {
	var binanceStruct = Binance{e}
	var actions = aggregates.Actions{
		Buy:             binanceStruct.Buy,
		Sell:            binanceStruct.Sell,
		MarketBuy:       binanceStruct.MarketBuy,
		MarketSell:      binanceStruct.MarketSell,
		GetOrder:        binanceStruct.GetOrder,
		CancelOrder:     binanceStruct.CancelOrder,
		GetTrades:       binanceStruct.GetTrades,
		GetExchangeInfo: binanceStruct.GetExchangeInfo,
		GetFees:         binanceStruct.GetFees,
		GetProfile:      binanceStruct.GetProfile,
		GetPrice:        binanceStruct.GetPrice,
		GetUserAssets:   binanceStruct.GetUserAssets,
		PriceWSHandler:  binanceStruct.PriceWSHandler,
		UserWSHandler:   binanceStruct.UserWs,
		StartUserStream: binanceStruct.StartUserStream,
		PingUserStream:  binanceStruct.PingUserStream,
		AggTrades:       binanceStruct.AggTrades,
		KlineData:       binanceStruct.Klines,
	}
	return actions
}

// InitExchange Function to initiate binance client
func InitExchange(exchange Binance) (*binance.Client, *common.APIError) {
	if exchange.TestNet {
		binance.UseTestnet = true
	} else {
		binance.UseTestnet = false
	}

	var client = binance.NewClient(exchange.ApiKey, exchange.ApiSecret)

	return client, nil
}

// Buy binance buy method
func (e Binance) Buy(symbol string, quantity float64, price string) (aggregates.CreateOrderResponse, *common.APIError) {
	var actionResult aggregates.CreateOrderResponse

	result, err := e.binanceCreateOrder(binance.SideTypeBuy, binance.OrderTypeLimit, symbol, quantity, price)
	copier.Copy(&actionResult, &result)
	return actionResult, err
}

// Sell binance buy method
func (e Binance) Sell(symbol string, quantity float64, price string) (aggregates.CreateOrderResponse, *common.APIError) {
	var actionResult aggregates.CreateOrderResponse
	result, err := e.binanceCreateOrder(binance.SideTypeSell, binance.OrderTypeLimit, symbol, quantity, price)
	copier.Copy(&actionResult, &result)
	return actionResult, err
}

// MarketBuy binance market buy method
func (e Binance) MarketBuy(symbol string, quantity float64) (aggregates.CreateOrderResponse, *common.APIError) {
	var actionResult aggregates.CreateOrderResponse
	result, err := e.binanceCreateOrder(binance.SideTypeBuy, binance.OrderTypeMarket, symbol, quantity, "")
	copier.Copy(&actionResult, &result)
	return actionResult, err
}

// MarketSell binance market sell method
func (e Binance) MarketSell(symbol string, quantity float64) (aggregates.CreateOrderResponse, *common.APIError) {
	var actionResult aggregates.CreateOrderResponse
	result, err := e.binanceCreateOrder(binance.SideTypeSell, binance.OrderTypeMarket, symbol, quantity, "")
	copier.Copy(&actionResult, &result)
	return actionResult, err
}

// GetOrder binance buy method
func (e Binance) GetOrder(orderId int64, symbol string) (aggregates.Order, *common.APIError) {
	formattedSymbol := strings.Replace(symbol, "/", "", 1) // Format symbol as exchange need it
	client, initErr := InitExchange(e)                     // Init exchange
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

// CancelOrder binance cancel an order method
func (e Binance) CancelOrder(orderId int64, symbol string) (aggregates.CancelOrderResponse, *common.APIError) {
	formattedSymbol := strings.Replace(symbol, "/", "", 1) // Format symbol as exchange need it
	client, initErr := InitExchange(e)                     // Init exchange
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

// GetTrades binance get trades by order id
func (e Binance) GetTrades(orderId int64, symbol string) ([]aggregates.Trade, *common.APIError) {
	formattedSymbol := strings.Replace(symbol, "/", "", 1) // Format symbol as exchange need it
	client, initErr := InitExchange(e)                     // Init exchange
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

// GetExchangeInfo binance fetch exchange info
func (e Binance) GetExchangeInfo(symbol string) (aggregates.ExchangeInfo, *common.APIError) {
	client, initErr := InitExchange(e) // Init exchange
	if initErr != nil {
		return aggregates.ExchangeInfo{}, initErr
	}
	formattedSymbol := strings.Replace(symbol, "/", "", 1) // Format symbol as exchange need it
	details, detailsErr := client.NewExchangeInfoService().Symbol(formattedSymbol).Do(context.Background())
	if detailsErr != nil {
		return aggregates.ExchangeInfo{}, ApiError(detailsErr)
	}
	var exchangeInfoResult aggregates.ExchangeInfo
	copier.Copy(&exchangeInfoResult, &details)
	return exchangeInfoResult, nil
}

// GetFees binance get fees details
func (e Binance) GetFees(symbol string) (aggregates.TradeFeeDetails, *common.APIError) {
	client, initErr := InitExchange(e) // Init exchange
	if initErr != nil {
		return aggregates.TradeFeeDetails{}, initErr
	}
	formattedSymbol := strings.Replace(symbol, "/", "", 1) // Format symbol as exchange need it
	details, detailsErr := client.NewTradeFeeService().Symbol(formattedSymbol).Do(context.Background())

	if detailsErr != nil {
		return aggregates.TradeFeeDetails{}, ApiError(detailsErr)
	}

	var exchangeInfoResult aggregates.TradeFeeDetails
	copier.Copy(&exchangeInfoResult, &details[0])
	return exchangeInfoResult, nil
}

// GetProfile binance get user details
func (e Binance) GetProfile() (aggregates.Account, *common.APIError) {
	client, initErr := InitExchange(e)
	if initErr != nil {
		return aggregates.Account{}, initErr
	}
	clientInfo, err := client.NewGetAccountService().Do(context.Background())

	if err != nil {
		return aggregates.Account{}, ApiError(err)
	}

	var clientInfoResult aggregates.Account
	copier.Copy(&clientInfoResult, &clientInfo)
	return clientInfoResult, nil
}

// GetUserAssets binance get user assets
func (e Binance) GetUserAssets() ([]aggregates.UserAssetRecord, *common.APIError) {
	client, initErr := InitExchange(e)
	if initErr != nil {
		return nil, initErr
	}
	clientInfo, err := client.NewGetUserAsset().NeedBtcValuation(true).Do(context.Background())
	if err != nil {
		profileInfo, profileErr := e.GetProfile()
		if profileErr != nil {
			return nil, profileErr
		}
		return profileInfo.Balances, nil
	}
	var clientInfoResult []aggregates.UserAssetRecord
	copier.Copy(&clientInfoResult, &clientInfo)
	return clientInfoResult, nil
}

// GetPrice binance get symbol current price
func (e Binance) GetPrice(symbol string) (float64, *common.APIError) {
	var price float64
	formattedSymbol := strings.Replace(symbol, "/", "", 1)

	client, initErr := InitExchange(e)
	if initErr != nil {
		return price, initErr
	}
	clientInfo, err := client.NewAveragePriceService().Symbol(formattedSymbol).Do(context.Background())
	if err != nil {
		return price, ApiError(err)
	}
	price, _ = strconv.ParseFloat(clientInfo.Price, 64)
	return price, nil
}

// BinanceCreateOrder binance create order helper
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

// StartUserStream start a new user stream
func (e Binance) StartUserStream() (string, *common.APIError) {
	client, initErr := InitExchange(e)
	if initErr != nil {
		return "", initErr
	}
	clientInfo, err := client.NewStartUserStreamService().Do(context.Background())
	if err != nil {
		return clientInfo, ApiError(err)
	}
	return clientInfo, nil
}

// PingUserStream keep alive a new user stream
func (e Binance) PingUserStream(listenKey string) *common.APIError {
	client, initErr := InitExchange(e)
	if initErr != nil {
		return initErr
	}
	err := client.NewKeepaliveUserStreamService().ListenKey(listenKey).Do(context.Background())
	if err != nil {
		return ApiError(err)
	}
	return nil
}

// Klines Kline/candlestick bars for a symbol. Klines are uniquely identified by their open time.
func (e Binance) Klines(payload aggregates.KlinePayload) ([]aggregates.KlineResponse, *common.APIError) {
	client, initErr := InitExchange(e)
	if initErr != nil {
		return nil, initErr
	}
	clientQuery := client.NewKlinesService().Symbol(payload.Symbol).Interval(payload.Interval)

	if payload.StartTime > 0 {
		clientQuery.StartTime(payload.StartTime)
	}

	if payload.EndTime > 0 {
		clientQuery.EndTime(payload.EndTime)
	}

	if payload.Limit > 0 {
		clientQuery.Limit(payload.Limit)
	}

	clientData, err := clientQuery.Do(context.Background())
	if err != nil {
		return nil, ApiError(err)
	}

	var data []aggregates.KlineResponse
	copier.Copy(&data, &clientData)
	return data, nil
}

// AggTrades Get compressed, aggregate trades. Trades that fill at the time, from the same order,
// with the same price will have the quantity aggregated.
func (e Binance) AggTrades(payload aggregates.AggTradesPayload) ([]aggregates.AggTradesResponse, *common.APIError) {
	client, initErr := InitExchange(e)
	if initErr != nil {
		return nil, initErr
	}
	clientQuery := client.NewAggTradesService().Symbol(payload.Symbol)

	if payload.FromId > 0 {
		clientQuery.FromID(payload.FromId)
	}

	if payload.StartTime > 0 {
		clientQuery.StartTime(payload.StartTime)
	}

	if payload.EndTime > 0 {
		clientQuery.EndTime(payload.EndTime)
	}

	if payload.Limit > 0 {
		clientQuery.Limit(payload.Limit)
	}

	clientData, err := clientQuery.Do(context.Background())
	if err != nil {
		return nil, ApiError(err)
	}

	var data []aggregates.AggTradesResponse
	copier.Copy(&data, &clientData)
	return data, nil
}
