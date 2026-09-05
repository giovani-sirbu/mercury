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

func ParentTradeHasProfit(event events.Events) (events.Events, error) {
	// the quantity the close would actually submit (net of embodied fees)
	quantity, historyType := quantities.SimulatedCloseQuantity(event)

	// simulate sell event to calculate profit & get trade profit
	trade := event.Trade
	trade.History = append(trade.History, aggragates.TradesHistory{
		Type:     historyType,
		Quantity: quantity,
		Price:    trade.PositionPrice,
	})

	// get gross profit
	netProfit := profit.GetProfit(trade)

	// One leg's worth estimates the closing fee the simulated fill lacks, plus
	// whatever opening commission the fill quantities do not embody (see
	// HasProfit and fees.UnembodiedOpeningFees).
	closingFees := fees.GetFees(event)
	openingFees := fees.UnembodiedOpeningFees(event)

	// subtract fees and return net profit
	netProfit -= closingFees + openingFees

	// gen children profits
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

		// Each child answers in its OWN denomination: HasProfit gives a spot
		// child its quote and an inverse child its base, so an inverse child
		// is converted with its own price, never the parent's.
		childProfit := newEvent.Trade.Profit
		if childrenTrade.Inverse {
			childProfit *= childrenPrice
		}
		childrenProfit += childProfit
	}

	// Sum children profit into the parent's. Both sides are quote amounts now,
	// so this is an addition.
	//
	// It used to be `childrenProfit * event.Trade.PositionPrice`, which read
	// the children's quote profit as if it were denominated in the parent's
	// BASE asset and inflated it by the parent's whole price — on a BTC parent
	// a child's 3 USDC showed up as 300000. The children are trades on the
	// strategy's ImpassePairs, arbitrary symbols with their own inverse flag,
	// so the parent's price never converted anything for them.
	//
	// This assumes the children quote in the parent's quote asset, which is
	// how ImpassePairs are configured; a child on a different quote would need
	// its own cross rate, which nothing on the event carries.
	netProfit += childrenProfit

	if netProfit < 0 {
		msg := fmt.Sprintf("profit: %f is smaller then min profit for symbol %s, trade id %d, user id %d", netProfit, event.Trade.Symbol, event.Trade.ID, event.Trade.UserID)
		return events.Events{}, fmt.Errorf(msg)
	}

	return event, nil
}
