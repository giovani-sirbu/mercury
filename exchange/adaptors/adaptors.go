package adaptors

import (
	"fmt"
	"github.com/adshao/go-binance/v2/common"
	binanceAdaptor "github.com/giovani-sirbu/mercury/exchange/adaptors/binance"
	"github.com/giovani-sirbu/mercury/exchange/aggregates"
)

// GetExchangeActions method to fetch actions by exchange name
func GetExchangeActions(e aggregates.Exchange) (aggregates.Actions, error) {
	if e.Name == "" {
		return aggregates.Actions{}, fmt.Errorf("missing required payload")
	}
	if e.Name == "binance" {
		actions := binanceAdaptor.GetBinanceActions(e)
		return actions, nil
	}
	return aggregates.Actions{}, fmt.Errorf("exchange not allowed")
}

// ConvertErrorType converts a given error into an APIError
func ApiError(err error) *common.APIError {
	if err == nil {
		return nil // Return nil if there's no error
	}

	// Check if the error is already an APIError
	if apiErr, ok := err.(*common.APIError); ok {
		return &common.APIError{
			Code:    apiErr.Code,
			Message: apiErr.Message,
		}
	}

	// Check if the error is a Binance API error
	if binanceErr, ok := err.(*common.APIError); ok {
		return &common.APIError{
			Code:    int64(binanceErr.Code),
			Message: binanceErr.Message,
		}
	}

	// Handle other unknown errors as a generic APIError
	return &common.APIError{
		Code:    0, // No specific error code
		Message: err.Error(),
	}
}
