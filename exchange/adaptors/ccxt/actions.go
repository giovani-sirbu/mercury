package ccxt

import (
	"context"
	"fmt"

	"github.com/adshao/go-binance/v2/common"
	binanceAdaptor "github.com/giovani-sirbu/mercury/exchange/adaptors/binance"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
)

// GetCCXTActions assembles the function-bag for mercury's Actions interface
// backed by the official CCXT Go library. Every REST method is routed
// through CCXT; the WebSocket handlers stay on the legacy binance adaptor
// because CCXT v4 has no streaming support yet and migrating prices to a
// custom Binance WS client (session.logon + Ed25519) is the explicit
// follow-up PR scope.
//
// PriceWSHandler / UserWSHandler routing rules:
//   - Binance: reuse the existing aggTrade subscriber from
//     mercury/exchange/adaptors/binance/ws.go — verified working today.
//   - Crypto.com: no native WS yet. PriceWSHandler is nil; hermes will treat
//     this as "no live prices for this exchange" and fall back to its REST
//     polling path (already in place for the Crypto.com smoke test case).
//   - UserWSHandler is nil across the board — the user-data stream feature
//     is parked while the cron + Redis side-channel covers the operational
//     need at 2 min latency.
func GetCCXTActions(e aggregates.Exchange) aggregates.Actions {
	actions := aggregates.Actions{
		Buy: func(symbol string, quantity float64, price string) (aggregates.CreateOrderResponse, *common.APIError) {
			return buy(e, symbol, quantity, price)
		},
		Sell: func(symbol string, quantity float64, price string) (aggregates.CreateOrderResponse, *common.APIError) {
			return sell(e, symbol, quantity, price)
		},
		MarketBuy: func(symbol string, quantity float64) (aggregates.CreateOrderResponse, *common.APIError) {
			return marketBuy(e, symbol, quantity)
		},
		MarketSell: func(symbol string, quantity float64) (aggregates.CreateOrderResponse, *common.APIError) {
			return marketSell(e, symbol, quantity)
		},
		GetOrder: func(orderId int64, symbol string) (aggregates.Order, *common.APIError) {
			return getOrder(e, orderId, symbol)
		},
		CancelOrder: func(orderId int64, symbol string) (aggregates.CancelOrderResponse, *common.APIError) {
			return cancelOrder(e, orderId, symbol)
		},
		GetTrades: func(orderId int64, symbol string) ([]aggregates.Trade, *common.APIError) {
			return getTrades(e, orderId, symbol)
		},
		GetExchangeInfo: func(symbol string) (aggregates.ExchangeInfo, *common.APIError) {
			return getExchangeInfo(e, symbol)
		},
		GetFees: func(symbol string) (aggregates.TradeFeeDetails, *common.APIError) {
			return getFees(e, symbol)
		},
		GetPrice: func(symbol string) (float64, *common.APIError) {
			return getPrice(e, symbol)
		},
		GetProfile: func() (aggregates.Account, *common.APIError) {
			return getProfile(e)
		},
		GetUserAssets: func() ([]aggregates.UserAssetRecord, *common.APIError) {
			return getUserAssets(e)
		},
		APIKeyPermission: func() (aggregates.APIKeyPermission, *common.APIError) {
			return getAPIKeyPermission(e)
		},
		AggTrades: func(payload aggregates.AggTradesPayload) ([]aggregates.AggTradesResponse, *common.APIError) {
			return aggTrades(e, payload)
		},
		KlineData: func(payload aggregates.KlinePayload) ([]aggregates.KlineResponse, *common.APIError) {
			return klineData(e, payload)
		},

		// User-stream handlers are explicit stubs rather than nil. Mercury's
		// aggregates.Actions struct uses function fields, not an interface,
		// so a nil field plus a caller that invokes it without nil-checking
		// (agora/handlers/common/userWebSocket.go calls
		// `client.StartUserStream()` unconditionally when USER_WS_ENABLED is
		// true) crashes with a nil function panic. Returning an APIError keeps
		// the agora goroutine alive, surfaces the situation in logs, and lets
		// the cron + Redis side-channel keep running uninterrupted.
		//
		// When CCXT Pro Go ships WebSocket support (or we add a Binance
		// session.logon client in a follow-up), these three fields are the
		// one place to swap in real implementations.
		StartUserStream: func() (string, *common.APIError) {
			return "", &common.APIError{
				Code:    0,
				Message: "ccxt adaptor: user-data WS not supported; set USER_WS_ENABLED=false",
			}
		},
		PingUserStream: func(listenKey string) *common.APIError {
			return &common.APIError{
				Code:    0,
				Message: "ccxt adaptor: user-data WS not supported",
			}
		},
		UserWSHandler: func(ctx context.Context, listenKey string, handler func(order aggregates.WsUserDataEvent, expireEvent string)) error {
			return fmt.Errorf("ccxt adaptor: user-data WS not supported")
		},
	}

	// PriceWSHandler — borrow from the binance adaptor only for Binance,
	// since that's the one exchange whose WS stream we have a tested
	// subscriber for. Other exchanges: nil ⇒ hermes treats it as "no live
	// prices for this exchange" and skips spinning up a WS goroutine.
	if e.Name == ExchangeBinance {
		legacyBinance := binanceAdaptor.GetBinanceActions(e)
		actions.PriceWSHandler = legacyBinance.PriceWSHandler
	}

	return actions
}
