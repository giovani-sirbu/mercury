package tests

import (
	"github.com/giovani-sirbu/mercury/exchange"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
	"github.com/giovani-sirbu/mercury/virtualExchange"
)

func GetVirtualExchange(asset string, amount string) exchange.Exchange {
	exchangeInit := virtualExchange.InitVirtualExchange([]aggregates.UserAssetRecord{{Asset: asset, Free: amount}})
	return exchangeInit
}
