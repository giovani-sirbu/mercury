package actions_test

import (
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/actions"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/trades/funds"
	"github.com/giovani-sirbu/mercury/trades/internal/virtualexchange"
	"testing"
)

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
	virtualexchange.ResetWallet()
	exchangeInit := GetVirtualExchange("ATOM", "3.5")
	defaultActions["updateTrade"] = EmptyUpdateTrade
	newEvent := events.Events{Trade: trade, Exchange: exchangeInit, Events: defaultActions, EventsNames: []string{"hasEnoughFunds"}}
	return newEvent
}

func TestHasEnoughFunds(t *testing.T) {
	type hasEnoughCase struct {
		Event events.Events
		Name  string
	}

	cases := []hasEnoughCase{
		{SimpleCaseHasEnoughFunds(), "Next buy, no history"},
	}

	for _, c := range cases {
		nextEvent, err := actions.HasEnoughFunds(c.Event)
		if err != nil {
			t.Fatalf("Failed with error: %s", err)
		}

		remainedQuantity, neededQuantity, _, _ := funds.GetFundsQuantities(nextEvent)
		if remainedQuantity < neededQuantity {
			t.Errorf("Insufficient wallet after HasEnoughFunds for %s: remained %f, needed %f",
				c.Name, remainedQuantity, neededQuantity)
		}
	}
}
