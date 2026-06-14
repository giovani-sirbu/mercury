package binanceAdaptor

import (
	"context"

	"github.com/adshao/go-binance/v2/common"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
	"github.com/jinzhu/copier"
)

// FuturesKlines returns futures candlestick bars for a symbol.
func (e Binance) FuturesKlines(payload aggregates.KlinePayload) ([]aggregates.KlineResponse, *common.APIError) {
	client, initErr := InitFuturesExchange(e)
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
