package virtualExchange

import (
	"fmt"
	"github.com/adshao/go-binance/v2/common"
	"github.com/giovani-sirbu/mercury/exchange"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Order struct {
	Symbol           string  `json:"symbol"`
	OrderID          int64   `json:"orderId"`
	Price            string  `json:"price"`
	Quantity         float64 `json:"quantity"`
	ExecutedQuantity string  `json:"executedQuantity"`
	Status           string  `json:"status"`
	Side             string  `json:"side"`
}

var Wallet []aggregates.UserAssetRecord
var Orders = make(map[int]Order)
var CurrentAggTrades = make(map[string]aggregates.AggTradesResponse)
var Prices = make(map[string]float64)
var rwMutex sync.RWMutex

func ParseToFloat(s string) float64 {
	i, _ := strconv.ParseFloat(s, 64)
	return i
}

func ResetWallet() {
	Orders = make(map[int]Order)
	Wallet = []aggregates.UserAssetRecord{}
	CurrentAggTrades = make(map[string]aggregates.AggTradesResponse)
	Prices = make(map[string]float64)
}

func UpdateWalletBalance(asset string, qty float64) {
	assetsExist := false
	for i, wallet := range Wallet {
		if wallet.Asset == asset {
			qtyInString := fmt.Sprintf("%f", ParseToFloat(Wallet[i].Free)+qty)
			Wallet[i].Free = qtyInString
			assetsExist = true
		}
	}
	if !assetsExist {
		Wallet = append(Wallet, aggregates.UserAssetRecord{Asset: asset, Free: fmt.Sprintf("%f", qty)})
	}
}

func UpdateWalletBalances(symbol string, price string, quantity float64, side string) {
	symbols := strings.Split(symbol, "/")
	floatPrice := ParseToFloat(price)
	baseSymbol := symbols[0]
	quoteSymbol := symbols[1]
	baseQty := quantity
	quoteQty := quantity * floatPrice

	if side == "BUY" {
		quoteQty *= -1
	} else {
		baseQty *= -1
	}

	UpdateWalletBalance(baseSymbol, baseQty)
	UpdateWalletBalance(quoteSymbol, quoteQty)
}

func SetCurrentAggTradeBySymbol(symbol string, aggTrade aggregates.AggTradesResponse) {
	CurrentAggTrades[symbol] = aggTrade
}

func GetCurrentAggTradeBySymbol(symbol string) aggregates.AggTradesResponse {
	if len(CurrentAggTrades) > 0 {
		for i, _ := range CurrentAggTrades {
			if i == symbol {
				return CurrentAggTrades[i]
			}
		}
	}

	return aggregates.AggTradesResponse{}
}

func GetPartiallyFilledOrderBySymbol(symbol string) Order {
	if len(Orders) > 0 {
		for _, order := range Orders {
			if order.Symbol == symbol && order.Status == "PARTIALLY_FILLED" {
				return order
			}
		}
	}

	return Order{}
}

func setOrderData(order Order) (aggregates.CreateOrderResponse, *common.APIError) {
	orderId := time.Now().UnixMilli()
	order.OrderID = orderId
	if order.Status == "" {
		order.Status = "FILLED"
	}

	rwMutex.RLock()
	currentOrder := GetCurrentAggTradeBySymbol(order.Symbol)
	rwMutex.RUnlock()

	// run only to backtesting, skip for manual testing
	if currentOrder.AggTradeID != 0 {
		// fix price for market buy/sell
		if order.Price == "" {
			order.Price = currentOrder.Price
		}

		currentOrderQty := ParseToFloat(currentOrder.Quantity)

		order.ExecutedQuantity = strconv.FormatFloat(order.Quantity, 'f', -1, 64)

		// update partially filled orders
		partiallyFilledOrder := GetPartiallyFilledOrderBySymbol(order.Symbol)

		// if not enough available quantity, set partial fill
		if partiallyFilledOrder.OrderID == 0 && currentOrderQty < order.Quantity && order.Status == "FILLED" {
			order.Status = "PARTIALLY_FILLED"
			order.ExecutedQuantity = currentOrder.Quantity
		}
	}

	// set order
	Orders[int(orderId)] = order

	// return orderID
	return aggregates.CreateOrderResponse{
		OrderID: orderId,
	}, nil
}

func setCustomActions(customActions aggregates.Actions, assets []aggregates.UserAssetRecord) aggregates.Actions {
	customActions.GetUserAssets = func() ([]aggregates.UserAssetRecord, *common.APIError) {
		if len(Wallet) == 0 {
			Wallet = assets
		}
		return Wallet, nil
	}
	customActions.Buy = func(symbol string, quantity float64, price string) (aggregates.CreateOrderResponse, *common.APIError) {
		payload := Order{
			Symbol:   symbol,
			Quantity: quantity,
			Price:    price,
			Side:     "BUY",
		}
		UpdateWalletBalances(symbol, price, quantity, "BUY")
		data, err := setOrderData(payload)
		if err != nil {
			return aggregates.CreateOrderResponse{}, &common.APIError{Message: err.Error()}
		}
		return data, nil
	}
	customActions.MarketBuy = func(symbol string, quantity float64) (aggregates.CreateOrderResponse, *common.APIError) {
		payload := Order{
			Symbol:   symbol,
			Quantity: quantity,
			Side:     "BUY",
		}
		data, err := setOrderData(payload)
		price := fmt.Sprintf("%f", Prices[symbol])
		UpdateWalletBalances(symbol, price, quantity, "BUY")
		if err != nil {
			return aggregates.CreateOrderResponse{}, err
		}
		return data, nil
	}
	customActions.Sell = func(symbol string, quantity float64, price string) (aggregates.CreateOrderResponse, *common.APIError) {
		payload := Order{
			Symbol:   symbol,
			Quantity: quantity,
			Price:    price,
			Side:     "SELL",
		}
		data, err := setOrderData(payload)
		UpdateWalletBalances(symbol, price, quantity, "SELL")
		if err != nil {
			return aggregates.CreateOrderResponse{}, err
		}
		return data, nil
	}
	customActions.MarketSell = func(symbol string, quantity float64) (aggregates.CreateOrderResponse, *common.APIError) {
		payload := Order{
			Symbol:   symbol,
			Quantity: quantity,
			Side:     "BUY",
		}
		data, err := setOrderData(payload)
		price := fmt.Sprintf("%f", Prices[symbol])
		UpdateWalletBalances(symbol, price, quantity, "SELL")
		if err != nil {
			return aggregates.CreateOrderResponse{}, err
		}
		return data, nil
	}

	customActions.CancelOrder = func(orderId int64, symbol string) (aggregates.CancelOrderResponse, *common.APIError) {
		delete(Orders, int(orderId))
		return aggregates.CancelOrderResponse{
			Symbol:  symbol,
			OrderID: orderId,
		}, nil
	}

	customActions.GetPrice = func(symbol string) (float64, *common.APIError) {
		var price = Prices[symbol]
		return price, nil
	}

	return customActions
}

func InitVirtualExchange(assets []aggregates.UserAssetRecord) exchange.Exchange {
	var customActions aggregates.Actions
	exchangeInit := exchange.Exchange{
		Name:          "binance",
		CustomActions: setCustomActions(customActions, assets),
		IsCustom:      true,
	}
	return exchangeInit
}

func UpdateVirtualOrderTrade(trade Order) {
	if len(Orders) > 0 {
		for i, order := range Orders {
			if order.OrderID == trade.OrderID {
				Orders[i] = trade
			}
		}
	}
}
