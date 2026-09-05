// Package profit computes a trade's gross, realized, USD and minimum profit
// from its history and fees, and the close price the profit gates value a
// simulated exit at.
package profit

import (
	"fmt"
	"github.com/giovani-sirbu/mercury/log"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"strings"
)

func GetProfit(trade aggragates.Trades) float64 {
	var buyTotal, sellTotal, dust, profit float64

	for _, data := range trade.History {
		if strings.ToLower(data.Type) == "buy" {
			buyAmount := data.Quantity
			// if NOT inverse we must multiply with price
			if !trade.Inverse {
				buyAmount *= data.Price
			}
			buyTotal += buyAmount
		} else {
			sellAmount := data.Quantity
			// if NOT inverse we must multiply with price
			if !trade.Inverse {
				sellAmount *= data.Price
			}
			sellTotal += sellAmount
		}
	}

	dust = trade.Dust * trade.PositionPrice
	profit = sellTotal - buyTotal + dust

	if trade.Inverse {
		dust = trade.Dust
		profit = buyTotal - sellTotal + dust
	}

	log.Debug(fmt.Sprintf("getProfit(%s, #%d): profit(%f), dust(%f), sellTotal(%f), buyTotal(%f), inverse(%t)", trade.Symbol, trade.ID, profit, dust, sellTotal, buyTotal, trade.Inverse))

	return profit
}
