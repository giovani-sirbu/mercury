package binanceAdaptor

import (
	"context"
	"fmt"
	"github.com/adshao/go-binance/v2"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
	"github.com/jinzhu/copier"
	"strconv"
	"strings"
)

// Binance structure to init the binance actions
type Binance struct {
	aggregates.Exchange
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
	}
	return actions
}

// InitExchange Function to initiate binance client
func InitExchange(exchange Binance) (*binance.Client, error) {
	if exchange.TestNet {
		binance.UseTestnet = true
	} else {
		binance.UseTestnet = false
	}

	var client = binance.NewClient(exchange.ApiKey, exchange.ApiSecret)

	return client, nil
}

// Buy binance buy method
func (e Binance) Buy(symbol string, quantity float64, price string) (aggregates.CreateOrderResponse, error) {
	var actionResult aggregates.CreateOrderResponse

	result, err := e.binanceCreateOrder(binance.SideTypeBuy, binance.OrderTypeLimit, symbol, quantity, price)
	copier.Copy(&actionResult, &result)
	return actionResult, err
}

// Sell binance buy method
func (e Binance) Sell(symbol string, quantity float64, price string) (aggregates.CreateOrderResponse, error) {
	var actionResult aggregates.CreateOrderResponse
	result, err := e.binanceCreateOrder(binance.SideTypeSell, binance.OrderTypeLimit, symbol, quantity, price)
	copier.Copy(&actionResult, &result)
	return actionResult, err
}

// MarketBuy binance market buy method
func (e Binance) MarketBuy(symbol string, quantity float64) (aggregates.CreateOrderResponse, error) {
	var actionResult aggregates.CreateOrderResponse
	result, err := e.binanceCreateOrder(binance.SideTypeBuy, binance.OrderTypeMarket, symbol, quantity, "")
	copier.Copy(&actionResult, &result)
	return actionResult, err
}

// MarketSell binance market sell method
func (e Binance) MarketSell(symbol string, quantity float64) (aggregates.CreateOrderResponse, error) {
	var actionResult aggregates.CreateOrderResponse
	result, err := e.binanceCreateOrder(binance.SideTypeSell, binance.OrderTypeMarket, symbol, quantity, "")
	copier.Copy(&actionResult, &result)
	return actionResult, err
}

// GetOrder binance buy method
func (e Binance) GetOrder(orderId int64, symbol string) (aggregates.Order, error) {
	formattedSymbol := strings.Replace(symbol, "/", "", 1) // Format symbol as exchange need it
	client, initErr := InitExchange(e)                     // Init exchange
	if initErr != nil {
		return aggregates.Order{}, initErr
	}
	order, err := client.NewGetOrderService().Symbol(formattedSymbol).
		OrderID(orderId).Do(context.Background())
	if err != nil {
		return aggregates.Order{}, err
	}
	var orderResult aggregates.Order
	copier.Copy(&orderResult, &order)
	return orderResult, nil
}

// CancelOrder binance cancel an order method
func (e Binance) CancelOrder(orderId int64, symbol string) (aggregates.CancelOrderResponse, error) {
	formattedSymbol := strings.Replace(symbol, "/", "", 1) // Format symbol as exchange need it
	client, initErr := InitExchange(e)                     // Init exchange
	if initErr != nil {
		return aggregates.CancelOrderResponse{}, initErr
	}
	order, err := client.NewCancelOrderService().Symbol(formattedSymbol).
		OrderID(orderId).Do(context.Background())
	if err != nil {
		return aggregates.CancelOrderResponse{}, err
	}

	var orderResult aggregates.CancelOrderResponse
	copier.Copy(&orderResult, &order)
	return orderResult, nil
}

