package crashguard

import (
	"strings"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func logCapitulation(event events.Events, message string) events.Events {
	if capitulationAlreadyLogged(event.Trade.Logs, message) {
		return event
	}
	event.Trade.Logs = append(event.Trade.Logs, aggragates.TradesLogs{
		Percentage: event.Params.Percentage,
		Message:    message,
		Type:       aggragates.LOG_INFO,
		Price:      event.Trade.PositionPrice,
		Quantity:   event.Params.Quantity,
		TradeID:    event.Trade.ID,
	})
	return event
}

func capitulationAlreadyLogged(logs []aggragates.TradesLogs, message string) bool {
	prefix := capitulationLogPrefix(message)
	for i := len(logs) - 1; i >= 0; i-- {
		msg := logs[i].Message
		if strings.HasPrefix(msg, CapitulationFreezeOffPrefix) {
			return false
		}
		if strings.HasPrefix(msg, prefix) {
			return true
		}
	}
	return false
}

func capitulationLogPrefix(message string) string {
	switch {
	case strings.HasPrefix(message, CapitulationFreezeOffPrefix):
		return CapitulationFreezeOffPrefix
	case strings.HasPrefix(message, CapitulationFreezeOnPrefix):
		return CapitulationFreezeOnPrefix
	case strings.HasPrefix(message, CapitulationAllowedPrefix):
		return CapitulationAllowedPrefix
	default:
		return CapitulationTaggedPrefix
	}
}
