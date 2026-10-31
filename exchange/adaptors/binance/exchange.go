package binanceAdaptor

import (
	"github.com/adshao/go-binance/v2"
	"github.com/adshao/go-binance/v2/common"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
)

// Binance is the exchange config every adaptor method hangs off.
type Binance struct {
	aggregates.Exchange
}

// ApiError normalises an error into *common.APIError; nil stays nil.
func ApiError(err error) *common.APIError {
	if err == nil {
		return nil
	}

	if apiErr, ok := err.(*common.APIError); ok {
		// Response keeps the raw body of a non-JSON answer.
		return &common.APIError{
			Code:     apiErr.Code,
			Message:  apiErr.Message,
			Response: apiErr.Response,
		}
	}

	return &common.APIError{
		Code:    0,
		Message: err.Error(),
	}
}

// GetBinanceActions builds the Actions function bag for a spot exchange.
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
		GetPrices:        b.GetPrices,
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

// InitExchange constructs an authenticated client; TestNet toggles a library-level global.
func InitExchange(exchange Binance) (*binance.Client, *common.APIError) {
	if exchange.TestNet {
		binance.UseTestnet = true
	} else {
		binance.UseTestnet = false
	}
	return binance.NewClient(exchange.ApiKey, exchange.ApiSecret), nil
}
