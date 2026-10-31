package binanceAdaptor

import (
	"github.com/adshao/go-binance/v2/common"
	"github.com/adshao/go-binance/v2/futures"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
)

// GetFuturesBinanceActions builds the FuturesActions function bag.
func GetFuturesBinanceActions(e aggregates.Exchange) aggregates.FuturesActions {
	b := Binance{e}
	return aggregates.FuturesActions{
		CreateFuturesOrder:      b.CreateFutureOrder,
		ModifyFuturesOrderPrice: b.ModifyFuturesOrderPrice,
		ListOrders:              b.ListOrders,
		GetOrderById:            b.GetOrderById,
		CancelOrders:            b.CancelOrders,
		GetSymbolPosition:       b.GetSymbolPosition,
		SetSymbolLeverage:       b.SetSymbolLeverage,
		GetFuturesExchangeInfo:  b.GetFuturesExchangeInfo,
		GetIncomeHistory:        b.GetIncomeHistory,
		GetFuturesBalance:       b.GetFuturesBalance,
		KlineData:               b.FuturesKlines,
	}
}

// InitFuturesExchange constructs an authenticated futures client; TestNet toggles a library-level global.
func InitFuturesExchange(exchange Binance) (*futures.Client, *common.APIError) {
	if exchange.TestNet {
		futures.UseTestnet = true
	} else {
		futures.UseTestnet = false
	}
	return futures.NewClient(exchange.ApiKey, exchange.ApiSecret), nil
}
