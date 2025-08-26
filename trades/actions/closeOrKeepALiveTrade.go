package actions

import (
	"fmt"
	"github.com/giovani-sirbu/mercury/events"
)

func CloseOrKeepALiveTrade(event events.Events) (events.Events, error) {
	// If AI action is not in same direction position, close it
	if (event.Params.AIIndicators.AIAction == "LONG" && event.Trade.PositionType == "sell") ||
		(event.Params.AIIndicators.AIAction == "SHORT" && event.Trade.PositionType == "buy") {
		newEvent, newError := CloseFuturesTrade(event)
		return newEvent, newError
	}
	return events.Events{}, fmt.Errorf("position was kept alive")
}
