package actions

import (
	"fmt"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/helpers"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/fees"
	"github.com/giovani-sirbu/mercury/trades/profit"
	"github.com/giovani-sirbu/mercury/trades/quantities"
)

// ParentTradeHasProfit refuses an impasse parent's exit when the parent's
// simulated close plus every child's own net profit, converted to quote,
// totals under zero.
func ParentTradeHasProfit(event events.Events) (events.Events, error) {
	quantity, dust, historyType := quantities.SimulatedClose(event)

	trade := event.Trade

	// Sell keeps the remainder as Dust and GetProfit credits it at the position price.
	trade.Dust = dust

	trade.History = append(trade.History, aggragates.TradesHistory{
		Type:     historyType,
		Quantity: quantity,
		Price:    trade.PositionPrice,
	})

	netProfit := profit.GetProfit(trade)

	// One closing leg plus the opening commissions the quantities do not embody.
	closingFees := fees.GetFees(event)
	openingFees := fees.UnembodiedOpeningFees(event)

	netProfit -= closingFees + openingFees

	var childrenProfit float64
	client, _ := event.Exchange.Client()
	for index, childrenTrade := range event.ChildrenTrades {
		childrenPrice, priceErr := client.GetPrice(childrenTrade.Symbol)

		if priceErr != nil || childrenPrice == 0 {
			return events.Events{}, fmt.Errorf("failed to get children price")
		}

		childrenPrice = helpers.ToFixed(childrenPrice, int(childrenTrade.StrategyPair.TradeFilters.PriceFilter))

		childrenTrade.PositionPrice = childrenPrice
		event.ChildrenTrades[index].PositionPrice = childrenPrice
		newEvent := events.Events{Trade: childrenTrade, Events: event.Events, EventsNames: []string{"hasProfit"}}
		newEvent, _ = event.Events["hasProfit"](newEvent)

		// Each child answers in its own denomination; an inverse child converts at its own price.
		childProfit := newEvent.Trade.Profit
		if childrenTrade.Inverse {
			childProfit *= childrenPrice
		}
		childrenProfit += childProfit
	}

	// Both sides are quote amounts: ImpassePairs quote in the parent's quote asset.
	netProfit += childrenProfit

	if netProfit < 0 {
		msg := fmt.Sprintf("profit: %f is smaller then min profit for symbol %s, trade id %d, user id %d", netProfit, event.Trade.Symbol, event.Trade.ID, event.Trade.UserID)
		return events.Events{}, fmt.Errorf(msg)
	}

	return event, nil
}
