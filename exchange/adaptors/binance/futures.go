package binanceAdaptor

import (
	"context"
	"github.com/adshao/go-binance/v2/common"
	"github.com/adshao/go-binance/v2/futures"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
	"github.com/jinzhu/copier"
	"strconv"
	"strings"
)

// GetFuturesBinanceActions map all binance actions
func GetFuturesBinanceActions(e aggregates.Exchange) aggregates.FuturesActions {
	var binanceStruct = Binance{e}
	var actions = aggregates.FuturesActions{
		CreateFuturesOrder:      binanceStruct.CreateFutureOrder,
		ModifyFuturesOrderPrice: binanceStruct.ModifyFuturesOrderPrice,
		ListOrders:              binanceStruct.ListOrders,
		GetOrderById:            binanceStruct.GetOrderById,
		CancelOrders:            binanceStruct.CancelOrders,
		GetSymbolPosition:       binanceStruct.GetSymbolPosition,
		SetSymbolLeverage:       binanceStruct.SetSymbolLeverage,
		GetFuturesExchangeInfo:  binanceStruct.GetFuturesExchangeInfo,
		GetIncomeHistory:        binanceStruct.GetIncomeHistory,
		GetFuturesBalance:       binanceStruct.GetFuturesBalance,
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

func (e Binance) ModifyFuturesOrderPrice(symbol string, orderId int64, newPrice string) (aggregates.CreateOrderResponse, *common.APIError) {
	var updateResponse aggregates.CreateOrderResponse

	client, initErr := InitFuturesExchange(e)
	if initErr != nil {
		return updateResponse, initErr
	}

	formattedSymbol := strings.Replace(symbol, "/", "", 1)

	// 1. Cancel the existing order
	cancelResp, err := client.NewCancelOrderService().
		Symbol(formattedSymbol).
		OrderID(orderId).
		Do(context.Background())
	if err != nil {
		return updateResponse, ApiError(err)
	}

	// Parse new stop price
	stopPrice, _ := strconv.ParseFloat(newPrice, 64)

	// 2. Get current mark price
	markPriceResp, err := client.NewPremiumIndexService().
		Symbol(formattedSymbol).
		Do(context.Background())
	if err != nil {
		return updateResponse, ApiError(err)
	}
	currPrice, _ := strconv.ParseFloat(markPriceResp[0].MarkPrice, 64)

	// 3. Validate stop price relative to current price
	if cancelResp.Side == futures.SideTypeSell && stopPrice >= currPrice {
		return updateResponse, &common.APIError{Message: "Invalid stop: sell stop must be below current price"}
	}
	if cancelResp.Side == futures.SideTypeBuy && stopPrice <= currPrice {
		return updateResponse, &common.APIError{Message: "Invalid stop: buy stop must be above current price"}
	}

	// 4. Check position before setting ReduceOnly
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

	// 5. Place a new STOP_MARKET order with validated params
	newOrder, err := client.NewCreateOrderService().
		Symbol(formattedSymbol).
		Side(cancelResp.Side).
		Type(futures.OrderTypeStopMarket). // force STOP_MARKET
		Quantity(cancelResp.OrigQuantity).
		StopPrice(newPrice).
		ReduceOnly(reduceOnly).
		WorkingType(futures.WorkingTypeMarkPrice). // safer trigger
		Do(context.Background())
	if err != nil {
		return updateResponse, ApiError(err)
	}

	copier.Copy(&updateResponse, &newOrder)
	return updateResponse, nil
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

func (e Binance) GetIncomeHistory(symbol string) ([]aggregates.IncomeHistory, *common.APIError) {
	var incomeHistory []aggregates.IncomeHistory
	client, initErr := InitFuturesExchange(e)
	if initErr != nil {
		return incomeHistory, initErr
	}
	formattedSymbol := strings.Replace(symbol, "/", "", 1)
	income, incomeErr := client.NewGetIncomeHistoryService().
		Symbol(formattedSymbol).
		IncomeType("REALIZED_PNL").
		Do(context.Background())

	fees, feesErr := client.NewGetIncomeHistoryService().
		Symbol(formattedSymbol).
		IncomeType("COMMISSION").
		Do(context.Background())

	if feesErr != nil {
		incomeErr = feesErr
	}

	income = append(income, fees...)

	copier.Copy(&incomeHistory, &income)
	return incomeHistory, ApiError(incomeErr)
}

func (e Binance) GetFuturesBalance() ([]aggregates.FuturesBalance, *common.APIError) {
	var balance []aggregates.FuturesBalance
	client, initErr := InitFuturesExchange(e)
	if initErr != nil {
		return balance, initErr
	}
	income, incomeErr := client.NewGetBalanceService().Do(context.Background())

	copier.Copy(&balance, &income)
	return balance, ApiError(incomeErr)
}
