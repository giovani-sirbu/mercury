package actions

import (
	"github.com/giovani-sirbu/mercury/events"
	"math"
	"strconv"
	"strings"
)

func GetPrecision(event events.Events) (qtyPrecision, pricePrecision int, err error) {
	// Init futures client
	client, clientErr := event.Exchange.FuturesClient()
	if clientErr != nil {
		return 0, 0, err
	}

	// Get futures exchange info
	exchangeInfo, infoErr := client.GetFuturesExchangeInfo()
	if infoErr != nil {
		return 0, 0, err
	}

	// Go through symbols to fetch the needed precision values
	formattedSymbol := strings.Replace(event.Trade.Symbol, "/", "", 1)
	for _, s := range exchangeInfo.Symbols {
		if s.Symbol == formattedSymbol {
			for _, filter := range s.Filters {
				switch filter["filterType"] {
				case "LOT_SIZE":
					stepSize, _ := strconv.ParseFloat(filter["stepSize"].(string), 64)
					qtyPrecision = int(math.Abs(math.Log10(stepSize)))
				case "PRICE_FILTER":
					tickSize, _ := strconv.ParseFloat(filter["tickSize"].(string), 64)
					pricePrecision = int(math.Abs(math.Log10(tickSize)))
				}
			}
			break
		}
	}
	return qtyPrecision, pricePrecision, nil
}
