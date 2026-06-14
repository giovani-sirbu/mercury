package binanceAdaptor

import (
	"context"
	"strings"

	"github.com/adshao/go-binance/v2/common"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
	"github.com/jinzhu/copier"
)

// GetSymbolPosition returns the current open position for a symbol. In hedge
// mode this can return two entries (one per side); in one-way mode at most
// one entry has a non-zero PositionAmt.
func (e Binance) GetSymbolPosition(symbol string) ([]aggregates.PositionRisk, *common.APIError) {
	var positions []aggregates.PositionRisk
	client, initErr := InitFuturesExchange(e)
	if initErr != nil {
		return nil, initErr
	}
	formattedSymbol := strings.Replace(symbol, "/", "", 1)
	positionsResponse, err := client.NewGetPositionRiskService().Symbol(formattedSymbol).Do(context.Background())

	copier.Copy(&positions, &positionsResponse)
	return positions, ApiError(err)
}

// SetSymbolLeverage changes the leverage multiplier for a symbol. Binance
// allows only integer leverage values.
func (e Binance) SetSymbolLeverage(symbol string, leverage int) (aggregates.SymbolLeverage, *common.APIError) {
	var symbolLeverage aggregates.SymbolLeverage
	client, initErr := InitFuturesExchange(e)
	if initErr != nil {
		return symbolLeverage, initErr
	}

	formattedSymbol := strings.Replace(symbol, "/", "", 1)
	response, err := client.NewChangeLeverageService().
		Symbol(formattedSymbol).
		Leverage(leverage).
		Do(context.Background())

	copier.Copy(&symbolLeverage, &response)
	return symbolLeverage, ApiError(err)
}
