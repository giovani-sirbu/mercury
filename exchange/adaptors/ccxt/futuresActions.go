package ccxt

import (
	binanceAdaptor "github.com/giovani-sirbu/mercury/exchange/adaptors/binance"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
)

// GetCCXTFuturesActions returns the futures function-bag. In this PR futures
// stays on the legacy binance adaptor — porting the 12 futures methods to
// CCXT's binanceusdm wrapper is deferred. Spot is the multi-exchange goal
// (Binance + Crypto.com), futures remains Binance-only until we add a futures
// exchange to the lineup.
//
// The factory in exchange.go calls this when the REST backend is "ccxt", so
// even after the cutover, agora/hermes futures actions still go through
// adshao/go-binance with no behaviour change. When CCXT futures migration
// happens, this function is the single touchpoint to swap.
//
// Crypto.com is not handled here because the platform has no Crypto.com
// futures support — adding it is out of scope for this PR.
func GetCCXTFuturesActions(e aggregates.Exchange) aggregates.FuturesActions {
	return binanceAdaptor.GetFuturesBinanceActions(e)
}
