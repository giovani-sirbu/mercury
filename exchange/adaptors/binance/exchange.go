package binanceAdaptor

import (
	"fmt"
	"strings"

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
//
// The message is never empty: go-binance yields APIError{Code:0, Message:""}
// when a non-JSON body comes back (a /sapi/* redirect to HTML on testnet, a
// proxy page), and that blank used to surface to the user verbatim.
func ApiError(err error) *common.APIError {
	if err == nil {
		return nil
	}

	if apiErr, ok := err.(*common.APIError); ok {
		return &common.APIError{
			Code:    apiErr.Code,
			Message: describeExchangeError(apiErr.Code, apiErr.Message),
		}
	}

	return &common.APIError{
		Code:    0,
		Message: describeExchangeError(0, err.Error()),
	}
}

// describeExchangeError substitutes a meaningful message for blank ones.
func describeExchangeError(code int64, message string) string {
	if strings.TrimSpace(message) != "" {
		return message
	}
	if code == 0 {
		return "exchange returned an unreadable response (redirect or non-JSON body)"
	}
	return fmt.Sprintf("exchange error code %d (no message)", code)
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

// Spot REST base URLs. The network is selected PER CLIENT INSTANCE via
// Client.BaseURL — never through the process-wide binance.UseTestnet global —
// so mainnet and testnet accounts trade safely side by side in one process.
// Public market-data helpers that build their own key-less client (sophos)
// and the hermes price stream (ws.go getUrlByExchange) stay on mainnet by
// design: testnet order books are too thin to drive strategy decisions.
const (
	SpotMainnetBaseURL = "https://api.binance.com"
	SpotTestnetBaseURL = "https://testnet.binance.vision"
)

// SpotBaseURL returns the spot REST base URL for the requested network.
func SpotBaseURL(testNet bool) string {
	if testNet {
		return SpotTestnetBaseURL
	}
	return SpotMainnetBaseURL
}

// InitExchange constructs an authenticated spot client bound to the
// exchange's network (mainnet default, testnet when exchange.TestNet).
func InitExchange(exchange Binance) (*binance.Client, *common.APIError) {
	client := binance.NewClient(exchange.ApiKey, exchange.ApiSecret)
	client.BaseURL = SpotBaseURL(exchange.TestNet)
	return client, nil
}
