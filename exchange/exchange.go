package exchange

import (
	"github.com/giovani-sirbu/mercury/exchange/adaptors"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
)

// Exchange structure to init exchange
type Exchange struct {
	Name      string `bson:"name" json:"name"`
	ApiKey    string `bson:"apiKey" json:"apiKey"`
	ApiSecret string `bson:"apiSecret" json:"apiSecret"`
	TestNet   bool   `bson:"testNet" json:"testNet"`
}

// Client Add client method to receive exchange actions
func (e Exchange) Client() (aggregates.Actions, error) {
	var initExchange = aggregates.Exchange{Name: e.Name, ApiKey: e.ApiKey, ApiSecret: e.ApiSecret, TestNet: e.TestNet}
	exchangeActions, err := adaptors.GetExchangeActions(initExchange)
	return exchangeActions, err
}
