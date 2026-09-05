package profit

import (
	"fmt"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/log"
	"github.com/giovani-sirbu/mercury/trades/fees"
)

// CalculateProfit is used in Agora service to return total profit for closed pending orders
func CalculateProfit(event events.Events) float64 {
	trade := event.Trade

	// get trade profit
	grossProfit := GetProfit(trade)

	// Only the settlement side of the fees comes off here: the opening leg's
	// commission is already embodied in the fill quantities GetProfit summed.
	settlementFees := fees.GetSettlementFees(event)

	log.Debug(fmt.Sprintf("CalculateProfit(TradeID:#%d): PositionPrice(%f), profit(%f), fees(%f)", event.Trade.ID, event.Trade.PositionPrice, grossProfit-settlementFees, settlementFees))

	return grossProfit - settlementFees
}