// GetTrades binance get trades by order id
func (e Binance) GetTrades(orderId int64, symbol string) ([]aggregates.Trade, error) {
	formattedSymbol := strings.Replace(symbol, "/", "", 1) // Format symbol as exchange need it
	client, initErr := InitExchange(e)                     // Init exchange
	if initErr != nil {
		return nil, initErr
	}
	order, err := client.NewListTradesService().Symbol(formattedSymbol).
		OrderId(orderId).Do(context.Background())
	if err != nil {
		return nil, err
	}
	var orderResult []aggregates.Trade
	copier.Copy(&orderResult, &order)
	return orderResult, nil
}

// GetExchangeInfo binance fetch exchange info
func (e Binance) GetExchangeInfo(symbol string) (aggregates.ExchangeInfo, error) {
	client, initErr := InitExchange(e) // Init exchange
	if initErr != nil {
		return aggregates.ExchangeInfo{}, initErr
	}
	formattedSymbol := strings.Replace(symbol, "/", "", 1) // Format symbol as exchange need it
	details, detailsErr := client.NewExchangeInfoService().Symbol(formattedSymbol).Do(context.Background())
	if detailsErr != nil {
		return aggregates.ExchangeInfo{}, detailsErr
	}
	var exchangeInfoResult aggregates.ExchangeInfo
	copier.Copy(&exchangeInfoResult, &details)
	return exchangeInfoResult, nil
}

// GetFees binance get fees details
func (e Binance) GetFees(symbol string) (aggregates.TradeFeeDetails, error) {
	client, initErr := InitExchange(e) // Init exchange
	if initErr != nil {
		return aggregates.TradeFeeDetails{}, initErr
	}
	formattedSymbol := strings.Replace(symbol, "/", "", 1) // Format symbol as exchange need it
	details, detailsErr := client.NewTradeFeeService().Symbol(formattedSymbol).Do(context.Background())

	if detailsErr != nil {
		return aggregates.TradeFeeDetails{}, detailsErr
	}

	var exchangeInfoResult aggregates.TradeFeeDetails
	copier.Copy(&exchangeInfoResult, &details[0])
	return exchangeInfoResult, nil
}

// GetProfile binance get user details
func (e Binance) GetProfile() (aggregates.Account, error) {
	client, initErr := InitExchange(e)
	if initErr != nil {
		return aggregates.Account{}, initErr
	}
	clientInfo, err := client.NewGetAccountService().Do(context.Background())

	if err != nil {
		return aggregates.Account{}, err
	}

	var clientInfoResult aggregates.Account
	copier.Copy(&clientInfoResult, &clientInfo)
	return clientInfoResult, nil
}

// GetUserAssets binance get user assets
func (e Binance) GetUserAssets() ([]aggregates.UserAssetRecord, error) {
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
func (e Binance) GetPrice(symbol string) (float64, error) {
	var price float64
	formattedSymbol := strings.Replace(symbol, "/", "", 1)

	client, initErr := InitExchange(e)
	if initErr != nil {
		return price, initErr
	}
	clientInfo, err := client.NewAveragePriceService().Symbol(formattedSymbol).Do(context.Background())
	if err != nil {
		return price, err
	}
	price, _ = strconv.ParseFloat(clientInfo.Price, 64)
	return price, nil
}

// BinanceCreateOrder binance create order helper
func (e Binance) binanceCreateOrder(sideType binance.SideType, orderType binance.OrderType, symbol string, quantity float64, price string) (*binance.CreateOrderResponse, error) {
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

	return order, err
}

// StartUserStream start a new user stream
func (e Binance) StartUserStream() (string, error) {
	client, initErr := InitExchange(e)
	if initErr != nil {
		return "", initErr
	}
	clientInfo, err := client.NewStartUserStreamService().Do(context.Background())
	if err != nil {
		return clientInfo, err
	}
	return clientInfo, nil
}

// PingUserStream keep alive a new user stream
func (e Binance) PingUserStream(listenKey string) error {
	client, initErr := InitExchange(e)
	if initErr != nil {
		return initErr
	}
	err := client.NewKeepaliveUserStreamService().ListenKey(listenKey).Do(context.Background())
	if err != nil {
		return err
	}
	return nil
}
