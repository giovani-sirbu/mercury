package ccxt

import (
	"strconv"

	"github.com/adshao/go-binance/v2/common"
	ccxt "github.com/ccxt/ccxt/go/v4"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
)

// Buy submits a LIMIT buy. CCXT normalises order types ("limit") and sides
// ("buy"/"sell") across exchanges so the same call works for both Binance and
// Crypto.com. Symbol is passed in mercury's stored format (e.g. "BTC/USDC"),
// which is also CCXT's canonical form — no slash-stripping needed at this
// boundary the way the binance adaptor does for go-binance.
func buy(e aggregates.Exchange, symbol string, quantity float64, price string) (aggregates.CreateOrderResponse, *common.APIError) {
	return createOrder(e, symbol, "limit", "buy", quantity, price)
}

// Sell submits a LIMIT sell. Mirror of Buy with side=sell.
func sell(e aggregates.Exchange, symbol string, quantity float64, price string) (aggregates.CreateOrderResponse, *common.APIError) {
	return createOrder(e, symbol, "limit", "sell", quantity, price)
}

// MarketBuy executes at the current best ask.
func marketBuy(e aggregates.Exchange, symbol string, quantity float64) (aggregates.CreateOrderResponse, *common.APIError) {
	return createOrder(e, symbol, "market", "buy", quantity, "")
}

// MarketSell executes at the current best bid.
func marketSell(e aggregates.Exchange, symbol string, quantity float64) (aggregates.CreateOrderResponse, *common.APIError) {
	return createOrder(e, symbol, "market", "sell", quantity, "")
}

// createOrder is the shared CCXT.CreateOrder wrapper. Price is optional on
// market orders; we pass it via `WithCreateOrderPrice` only when present.
// The CCXT side strings are lower-case ("buy"/"sell"); mercury's downstream
// code compares uppercase, which the converter uppercases when constructing
// the CreateOrderResponse.
func createOrder(e aggregates.Exchange, symbol string, orderType string, side string, quantity float64, price string) (aggregates.CreateOrderResponse, *common.APIError) {
	client, err := newClient(e)
	if err != nil {
		return aggregates.CreateOrderResponse{}, apiError(err)
	}

	var opts []ccxt.CreateOrderOptions
	if price != "" {
		// Parse the stringified price back into a float. The mercury action
		// chain produces these as `strconv.FormatFloat(..., 'f', -1, 64)`
		// so the round-trip is lossless.
		if p, perr := strconv.ParseFloat(price, 64); perr == nil {
			opts = append(opts, ccxt.WithCreateOrderPrice(p))
		}
	}

	order, ccxtErr := client.CreateOrder(symbol, orderType, side, quantity, opts...)
	if ccxtErr != nil {
		return aggregates.CreateOrderResponse{}, apiError(ccxtErr)
	}
	resp := fromCCXTOrder(order)
	// CCXT's typed Order does not surface TimeInForce; preserve the GTC
	// default that fromCCXTOrder set, but overwrite Type/Side with the call
	// arguments because some exchanges return them in lowercase or omitted.
	if resp.Type == "" {
		resp.Type = ToUpper(orderType)
	}
	if resp.Side == "" {
		resp.Side = ToUpper(side)
	}
	return resp, nil
}

// GetOrder looks up an order by Binance-style int64 id. CCXT identifies
// orders by string id, so we format the int64 back to decimal at the boundary.
func getOrder(e aggregates.Exchange, orderId int64, symbol string) (aggregates.Order, *common.APIError) {
	client, err := newClient(e)
	if err != nil {
		return aggregates.Order{}, apiError(err)
	}
	order, ccxtErr := client.FetchOrder(strconv.FormatInt(orderId, 10), ccxt.WithFetchOrderSymbol(symbol))
	if ccxtErr != nil {
		return aggregates.Order{}, apiError(ccxtErr)
	}
	return fromCCXTOrderFull(order), nil
}

// CancelOrder cancels an open order by id. CCXT's CancelOrder returns the
// final Order shape; we re-map to CancelOrderResponse.
func cancelOrder(e aggregates.Exchange, orderId int64, symbol string) (aggregates.CancelOrderResponse, *common.APIError) {
	client, err := newClient(e)
	if err != nil {
		return aggregates.CancelOrderResponse{}, apiError(err)
	}
	order, ccxtErr := client.CancelOrder(strconv.FormatInt(orderId, 10), ccxt.WithCancelOrderSymbol(symbol))
	if ccxtErr != nil {
		return aggregates.CancelOrderResponse{}, apiError(ccxtErr)
	}
	return fromCCXTOrderCancel(order), nil
}

// GetTrades returns every fill associated with the given order id. CCXT's
// FetchMyTrades returns all recent trades for a symbol, so we filter client-
// side. For Binance specifically this scopes correctly — only fills tied to
// the orderId pass through.
func getTrades(e aggregates.Exchange, orderId int64, symbol string) ([]aggregates.Trade, *common.APIError) {
	client, err := newClient(e)
	if err != nil {
		return nil, apiError(err)
	}
	trades, ccxtErr := client.FetchMyTrades(ccxt.WithFetchMyTradesSymbol(symbol))
	if ccxtErr != nil {
		return nil, apiError(ccxtErr)
	}
	orderIdStr := strconv.FormatInt(orderId, 10)
	out := make([]aggregates.Trade, 0, len(trades))
	for _, t := range trades {
		if t.Order != nil && *t.Order == orderIdStr {
			out = append(out, fromCCXTTrade(t))
		}
	}
	return out, nil
}

// ToUpper is a tiny exported helper. We avoid importing strings just for one
// call in createOrder by keeping the conversion adjacent.
func ToUpper(s string) string {
	out := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			c -= 32
		}
		out[i] = c
	}
	return string(out)
}
