package exchange

import (
	"github.com/giovani-sirbu/mercury/exchange/adaptors"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// Exchange structure to init exchange
type Exchange struct {
	Name          string                `json:"name"`
	ApiKey        string                `json:"apiKey"`
	ApiSecret     string                `json:"apiSecret"`
	TestNet       bool                  `json:"testNet"`
	CustomActions aggregates.Actions    `json:"customActions"`
	TradeType     aggragates.TradeTypes `json:"tradeType"`
	IsCustom      bool                  `json:"isCustom"`
}

// Client Add client method to receive exchange actions
func (e Exchange) Client() (aggregates.Actions, error) {
	if e.IsCustom {
		return e.CustomActions, nil
	}
	var initExchange = aggregates.Exchange{Name: e.Name, ApiKey: e.ApiKey, ApiSecret: e.ApiSecret, TestNet: e.TestNet}
	exchangeActions, err := adaptors.GetExchangeActions(initExchange)
	return exchangeActions, err
}

// Client Add client method to receive exchange actions
func (e Exchange) FuturesClient() (aggregates.FuturesActions, error) {
	var initExchange = aggregates.Exchange{Name: e.Name, ApiKey: e.ApiKey, ApiSecret: e.ApiSecret, TestNet: e.TestNet}
	exchangeActions, err := adaptors.GetFuturesExchangeActions(initExchange)
	return exchangeActions, err
}
