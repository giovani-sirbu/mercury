package binanceAdaptor

import (
	"context"
	"strconv"
	"strings"

	"github.com/adshao/go-binance/v2/common"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
	"github.com/jinzhu/copier"
)

// GetExchangeInfo returns the /exchangeInfo payload for a single symbol.
func (e Binance) GetExchangeInfo(symbol string) (aggregates.ExchangeInfo, *common.APIError) {
	client, initErr := InitExchange(e)
	if initErr != nil {
		return aggregates.ExchangeInfo{}, initErr
	}
	formattedSymbol := strings.Replace(symbol, "/", "", 1)
	details, detailsErr := client.NewExchangeInfoService().Symbol(formattedSymbol).Do(context.Background())
	if detailsErr != nil {
		return aggregates.ExchangeInfo{}, ApiError(detailsErr)
	}
	var exchangeInfoResult aggregates.ExchangeInfo
	copier.Copy(&exchangeInfoResult, &details)
	return exchangeInfoResult, nil
}

// GetFees returns the maker/taker commission pair for a single symbol.
func (e Binance) GetFees(symbol string) (aggregates.TradeFeeDetails, *common.APIError) {
	client, initErr := InitExchange(e)
	if initErr != nil {
		return aggregates.TradeFeeDetails{}, initErr
	}
	formattedSymbol := strings.Replace(symbol, "/", "", 1)
	details, detailsErr := client.NewTradeFeeService().Symbol(formattedSymbol).Do(context.Background())

	if detailsErr != nil {
		return aggregates.TradeFeeDetails{}, ApiError(detailsErr)
	}

	var exchangeInfoResult aggregates.TradeFeeDetails
	copier.Copy(&exchangeInfoResult, &details[0])
	return exchangeInfoResult, nil
}

// GetPrice returns the current average price for the symbol. Preferred over
// the aggTrade WebSocket path when an on-demand number is needed (e.g. a
// fallback inside mercury's getSymbolPrice when WsPrices misses).
func (e Binance) GetPrice(symbol string) (float64, *common.APIError) {
	var price float64
	formattedSymbol := strings.Replace(symbol, "/", "", 1)

	client, initErr := InitExchange(e)
	if initErr != nil {
		return price, initErr
	}
	clientInfo, err := client.NewAveragePriceService().Symbol(formattedSymbol).Do(context.Background())
	if err != nil {
		return price, ApiError(err)
	}
	price, _ = strconv.ParseFloat(clientInfo.Price, 64)
	return price, nil
}

// GetPrices returns the last price of every symbol the exchange lists, keyed by
// the raw binance symbol ("ETHBTC"). One request covers the whole book, which is
// what makes it usable for valuing a wallet whose assets are not known up front
// — the per-symbol GetPrice would need one round trip per asset held.
func (e Binance) GetPrices() (map[string]float64, *common.APIError) {
	client, initErr := InitExchange(e)
	if initErr != nil {
		return nil, initErr
	}
	tickers, err := client.NewListPricesService().Do(context.Background())
	if err != nil {
		return nil, ApiError(err)
	}

	prices := make(map[string]float64, len(tickers))
	for _, ticker := range tickers {
		price, parseErr := strconv.ParseFloat(ticker.Price, 64)
		if parseErr != nil {
			continue
		}
		prices[ticker.Symbol] = price
	}
	return prices, nil
}

// Klines returns candlestick bars for a symbol. Klines are uniquely
// identified by their open time.
func (e Binance) Klines(payload aggregates.KlinePayload) ([]aggregates.KlineResponse, *common.APIError) {
	client, initErr := InitExchange(e)
	if initErr != nil {
		return nil, initErr
	}
	clientQuery := client.NewKlinesService().Symbol(payload.Symbol).Interval(payload.Interval)

	if payload.StartTime > 0 {
		clientQuery.StartTime(payload.StartTime)
	}
	if payload.EndTime > 0 {
		clientQuery.EndTime(payload.EndTime)
	}
	if payload.Limit > 0 {
		clientQuery.Limit(payload.Limit)
	}

	clientData, err := clientQuery.Do(context.Background())
	if err != nil {
		return nil, ApiError(err)
	}

	var data []aggregates.KlineResponse
	copier.Copy(&data, &clientData)
	return data, nil
}

// AggTrades returns compressed, aggregate trades. Trades that fill at the
// same time, from the same order, with the same price are aggregated.
func (e Binance) AggTrades(payload aggregates.AggTradesPayload) ([]aggregates.AggTradesResponse, *common.APIError) {
	client, initErr := InitExchange(e)
	if initErr != nil {
		return nil, initErr
	}
	clientQuery := client.NewAggTradesService().Symbol(payload.Symbol)

	if payload.FromId > 0 {
		clientQuery.FromID(payload.FromId)
	}
	if payload.StartTime > 0 {
		clientQuery.StartTime(payload.StartTime)
	}
	if payload.EndTime > 0 {
		clientQuery.EndTime(payload.EndTime)
	}
	if payload.Limit > 0 {
		clientQuery.Limit(payload.Limit)
	}

	clientData, err := clientQuery.Do(context.Background())
	if err != nil {
		return nil, ApiError(err)
	}

	var data []aggregates.AggTradesResponse
	copier.Copy(&data, &clientData)
	return data, nil
}
