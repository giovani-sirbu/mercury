package ccxt

import (
	"fmt"
	"strconv"

	"github.com/adshao/go-binance/v2/common"
	ccxt "github.com/ccxt/ccxt/go/v4"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
)

// GetPrice returns the last traded price of a symbol. CCXT's FetchTicker
// surfaces Last as `*float64`, which we deref + format back into the float
// shape mercury callers expect (the mercury action chain uses
// `getSymbolPrice` to round-trip Binance's string into a float; here we
// short-circuit by returning the float directly).
func getPrice(e aggregates.Exchange, symbol string) (float64, *common.APIError) {
	client, err := newClient(e)
	if err != nil {
		return 0, apiError(err)
	}
	ticker, ccxtErr := client.FetchTicker(symbol)
	if ccxtErr != nil {
		return 0, apiError(ccxtErr)
	}
	if ticker.Last == nil {
		return 0, apiError(fmt.Errorf("ccxt: FetchTicker(%s) returned nil Last price", symbol))
	}
	return *ticker.Last, nil
}

// GetExchangeInfo returns the exchange's market metadata. Mercury consumers
// use this to validate symbol existence and access tick/lot filters. CCXT
// represents markets via FetchMarkets, which we collapse into the simpler
// ExchangeInfo struct mercury already passes around.
func getExchangeInfo(e aggregates.Exchange, symbol string) (aggregates.ExchangeInfo, *common.APIError) {
	client, err := newClient(e)
	if err != nil {
		return aggregates.ExchangeInfo{}, apiError(err)
	}
	markets, ccxtErr := client.FetchMarkets()
	if ccxtErr != nil {
		return aggregates.ExchangeInfo{}, apiError(ccxtErr)
	}
	info := aggregates.ExchangeInfo{}
	for _, m := range markets {
		sym := m.Symbol
		if sym == nil {
			continue
		}
		if symbol != "" && *sym != symbol {
			continue
		}
		s := aggregates.Symbol{Symbol: derefString(sym)}
		if m.BaseCurrency != nil {
			s.BaseAsset = *m.BaseCurrency
		}
		if m.QuoteCurrency != nil {
			s.QuoteAsset = *m.QuoteCurrency
		}
		// Synthesise Binance-style filter dictionaries from CCXT's Limits +
		// raw Info. Mercury's addPair handler keys on filterType NOTIONAL /
		// LOT_SIZE / PRICE_FILTER and extracts minNotional / stepSize /
		// tickSize. CCXT exposes these as Limits (typed MinMax) plus per-
		// exchange data in Info["precision"]. We translate so legacy callers
		// keep working without case-by-case adaptor knowledge.
		s.Filters = buildBinanceStyleFilters(m)
		info.Symbols = append(info.Symbols, s)
	}
	return info, nil
}

// buildBinanceStyleFilters emits the per-symbol filter dictionaries mercury's
// addPair handler expects. Best-effort: when a particular limit is missing
// (e.g. an exchange that does not expose tick size in Limits), the
// corresponding filter is omitted and the handler's parse loop just skips it.
func buildBinanceStyleFilters(m ccxt.MarketInterface) []map[string]interface{} {
	var filters []map[string]interface{}
	if m.Limits.Cost.Min != nil {
		filters = append(filters, map[string]interface{}{
			"filterType":  "NOTIONAL",
			"minNotional": formatFloat(*m.Limits.Cost.Min),
		})
	}
	if precMap, ok := m.Info["precision"].(map[string]any); ok {
		if amountStep, ok := precMap["amount"]; ok {
			filters = append(filters, map[string]interface{}{
				"filterType": "LOT_SIZE",
				"stepSize":   fmt.Sprintf("%v", amountStep),
			})
		}
		if priceStep, ok := precMap["price"]; ok {
			filters = append(filters, map[string]interface{}{
				"filterType": "PRICE_FILTER",
				"tickSize":   fmt.Sprintf("%v", priceStep),
			})
		}
	} else if m.Limits.Amount.Min != nil {
		filters = append(filters, map[string]interface{}{
			"filterType": "LOT_SIZE",
			"stepSize":   formatFloat(*m.Limits.Amount.Min),
		})
	}
	return filters
}

