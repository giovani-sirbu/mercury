package tests

import (
	"github.com/giovani-sirbu/mercury/exchange"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
	"github.com/giovani-sirbu/mercury/virtualExchange"
)

func GetVirtualExchange() exchange.Exchange {
	exchangeInit := virtualExchange.InitVirtualExchange([]aggregates.UserAssetRecord{{Asset: "USDT", Free: "1000.0"}})
	return exchangeInit
}
