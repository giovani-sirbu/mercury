package aggregates

import (
	"context"

	"github.com/adshao/go-binance/v2/common"
)

// Actions is the function bag a spot exchange adaptor populates; an unassigned field is nil.
type Actions struct {
	Buy              func(symbol string, quantity float64, price string) (CreateOrderResponse, *common.APIError)
	Sell             func(symbol string, quantity float64, price string) (CreateOrderResponse, *common.APIError)
	MarketBuy        func(symbol string, quantity float64) (CreateOrderResponse, *common.APIError)
	MarketSell       func(symbol string, quantity float64) (CreateOrderResponse, *common.APIError)
	GetOrder         func(orderId int64, symbol string) (Order, *common.APIError)
	CancelOrder      func(orderId int64, symbol string) (CancelOrderResponse, *common.APIError)
	GetTrades        func(orderId int64, symbol string) ([]Trade, *common.APIError)
	GetExchangeInfo  func(symbol string) (ExchangeInfo, *common.APIError)
	GetFees          func(symbol string) (TradeFeeDetails, *common.APIError)
	GetPrice         func(symbol string) (float64, *common.APIError)
	GetPrices        func() (map[string]float64, *common.APIError)
	GetProfile       func() (Account, *common.APIError)
	GetUserAssets    func() ([]UserAssetRecord, *common.APIError)
	PriceWSHandler   func(ctx context.Context, pairs []string, handler func(PriceWSResponseData)) error
	UserWSHandler    func(ctx context.Context, listenKey string, handler func(order WsUserDataEvent, expireEvent string)) error
	PingUserStream   func(listenKey string) *common.APIError
	StartUserStream  func() (string, *common.APIError)
	AggTrades        func(payload AggTradesPayload) ([]AggTradesResponse, *common.APIError)
	KlineData        func(KlinePayload) ([]KlineResponse, *common.APIError)
	APIKeyPermission func() (APIKeyPermission, *common.APIError)
}

// FuturesActions is the futures-only counterpart to Actions.
type FuturesActions struct {
	CreateFuturesOrder      func(sideType string, orderType string, symbol string, quantity string, price string, reduceOnly bool) (CreateOrderResponse, *common.APIError)
	ModifyFuturesOrderPrice func(symbol string, orderId int64, price string) (CreateOrderResponse, *common.APIError)
	ListOrders              func(symbol string) ([]FuturesOrder, *common.APIError)
	GetOrderById            func(symbol string, orderId int64) (FuturesOrder, *common.APIError)
	CancelOrders            func(symbol string, orderId int64) (CancelFuturesOrderResponse, *common.APIError)
	GetSymbolPosition       func(symbol string) ([]PositionRisk, *common.APIError)
	SetSymbolLeverage       func(symbol string, leverage int) (SymbolLeverage, *common.APIError)
	GetFuturesExchangeInfo  func() (ExchangeInfo, *common.APIError)
	GetIncomeHistory        func(symbol string) ([]IncomeHistory, *common.APIError)
	GetFuturesBalance       func() ([]FuturesBalance, *common.APIError)
	KlineData               func(KlinePayload) ([]KlineResponse, *common.APIError)
}
