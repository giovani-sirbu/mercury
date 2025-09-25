package actions

import (
	"github.com/giovani-sirbu/mercury/events"
)

// CalculateProfit is used in Agora service to return total profit for closed pending orders
func CalculateProfit(event events.Events) float64 {
	trade := event.Trade

	// get trade profit
	profit := GetProfit(trade)

	// return event fees
	fees := GetFees(event)

	return profit - fees
}