// GetFees fetches the per-symbol maker/taker commission. CCXT exposes this as
// FetchTradingFee for a single symbol; we convert the returned floats into
// mercury's string representation.
func getFees(e aggregates.Exchange, symbol string) (aggregates.TradeFeeDetails, *common.APIError) {
	client, err := newClient(e)
	if err != nil {
		return aggregates.TradeFeeDetails{}, apiError(err)
	}
	fee, ccxtErr := client.FetchTradingFee(symbol)
	if ccxtErr != nil {
		return aggregates.TradeFeeDetails{}, apiError(ccxtErr)
	}
	return aggregates.TradeFeeDetails{
		Symbol:          derefString(fee.Symbol),
		MakerCommission: formatFloat(derefFloat(fee.Maker)),
		TakerCommission: formatFloat(derefFloat(fee.Taker)),
	}, nil
}

// KlineData returns OHLCV candles. The Binance-style payload (Symbol +
// Interval + optional time bounds) maps cleanly to CCXT's FetchOHLCV.
func klineData(e aggregates.Exchange, payload aggregates.KlinePayload) ([]aggregates.KlineResponse, *common.APIError) {
	client, err := newClient(e)
	if err != nil {
		return nil, apiError(err)
	}
	opts := []ccxt.FetchOHLCVOptions{
		ccxt.WithFetchOHLCVTimeframe(payload.Interval),
	}
	if payload.StartTime > 0 {
		opts = append(opts, ccxt.WithFetchOHLCVSince(payload.StartTime))
	}
	if payload.Limit > 0 {
		opts = append(opts, ccxt.WithFetchOHLCVLimit(int64(payload.Limit)))
	}
	candles, ccxtErr := client.FetchOHLCV(payload.Symbol, opts...)
	if ccxtErr != nil {
		return nil, apiError(ccxtErr)
	}
	out := make([]aggregates.KlineResponse, 0, len(candles))
	for _, c := range candles {
		out = append(out, aggregates.KlineResponse{
			OpenTime:  c.Timestamp,
			Open:      formatFloat(c.Open),
			High:      formatFloat(c.High),
			Low:       formatFloat(c.Low),
			Close:     formatFloat(c.Close),
			Volume:    formatFloat(c.Volume),
			CloseTime: c.Timestamp, // CCXT OHLCV does not split open/close timestamps; use the same value
		})
	}
	return out, nil
}

// AggTrades returns aggregated public trades. CCXT's FetchTrades is the
// closest unified call; aggregation behaviour matches Binance's /aggTrades
// approximately (per-exchange semantics may vary). Mercury consumers use this
// for short-term momentum analysis (sophos / hermes-side indicators) where
// exact aggTrade semantics aren't critical.
func aggTrades(e aggregates.Exchange, payload aggregates.AggTradesPayload) ([]aggregates.AggTradesResponse, *common.APIError) {
	client, err := newClient(e)
	if err != nil {
		return nil, apiError(err)
	}
	opts := []ccxt.FetchTradesOptions{}
	if payload.StartTime > 0 {
		opts = append(opts, ccxt.WithFetchTradesSince(payload.StartTime))
	}
	if payload.Limit > 0 {
		opts = append(opts, ccxt.WithFetchTradesLimit(int64(payload.Limit)))
	}
	trades, ccxtErr := client.FetchTrades(payload.Symbol, opts...)
	if ccxtErr != nil {
		return nil, apiError(ccxtErr)
	}
	out := make([]aggregates.AggTradesResponse, 0, len(trades))
	for _, t := range trades {
		parsedID, _ := strconv.ParseInt(derefString(t.Id), 10, 64)
		out = append(out, aggregates.AggTradesResponse{
			AggTradeID:   parsedID,
			Price:        formatFloat(derefFloat(t.Price)),
			Quantity:     formatFloat(derefFloat(t.Amount)),
			Timestamp:    derefInt64(t.Timestamp),
			IsBuyerMaker: derefString(t.TakerOrMaker) == "maker" && derefString(t.Side) == "sell",
		})
	}
	return out, nil
}
