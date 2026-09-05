// Package futures holds the USD-M futures helpers the futures actions share:
// symbol precision and the realized income of the latest closed position.
package futures

import (
	"fmt"
	"github.com/giovani-sirbu/mercury/events"
	"math"
	"strconv"
	"strings"
)

// GetPrecision returns the symbol's quantity and price precision from the
// futures exchange info.
//
// Every failure returns a real error. The three branches below used to return
// the named result `err`, which is nil at that point, so a client failure, an
// exchange-info failure and a symbol missing from the response all reported
// success with precision 0/0 — and `if precisionErr != nil` in all three
// callers was dead code. The callers then formatted the order's quantity and
// stop price with zero decimals, which the exchange rejects or, worse, fills
// at a rounded size.
func GetPrecision(event events.Events) (qtyPrecision, pricePrecision int, err error) {
	// Init futures client
	client, clientErr := event.Exchange.FuturesClient()
	if clientErr != nil {
		return 0, 0, clientErr
	}

	// Get futures exchange info
	exchangeInfo, infoErr := client.GetFuturesExchangeInfo()
	if infoErr != nil {
		return 0, 0, infoErr
	}

	// Go through symbols to fetch the needed precision values
	found := false
	formattedSymbol := strings.Replace(event.Trade.Symbol, "/", "", 1)
	for _, s := range exchangeInfo.Symbols {
		if s.Symbol == formattedSymbol {
			found = true
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
	if !found {
		return 0, 0, fmt.Errorf("futures precision not found for %s", formattedSymbol)
	}

	return qtyPrecision, pricePrecision, nil
}
