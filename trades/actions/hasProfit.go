package actions

import (
	"fmt"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/log"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/fees"
	"github.com/giovani-sirbu/mercury/trades/profit"
	"github.com/giovani-sirbu/mercury/trades/quantities"
)

func HasProfit(event events.Events) (events.Events, error) {
	// the quantity the close would actually submit (net of embodied fees)
	quantity, historyType := quantities.SimulatedCloseQuantity(event)

	// simulate sell event to calculate profit & get trade profit
	trade := event.Trade
	trade.History = append(trade.History, aggragates.TradesHistory{
		Type:     historyType,
		Quantity: quantity,
		Price:    profit.SimulatedClosePrice(event.Trade),
	})

	// get gross profit
	netProfit := profit.GetProfit(trade)

	// Estimate the closing leg's fee, which the simulated fill above does not
	// carry. The close sells what the whole ladder bought, so the accrued
	// opening fees cross-converted by GetFees are that same notional at the
	// same rate — one leg's worth.
	closingFees := fees.GetFees(event)

	// The opening legs are only paid for by the smaller quantity when the
	// exchange took their commission out of the asset the account received.
	// Whatever it took from somewhere else — a third-asset (BNB) fee, or a
	// quote fee on a spot trade — is embodied nowhere and has to be charged
	// here, or the round trip pays one leg where it owes two.
	openingFees := fees.UnembodiedOpeningFees(event)

	// subtract fees and return net profit
	netProfit -= closingFees + openingFees

	// assign net profit to trade
	event.Trade.Profit = netProfit
	event.Params.Profit = netProfit

	// get min profit
	minProfit := profit.CalculateMinProfit(event.Trade)

	log.Debug(fmt.Sprintf("hasProfit(TradeID:#%d): PositionPrice(%f), minProfit(%f), fees(%f), netProfit(%f)", event.Trade.ID, event.Trade.PositionPrice, minProfit, closingFees, netProfit))

	if netProfit < minProfit {
		msg := fmt.Sprintf("profit(%f) is smaller than min profit(%f) for symbol %s | trade_id: %d | user_id: %d", netProfit, minProfit, event.Trade.Symbol, event.Trade.ID, event.Trade.UserID)
		return event, fmt.Errorf(msg)
	}

	log.Debug("")

	return event, nil
}
