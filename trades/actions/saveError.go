package actions

import (
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"regexp"
	"strconv"
	"strings"
)

func SaveError(event events.Events, err error) (events.Events, error) {
	message := err.Error()
	errorType := aggragates.LOG_WARNING

	// set error type based on message
	if isAPIError(message) || hasInsufficientBalance(message) {
		event.Trade.Status = aggragates.Blocked
		errorType = aggragates.LOG_ERROR
	}

	// prevent duplicate logs
	if len(event.Trade.Logs) > 0 {
		lastError := event.Trade.Logs[len(event.Trade.Logs)-1].Message
		if RemoveNumbersFromString(lastError) == RemoveNumbersFromString(message) {
			if event.Trade.PositionType == "impasse" && event.Params.OldPosition != "impasse" {
				newEvent, _ := event.Events["updateTrade"](event)

				return newEvent, err
			}
			return event, err
		}
	}

	// Reset price and position to allow only the error update
	price := event.Trade.PositionPrice
	if event.Trade.PositionType != "impasse" {
		event.Trade.PositionType = event.Params.OldPosition
		event.Trade.PositionPrice = event.Params.OldPositionPrice
	}
	event.Params.PreventInfoLog = true

	event.Trade.Logs = append(event.Trade.Logs, aggragates.TradesLogs{
		Percentage: event.Params.Percentage,
		Message:    message,
		Type:       errorType,
		Price:      price,
		Quantity:   event.Params.Quantity,
		TradeID:    event.Trade.ID,
	})

	// add new log if trade was blocked due API or Insufficient Funds Error
	if event.Trade.Status == aggragates.Blocked {
		message = "Trade was paused due Exchange API Error."
		if hasInsufficientBalance(message) {
			message = "Trade was paused due insufficient funds."
		}

		event.Trade.Logs = append(event.Trade.Logs, aggragates.TradesLogs{
			Percentage: event.Params.Percentage,
			Message:    message,
			Type:       errorType,
			Price:      price,
			Quantity:   event.Params.Quantity,
			TradeID:    event.Trade.ID,
		})
	}

	newEvent, _ := event.Events["updateTrade"](event)

	return newEvent, err
}

func isAPIError(input string) bool {
	return strings.Contains(RemoveNumbersFromString(input), "<APIError>")
}

func hasInsufficientBalance(input string) bool {
	if strings.Contains(RemoveNumbersFromString(input), "Available quantity:") {

		// Regular expression to match floating-point numbers
		re := regexp.MustCompile(`\d+\.\d+`)

		// Find all matches in the input string
		matches := re.FindAllString(input, -1)

		// Check if we have at least 2 numbers
		if len(matches) < 2 {
			return false
		}

		// Convert first two matches to float64
		requiredQty, _ := strconv.ParseFloat(strings.TrimSpace(matches[0]), 64)
		availableQty, _ := strconv.ParseFloat(strings.TrimSpace(matches[1]), 64)

		return requiredQty > availableQty
	}
	
	if strings.Contains(RemoveNumbersFromString(input), "Insufficient funds") {
		return true
	}

	return false
}
