package actions

import (
	"fmt"
	"strings"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// saveHoldLog records a hold as an INFO entry in the trade logs, restores the
// previous position and stops the action chain. Consecutive holds for the
// same position are collapsed into their first entry, so flip-flopping hold
// reasons do not spam the trade history.
func saveHoldLog(event events.Events, position string, reason string) (events.Events, error) {
	message := fmt.Sprintf("Hold %s: %s", position, reason)
	err := fmt.Errorf("%s", message)

	if len(event.Trade.Logs) > 0 {
		last := event.Trade.Logs[len(event.Trade.Logs)-1]
		if strings.HasPrefix(last.Message, fmt.Sprintf("Hold %s:", position)) {
			return event, err
		}
	}

	// Reset price and position so only the hold entry is persisted.
	price := event.Trade.PositionPrice
	event.Trade.PositionType = event.Params.OldPosition
	event.Trade.PositionPrice = event.Params.OldPositionPrice
	event.Params.PreventInfoLog = true

	event.Trade.Logs = append(event.Trade.Logs, aggragates.TradesLogs{
		Percentage: event.Params.Percentage,
		Message:    message,
		Type:       aggragates.LOG_INFO,
		Price:      price,
		Quantity:   event.Params.Quantity,
		TradeID:    event.Trade.ID,
	})

	newEvent, _ := event.Events["updateTrade"](event)

	return newEvent, err
}
