package actions

import (
	"fmt"
	"github.com/giovani-sirbu/mercury/log"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"strings"
)

func GetProfitOld(history []aggragates.TradesHistory) (float64, float64) {
	var buyTotal float64
	var sellTotal float64
	for _, historyData := range history {
		if strings.ToLower(historyData.Type) == "buy" {
			buyPerHistory := historyData.Price * historyData.Quantity
			buyTotal += buyPerHistory
		} else {
			sellPerHistory := historyData.Price * historyData.Quantity
			sellTotal += sellPerHistory
		}
	}
	return sellTotal, buyTotal
}

func GetProfitInBase(history []aggragates.TradesHistory) (float64, float64) {
	var buyTotal float64
	var sellTotal float64
	for _, historyData := range history {
		if strings.ToLower(historyData.Type) == "buy" {
			buyTotal += historyData.Quantity
		} else {
			sellTotal += historyData.Quantity
		}
	}
	return sellTotal, buyTotal
}

func GetProfit(trade aggragates.Trades) float64 {
	var buyTotal float64
	var sellTotal float64
	var dust float64
	var profit float64

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
