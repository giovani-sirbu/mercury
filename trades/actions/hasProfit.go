package actions

import (
	"fmt"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/log"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func HasProfit(event events.Events) (events.Events, error) {
	// get trade quantities and history type
	quantity, historyType := GetQuantities(event)

	// simulate sell event to calculate profit & get trade profit
	trade := event.Trade
	trade.History = append(trade.History, aggragates.TradesHistory{
		Type:     historyType,
		Quantity: quantity,
		Price:    subtractToleranceFromPrice(event.Trade), // deduct strategy settings tolerance from position price to simulate unrealised PnL
	})

	// get gross profit
	profit := GetProfit(trade)

	// return event fees & multiply by 2 to simulate total fees for the sell event
	fees := GetFees(event)
	fees *= 2

	// subtract fees and return net profit
	profit -= fees

	// assign net profit to trade
	event.Trade.Profit = profit
	event.Params.Profit = profit

	// get min profit
	minProfit := CalculateMinProfit(event.Trade)

	log.Debug(fmt.Sprintf("hasProfit(TradeID:#%d): PositionPrice(%f), minProfit(%f), fees(%f), netProfit(%f)", event.Trade.ID, event.Trade.PositionPrice, minProfit, fees, profit))

	if profit < minProfit {
		msg := fmt.Sprintf("profit(%f) is smaller than min profit(%f) for symbol %s | trade_id: %d | user_id: %d", profit, minProfit, event.Trade.Symbol, event.Trade.ID, event.Trade.UserID)
		return event, fmt.Errorf(msg)
	}

	fmt.Println("")

	return event, nil
}

// deduct strategy settings tolerance from position price to simulate unrealised PnL
func subtractToleranceFromPrice(trade aggragates.Trades) float64 {
	toleranceAmount := trade.PositionPrice * (trade.StrategyPair.StrategySettings[0].Tolerance / 100)
	if trade.Inverse {
		trade.PositionPrice += toleranceAmount
	} else {
		trade.PositionPrice -= toleranceAmount
	}

	return ToFixed(trade.PositionPrice, int(trade.StrategyPair.TradeFilters.PriceFilter))
}
