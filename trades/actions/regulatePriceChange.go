package actions

import (
	"errors"
	"fmt"
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/ladder"
)

// RegulatePriceChange refuses a new entry whose position price has not moved
// a full percentage step from the last fill: under it on a long ladder, over
// it on an inverse one.
func RegulatePriceChange(event events.Events) (events.Events, error) {
	var lastPositionPrice float64
	pricePositionError := false

	if event.Trade.Inverse {
		lastPositionPrice = ladder.GetLatestTradePrice(event.Trade.History, "SELL")
		lastPositionPrice += lastPositionPrice * (event.Trade.StrategyPair.StrategySettings[0].Percentage / 100)

		pricePositionError = event.Trade.PositionPrice < lastPositionPrice
	} else {
		lastPositionPrice = ladder.GetLatestTradePrice(event.Trade.History, "BUY")
		lastPositionPrice -= lastPositionPrice * (event.Trade.StrategyPair.StrategySettings[0].Percentage / 100)

		pricePositionError = event.Trade.PositionPrice > lastPositionPrice
	}

	if pricePositionError {
		var err = fmt.Sprintf("Current price %f is bigger than last position price %f", event.Trade.PositionPrice, lastPositionPrice)
		return events.Events{}, errors.New(err)
	}
	return event, nil
}
