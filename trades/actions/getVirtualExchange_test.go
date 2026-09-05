package actions_test

import (
	"github.com/giovani-sirbu/mercury/exchange"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
	"github.com/giovani-sirbu/mercury/trades/internal/virtualexchange"
)

func GetVirtualExchange(asset string, amount string) exchange.Exchange {
	exchangeInit := virtualexchange.InitVirtualExchange([]aggregates.UserAssetRecord{{Asset: asset, Free: amount}})
	return exchangeInit
}
