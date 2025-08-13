package actions

import (
	"errors"
	"fmt"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades"
)

// RegulatePriceChange ensures the price of a new buy position does not exceed the last buy position price adjusted by a percentage threshold.
// It takes an event containing trade details, retrieves the latest buy trade price from the trade history, and compares it against the current position price.
// The last buy price is reduced by a percentage specified in the trade's strategy settings to set a maximum allowable price.
// If the current position price exceeds this threshold, an error is returned with a descriptive message.
// Otherwise, the input event is returned unchanged with a nil error.
//
// Parameters:
//   - event: The events.Events struct containing trade details, including the trade history and position price.
//
// Returns:
//   - events.Events: The input event if the price is within the allowed threshold, or an empty events.Events struct if an error occurs.
//   - error: Nil if the price is valid, or an error if the current position price exceeds the adjusted last position price.
func RegulatePriceChange(event events.Events) (events.Events, error) {
	var lastPositionPrice = trades.GetLatestTradePrice(event.Trade.History, "BUY")

	// subtract strategy percentage
	lastPositionPrice -= lastPositionPrice * (event.Trade.StrategyPair.StrategySettings[0].Percentage / 100)
	if event.Trade.PositionPrice > lastPositionPrice {
		var err = fmt.Sprintf("Current price %f is bigger than last position price %f", event.Trade.PositionPrice, lastPositionPrice)
		return events.Events{}, errors.New(err)
	}
	return event, nil
}
