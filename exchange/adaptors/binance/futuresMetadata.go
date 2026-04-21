package binanceAdaptor

import (
	"context"
	"strings"

	"github.com/adshao/go-binance/v2/common"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
	"github.com/jinzhu/copier"
)

// GetFuturesExchangeInfo returns the /exchangeInfo payload for the futures
// market (all symbols). Callers filter client-side.
func (e Binance) GetFuturesExchangeInfo() (aggregates.ExchangeInfo, *common.APIError) {
	var exchangeInfo aggregates.ExchangeInfo
	client, initErr := InitFuturesExchange(e)
	if initErr != nil {
		return exchangeInfo, initErr
	}
	exchangeInfoResponse, err := client.NewExchangeInfoService().Do(context.Background())
	copier.Copy(&exchangeInfo, &exchangeInfoResponse)
	return exchangeInfo, ApiError(err)
}

// GetIncomeHistory returns REALIZED_PNL and COMMISSION entries combined for
// a single symbol. Binance splits income by type in the API; callers that
// compute P&L need both, so we fetch them in sequence and concatenate.
//
// Note: if fees error but income succeeds we still return the income list
// with the fees error surfaced — matches the original behaviour where the
// caller only sees a single error.
func (e Binance) GetIncomeHistory(symbol string) ([]aggregates.IncomeHistory, *common.APIError) {
	var incomeHistory []aggregates.IncomeHistory
	client, initErr := InitFuturesExchange(e)
	if initErr != nil {
		return incomeHistory, initErr
	}
	formattedSymbol := strings.Replace(symbol, "/", "", 1)
	income, incomeErr := client.NewGetIncomeHistoryService().
		Symbol(formattedSymbol).
		IncomeType("REALIZED_PNL").
		Do(context.Background())

	fees, feesErr := client.NewGetIncomeHistoryService().
		Symbol(formattedSymbol).
		IncomeType("COMMISSION").
		Do(context.Background())

	if feesErr != nil {
		incomeErr = feesErr
	}

	income = append(income, fees...)
	copier.Copy(&incomeHistory, &income)
	return incomeHistory, ApiError(incomeErr)
}

// GetFuturesBalance returns the per-asset futures wallet balance.
func (e Binance) GetFuturesBalance() ([]aggregates.FuturesBalance, *common.APIError) {
	var balance []aggregates.FuturesBalance
	client, initErr := InitFuturesExchange(e)
	if initErr != nil {
		return balance, initErr
	}
	income, incomeErr := client.NewGetBalanceService().Do(context.Background())

	copier.Copy(&balance, &income)
	return balance, ApiError(incomeErr)
}
