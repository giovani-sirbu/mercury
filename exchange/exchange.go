package exchange

import (
	"github.com/giovani-sirbu/mercury/exchange/adaptors"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
)

// Exchange structure to init exchange
type Exchange struct {
	Name          string             `json:"name"`
	ApiKey        string             `json:"apiKey"`
	ApiSecret     string             `json:"apiSecret"`
	TestNet       bool               `json:"testNet"`
	CustomActions aggregates.Actions `json:"customActions"`
	IsCustom      bool               `json:"isCustom"`
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
