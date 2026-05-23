package binanceAdaptor

import (
	"testing"

	"github.com/giovani-sirbu/mercury/exchange/aggregates"
)

func TestGetFuturesBinanceActionsExposesKlineData(t *testing.T) {
	actions := GetFuturesBinanceActions(aggregates.Exchange{Name: "binance"})

	if actions.KlineData == nil {
		t.Fatal("expected futures KlineData action to be configured")
	}
}
