package tests

import (
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/actions"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/virtualExchange"
	"testing"
)

func EmptyUpdateTrade(event events.Events) (events.Events, error) {
	return event, nil
}

func SimpleCaseHasEnoughFunds() events.Events {
	var tradesHistory []aggragates.TradesHistory
	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 2, Price: 5.029, Type: "BUY"})
	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 4, Price: 5.158, Type: "BUY"})
	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: -2, Price: 0.0000000000001, Type: "BUY"})

	trade := aggragates.Trades{}
	trade.History = tradesHistory
	trade.PositionPrice = 6.315
	trade.Inverse = false
	trade.Symbol = "ATOM/USDT"
	trade.PositionType = "sellParent"
	trade.StrategyPair.TradeFilters = aggragates.TradeFilters{LotSize: 2, MinNotional: 5, PriceFilter: 3}
	trade.StrategyPair.StrategySettings = append(trade.StrategyPair.StrategySettings,
		aggragates.StrategySettings{MinDepths: 6, Depths: 8, Percentage: 2, Multiplier: 2, Tolerance: 0.25, InitialBid: 0.5},
	)
	defaultActions := actions.GetDefaultActions()
	virtualExchange.ResetWallet()
	exchangeInit := GetVirtualExchange("ATOM", "3.5")
	defaultActions["updateTrade"] = EmptyUpdateTrade
	newEvent := events.Events{Trade: trade, Exchange: exchangeInit, Events: defaultActions, EventsNames: []string{"hasEnoughFunds"}}
	return newEvent
}

func TestHasEnoughFunds(t *testing.T) {
	var tests []buyTests

	tests = append(tests, buyTests{SimpleCaseHasEnoughFunds(), 1.36, false, "Next buy, no history"})

	for _, test := range tests {
		nextEvent, err := actions.HasEnoughFunds(test.Event)

		remainedQuantity, neededQuantity, _, _ := actions.GetFundsQuantities(nextEvent)

		if err != nil {
			t.Fatalf("Failed with error: %s", err)
		}

		if remainedQuantity < neededQuantity {
			t.Errorf("Incorrect quantity %f, wanted %f, when testing %s", nextEvent.Params.Quantity, test.ExpectedQuantity, test.Name)
		} else {
			t.Logf("PASS: Correct quantity %f when testing %s", nextEvent.Params.Quantity, test.Name)
		}
	}

}
