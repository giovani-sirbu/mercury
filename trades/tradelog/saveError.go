// Package tradelog writes the non-action rows of Trade.Logs: the ERROR entry
// an exchange or funds failure leaves on the trade, with the exchange error
// text normalized so repeats collapse.
package tradelog

import (
	"github.com/giovani-sirbu/mercury/helpers"
	"strings"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

func SaveError(event events.Events, err error) (events.Events, error) {
	message := err.Error()
	errorType := aggragates.LOG_WARNING

	// prevent duplicate logs
	if len(event.Trade.Logs) > 0 {
		lastError := event.Trade.Logs[len(event.Trade.Logs)-1].Message
		if helpers.RemoveNumbersFromString(lastError) == helpers.RemoveNumbersFromString(message) {
			if event.Trade.PositionType == "impasse" && event.Params.OldPosition != "impasse" {
				newEvent, _ := event.Events["updateTrade"](event)

				return newEvent, err
			}
			return event, err
		}
	}

	// set error type based on message
	if isAPIError(message) || hasInsufficientBalance(message) {
		event.Trade.Status = aggragates.Blocked
		errorType = aggragates.LOG_ERROR
	}

	// Reset price and position to allow only the error update
	price := event.Trade.PositionPrice
	if event.Trade.PositionType != "impasse" {
		event.Trade.PositionType = event.Params.OldPosition
		event.Trade.PositionPrice = event.Params.OldPositionPrice
	}
	event.Params.PreventInfoLog = true

	if isAPIError(message) {
		message = extractExchangeErrorMessage(message)
	}

	event.Trade.Logs = append(event.Trade.Logs, aggragates.TradesLogs{
		Percentage: event.Params.Percentage,
		Message:    message,
		Type:       errorType,
		Price:      price,
		Quantity:   event.Params.Quantity,
		TradeID:    event.Trade.ID,
	})

	newEvent, _ := event.Events["updateTrade"](event)

	return newEvent, err
}

func isAPIError(input string) bool {
	return strings.Contains(helpers.RemoveNumbersFromString(input), "<APIError>")
}

func hasInsufficientBalance(input string) bool {
	return strings.Contains(helpers.RemoveNumbersFromString(input), "Insufficient funds")
}
