package adaptors

import (
	"fmt"
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
