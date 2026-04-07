package tests

import (
	"math"
	"testing"

	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/actions"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/virtualExchange"
)

// EmptyUpdateTrade is a no-op mock for the updateTrade action.
func EmptyUpdateTrade(event events.Events) (events.Events, error) {
	return event, nil
}

// AssertFloatEqual fails the test if got and want differ by more than epsilon.
func AssertFloatEqual(t *testing.T, got, want, epsilon float64, msg string) {
	t.Helper()
	if math.Abs(got-want) > epsilon {
		t.Errorf("%s: got %f, want %f (epsilon %g)", msg, got, want, epsilon)
	}
}

// AssertNoError fails the test if err is non-nil.
func AssertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
}

// AssertError fails the test if err is nil.
func AssertError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// DefaultTradeFilters returns standard trade filters for testing.
func DefaultTradeFilters() aggragates.TradeFilters {
	return aggragates.TradeFilters{LotSize: 2, MinNotional: 5, PriceFilter: 3}
}

// DefaultStrategySettings returns standard strategy settings for testing.
func DefaultStrategySettings() []aggragates.StrategySettings {
	return []aggragates.StrategySettings{
		{MinDepths: 6, Depths: 8, Percentage: 2, Multiplier: 2, Tolerance: 0.25, InitialBid: 0.5},
	}
}

// MakeEvent creates a test event with virtual exchange, default actions, and no-op updateTrade.
func MakeEvent(trade aggragates.Trades, quoteAsset string, quoteBalance string, eventNames []string) events.Events {
	virtualExchange.ResetWallet()
	exchangeInit := GetVirtualExchange(quoteAsset, quoteBalance)
	defaultActions := actions.GetDefaultActions()
	defaultActions["updateTrade"] = EmptyUpdateTrade
	return events.Events{
		Trade:       trade,
		Exchange:    exchangeInit,
		Events:      defaultActions,
		EventsNames: eventNames,
	}
}

// MakeTrade creates a trade with sane defaults for symbol, filters, and settings.
func MakeTrade(symbol string, positionPrice float64, inverse bool, history []aggragates.TradesHistory) aggragates.Trades {
	return aggragates.Trades{
		Symbol:        symbol,
		PositionPrice: positionPrice,
		Inverse:       inverse,
		History:       history,
		StrategyPair: aggragates.StrategiesPairs{
			TradeFilters:     DefaultTradeFilters(),
			StrategySettings: DefaultStrategySettings(),
		},
	}
}
