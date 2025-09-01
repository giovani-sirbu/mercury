package binanceAdaptor

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/adshao/go-binance/v2/common"
	"github.com/adshao/go-binance/v2/futures"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
	"github.com/jinzhu/copier"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
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

// ModifyFuturesOrder modifies an existing futures order on Binance
func (e Binance) ModifyFuturesOrderPrice(symbol string, orderID int64, price string) (aggregates.ModifyFuturesOrderResponse, *common.APIError) {
	// API endpoint
	const endpoint = "https://fapi.binance.com/fapi/v1/order"

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var response aggregates.ModifyFuturesOrderResponse

	oldOrder, getOrderErr := e.GetOrderById(symbol, orderID)

	if getOrderErr != nil {
		return response, ApiError(fmt.Errorf("failed to get old order request: %v", getOrderErr))
	}

	// Prepare query parameters
	params := url.Values{}
	params.Add("symbol", oldOrder.Symbol)
	params.Add("orderId", strconv.FormatInt(orderID, 10))
	params.Add("price", price)
	params.Add("side", oldOrder.Side)
	params.Add("quantity", oldOrder.OrigQuantity)
	params.Add("type", oldOrder.Type)
	params.Add("timestamp", strconv.FormatInt(time.Now().UnixMilli(), 10))
	params.Add("recvWindow", "5000") // Optional: adjust as needed

	// Create HMAC-SHA256 signature
	queryString := params.Encode()
	hmac := hmac.New(sha256.New, []byte(e.ApiSecret))
	hmac.Write([]byte(queryString))
	signature := hex.EncodeToString(hmac.Sum(nil))
	params.Add("signature", signature)

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return response, ApiError(fmt.Errorf("failed to create request: %v", err))
	}

	// Set headers
	req.Header.Set("X-MBX-APIKEY", e.ApiKey)
	req.Header.Set("Content-Type", "application/json")

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return response, ApiError(fmt.Errorf("failed to send request: %v", err))
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		var errorResp struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&errorResp); err != nil {
			return response, ApiError(fmt.Errorf("failed to decode error response: %v", err))
		}
		return response, ApiError(fmt.Errorf("API error: code=%d, msg=%s", errorResp.Code, errorResp.Msg))
	}

	// Parse response
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return response, ApiError(fmt.Errorf("failed to parse response: %v", err))
	}

	return response, nil
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

	copier.Copy(&incomeHistory, &income)
	return incomeHistory, ApiError(incomeErr)
}
