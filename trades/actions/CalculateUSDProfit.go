package actions

import (
	"github.com/giovani-sirbu/mercury/crypto"
	"github.com/giovani-sirbu/mercury/exchange"
	"github.com/giovani-sirbu/mercury/log"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"os"
	"strings"
)

const fiatConversionSymbol = "USDT"

func CalculateUSDProfit(trade aggragates.Trades) float64 {
	secretKey, decryptErr := crypto.Decrypt(trade.Exchange.ApiSecret, os.Getenv("API_SECRET"))
	if decryptErr != nil {
		log.Error(decryptErr.Error(), "decryptErr", "CalculateUSDProfit")
		return 0
	}

	exchangeInit := exchange.Exchange{
		Name:      trade.Exchange.Name,
		ApiSecret: secretKey,
		ApiKey:    trade.Exchange.ApiKey,
		TestNet:   trade.Exchange.TestNet,
	}
	client, clientErr := exchangeInit.Client()
	if clientErr != nil {
		log.Error(clientErr.Error(), "clientErr", "CalculateUSDProfit")
		return 0
	}

	// return result if profit asset is in usd
	if strings.Contains(trade.ProfitAsset, "USD") {
		return trade.Profit
	}

	// get profit asset price
	price, priceErr := client.GetPrice(trade.ProfitAsset + "/" + fiatConversionSymbol)
	if priceErr != nil {
		log.Error(priceErr.Error(), "priceErr", "CalculateUSDProfit")
		return 0
	}

	return trade.Profit * price
}
