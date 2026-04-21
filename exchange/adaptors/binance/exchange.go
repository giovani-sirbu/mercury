package binanceAdaptor

import (
	"github.com/adshao/go-binance/v2"
	"github.com/adshao/go-binance/v2/common"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
)

// Binance is the embedded Exchange-configured struct that every method on
// the binance adaptor hangs off. The individual method groups live in sibling
// files under this package:
//
//   - orders.go       — Buy / Sell / GetOrder / CancelOrder / GetTrades
//   - accountInfo.go  — GetProfile / GetUserAssets / APIKeyPermission
//   - marketData.go   — GetExchangeInfo / GetFees / GetPrice / Klines / AggTrades
//   - userStream.go   — StartUserStream / PingUserStream
//   - ws.go           — PriceWSHandler / UserWs (context-driven WS)
//   - futures.go      — futures-only endpoints (FuturesActions surface)
type Binance struct {
	aggregates.Exchange
}

// ApiError normalises an arbitrary error into the exported APIError shape.
// The nil-error short-circuit matters because callers often assign the
// result into a *common.APIError field and check it later; a naked wrap
// of nil would produce a non-nil wrapper and break that check.
func ApiError(err error) *common.APIError {
	if err == nil {
		return nil
	}

	if apiErr, ok := err.(*common.APIError); ok {
		return &common.APIError{
			Code:    apiErr.Code,
			Message: apiErr.Message,
		}
	}

	return &common.APIError{
		Code:    0,
		Message: err.Error(),
	}
}

// GetBinanceActions builds the Actions function-bag for a spot exchange. The
// struct exists so custom exchanges (virtual/backtesting) can override only
// the subset they need; mercury's consumers always go through this surface
// rather than the Binance struct directly.
func GetBinanceActions(e aggregates.Exchange) aggregates.Actions {
	b := Binance{e}
	return aggregates.Actions{
		Buy:              b.Buy,
		Sell:             b.Sell,
		MarketBuy:        b.MarketBuy,
		MarketSell:       b.MarketSell,
		GetOrder:         b.GetOrder,
		CancelOrder:      b.CancelOrder,
		GetTrades:        b.GetTrades,
		GetExchangeInfo:  b.GetExchangeInfo,
		GetFees:          b.GetFees,
		GetProfile:       b.GetProfile,
		GetPrice:         b.GetPrice,
		GetUserAssets:    b.GetUserAssets,
		PriceWSHandler:   b.PriceWSHandler,
		UserWSHandler:    b.UserWs,
		StartUserStream:  b.StartUserStream,
		PingUserStream:   b.PingUserStream,
		AggTrades:        b.AggTrades,
		KlineData:        b.Klines,
		APIKeyPermission: b.APIKeyPermission,
	}
}

// InitExchange constructs an authenticated binance client. TestNet toggles
// the library-level global, so concurrent use across mainnet + testnet
// exchanges within the same process is not safe — accepted trade-off given
// that every deployment pins to one network.
func InitExchange(exchange Binance) (*binance.Client, *common.APIError) {
	if exchange.TestNet {
		binance.UseTestnet = true
	} else {
		binance.UseTestnet = false
	}
	return binance.NewClient(exchange.ApiKey, exchange.ApiSecret), nil
}
