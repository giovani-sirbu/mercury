// Package quantities derives order and position sizes from a trade's history:
// the gross and net position, the quantity a close would submit and the
// exchange minimum order.
package quantities

import (
	"fmt"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/helpers"
	"github.com/giovani-sirbu/mercury/log"
	"strings"
)

func GetQuantities(event events.Events) (float64, string) {
	var quantity float64
	var historyType string
	var buyTotal float64
	var sellTotal float64

	for _, data := range event.Trade.History {
		if strings.ToLower(data.Type) == "buy" {
			buyAmount := data.Quantity
			// if inverse we must multiply with price
			if event.Trade.Inverse {
				buyAmount *= data.Price
			}
			buyTotal += buyAmount
		} else {
			sellAmount := data.Quantity
			// if inverse we must multiply with price
			if event.Trade.Inverse {
				sellAmount *= data.Price
			}
			sellTotal += sellAmount
		}
	}

	historyType = "sell"
	quantity = buyTotal - sellTotal

	if event.Trade.Inverse {
		historyType = "buy"
		quantity = sellTotal - buyTotal
		quantity /= event.Trade.PositionPrice
	}

	quantity = helpers.ToFixed(quantity, int(event.Trade.StrategyPair.TradeFilters.LotSize))

	log.Debug(fmt.Sprintf("GetQuantities: quantity(%f), historyType(%s), sellTotal(%f), buyTotal(%f), inverse(%t)", quantity, historyType, sellTotal, buyTotal, event.Trade.Inverse))

	return quantity, historyType
}
