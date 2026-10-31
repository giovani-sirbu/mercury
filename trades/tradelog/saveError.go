package tradelog

import (
	"strings"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/helpers"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
)

// SaveError records a chain failure as a keyed row, restores the position and blocks the trade on a funds or exchange error; a repeat writes nothing.
func SaveError(event events.Events, err error) (events.Events, error) {
	message := err.Error()
	errorType := aggragates.LOG_WARNING
	key := errorKey(message)

	if len(event.Trade.Logs) > 0 && repeatsLastRow(event.Trade.Logs[len(event.Trade.Logs)-1], key, message) {
		if event.Trade.PositionType == "impasse" && event.Params.OldPosition != "impasse" {
			newEvent, _ := event.Events["updateTrade"](event)

			return newEvent, events.Repeated(err)
		}
		return event, events.Repeated(err)
	}

	// set error type based on message
	if isAPIError(message) || HasInsufficientBalance(message) {
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
		MessageKey: key,
		Type:       errorType,
		Price:      price,
		Quantity:   event.Params.Quantity,
		TradeID:    event.Trade.ID,
	})

	newEvent, _ := event.Events["updateTrade"](event)

	return newEvent, err
}

// errorKey is the key of a failure's row: a funds refusal, an exchange error, else any other failure.
func errorKey(message string) string {
	switch {
	case HasInsufficientBalance(message):
		return aggragates.LogKeyInsufficientFunds
	case isAPIError(message):
		return aggragates.LogKeyExchangeError
	}
	return aggragates.LogKeyActionFailed
}

// repeatsLastRow is whether a failure keyed key with message repeats the row: the same key and the same text without numbers.
func repeatsLastRow(row aggragates.TradesLogs, key, message string) bool {
	return row.MessageKey == key && helpers.RemoveNumbersFromString(row.Message) == helpers.RemoveNumbersFromString(message)
}

func isAPIError(input string) bool {
	return strings.Contains(helpers.RemoveNumbersFromString(input), "<APIError>")
}

// HasInsufficientBalance is whether the message is a funds error, the one that blocks the trade.
func HasInsufficientBalance(input string) bool {
	return strings.Contains(helpers.RemoveNumbersFromString(input), "Insufficient funds")
}
