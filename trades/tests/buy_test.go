package tests

import (
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/actions"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"testing"
)

type buyTests struct {
	Event            events.Events
	ExpectedQuantity float64
	FailCase         bool
	Name             string
}

func SimpleCase() events.Events {
	var tradesHistory []aggragates.TradesHistory
	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 2, Price: 5.029, Type: "BUY"})
	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 4, Price: 5.158, Type: "BUY"})
	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 8, Price: 5.281, Type: "BUY"})

	trade := aggragates.Trades{}
	trade.History = tradesHistory
	trade.PositionPrice = 6.315
	trade.Inverse = false
	trade.Symbol = "ATOM/USDT"
	trade.PositionType = "buy"
	trade.StrategyPair.TradeFilters = aggragates.TradeFilters{LotSize: 2, MinNotional: 5, PriceFilter: 3}
	trade.StrategyPair.StrategySettings = append(trade.StrategyPair.StrategySettings,
		aggragates.StrategySettings{MinDepths: 6, Depths: 8, Percentage: 2, Multiplier: 2, Tolerance: 0.25, InitialBid: 0.5},
	)
	defaultActions := actions.GetDefaultActions()
	exchangeInit := GetVirtualExchange("USDT", "1000")
	newEvent := events.Events{Trade: trade, Exchange: exchangeInit, Events: defaultActions, EventsNames: []string{"buy"}}
	return newEvent
}

func NoHistory() events.Events {
	var tradesHistory []aggragates.TradesHistory

	trade := aggragates.Trades{}
	trade.History = tradesHistory
	trade.PositionPrice = 5.0
	trade.Inverse = false
	trade.Symbol = "ATOM/USDT"
	trade.PositionType = "buy"
	trade.StrategyPair.TradeFilters = aggragates.TradeFilters{LotSize: 2, MinNotional: 5, PriceFilter: 3}
	trade.StrategyPair.StrategySettings = append(trade.StrategyPair.StrategySettings,
		aggragates.StrategySettings{MinDepths: 5, Depths: 8, Percentage: 2, Multiplier: 2, Tolerance: 0.25, TrailingTakeProfit: 0.5},
	)
	defaultActions := actions.GetDefaultActions()
	exchangeInit := GetVirtualExchange("USDT", "1000")
	newEvent := events.Events{Trade: trade, Exchange: exchangeInit, Events: defaultActions, EventsNames: []string{"buy"}}
	return newEvent
}

func Inverse() events.Events {
	var tradesHistory []aggragates.TradesHistory
	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 2, Price: 5.281, Type: "SELL"})
	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 4, Price: 5.158, Type: "SELL"})
	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 8, Price: 5.029, Type: "SELL"})

	trade := aggragates.Trades{}
	trade.History = tradesHistory
	trade.PositionPrice = 5.0
	trade.Inverse = true
	trade.Symbol = "ATOM/USDT"
	trade.PositionType = "buy"
	trade.StrategyPair.TradeFilters = aggragates.TradeFilters{LotSize: 2, MinNotional: 5, PriceFilter: 3}
	trade.StrategyPair.StrategySettings = append(trade.StrategyPair.StrategySettings,
		aggragates.StrategySettings{MinDepths: 5, Depths: 8, Percentage: 2, Multiplier: 2, Tolerance: 0.25, TrailingTakeProfit: 0.5},
	)
	defaultActions := actions.GetDefaultActions()
	exchangeInit := GetVirtualExchange("USDT", "1000")
	newEvent := events.Events{Trade: trade, Exchange: exchangeInit, Events: defaultActions, EventsNames: []string{"buy"}}
	return newEvent
}

func TestBuy(t *testing.T) {
	var tests []buyTests

	tests = append(tests, buyTests{NoHistory(), 1.36, false, "Next buy, no history"})
	tests = append(tests, buyTests{NoHistory(), 1.08, true, "Next buy, no history case incorrect qty"})
	tests = append(tests, buyTests{SimpleCase(), 16.0, false, "Next buy case"})
	tests = append(tests, buyTests{SimpleCase(), 15.0, true, "Next buy case incorrect qty"})
	tests = append(tests, buyTests{Inverse(), 16.0, false, "Next buy inverse case"})

	for _, test := range tests {
		nextEvent, err := actions.Buy(test.Event)

		if err != nil {
			t.Fatalf("Failed with error: %s", err)
		}

		if nextEvent.Params.Quantity != test.ExpectedQuantity {
			if test.FailCase {
				t.Logf("PASS: Incorrect quantity %f as expected when testing %s", test.ExpectedQuantity, test.Name)

			} else {
				t.Errorf("Incorrect quantity %f, wanted %f, when testing %s", nextEvent.Params.Quantity, test.ExpectedQuantity, test.Name)
			}
		} else {
			t.Logf("PASS: Correct quantity %f when testing %s", nextEvent.Params.Quantity, test.Name)
		}
	}

}
