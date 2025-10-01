package actions

import (
	"fmt"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/log"
)

// CalculateProfit is used in Agora service to return total profit for closed pending orders
func CalculateProfit(event events.Events) float64 {
	trade := event.Trade

	// get trade profit
	profit := GetProfit(trade)

	// return event fees
	fees := GetFees(event)

	log.Debug(fmt.Sprintf("CalculateProfit(TradeID:#%d): PositionPrice(%f), profit(%f), fees(%f)", event.Trade.ID, event.Trade.PositionPrice, profit-fees, fees))

	return profit - fees
}
