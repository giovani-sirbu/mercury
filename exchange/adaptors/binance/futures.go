package binanceAdaptor

import (
	"github.com/adshao/go-binance/v2/common"
	"github.com/adshao/go-binance/v2/futures"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
)

// GetFuturesBinanceActions builds the FuturesActions function-bag surface
// the rest of mercury consumes. The individual method groups live in sibling
// files:
//
//   - futuresOrders.go     — Create / List / Get / Cancel / Modify orders
//   - futuresPositions.go  — GetSymbolPosition / SetSymbolLeverage
//   - futuresMetadata.go   — GetFuturesExchangeInfo / GetIncomeHistory / GetFuturesBalance
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

// USD-M futures REST base URLs, selected per client instance exactly like
// the spot ones (see exchange.go): the futures.UseTestnet global is never
// touched, so mixed mainnet/testnet accounts are safe in one process.
const (
	FuturesMainnetBaseURL = "https://fapi.binance.com"
	FuturesTestnetBaseURL = "https://testnet.binancefuture.com"
)

// FuturesBaseURL returns the futures REST base URL for the requested network.
func FuturesBaseURL(testNet bool) string {
	if testNet {
		return FuturesTestnetBaseURL
	}
	return FuturesMainnetBaseURL
}

// InitFuturesExchange constructs an authenticated futures client bound to
// the exchange's network (mainnet default, testnet when exchange.TestNet).
func InitFuturesExchange(exchange Binance) (*futures.Client, *common.APIError) {
	client := futures.NewClient(exchange.ApiKey, exchange.ApiSecret)
	client.BaseURL = FuturesBaseURL(exchange.TestNet)
	return client, nil
}
