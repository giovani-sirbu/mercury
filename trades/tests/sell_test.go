package tests

import (
	"github.com/giovani-sirbu/mercury/events"
	"github.com/giovani-sirbu/mercury/trades/actions"
	"github.com/giovani-sirbu/mercury/trades/aggragates"
	"github.com/giovani-sirbu/mercury/virtualExchange"
	"testing"
)

func SellFeeQtyInverseHasFunds() events.Events {
	var tradesHistory []aggragates.TradesHistory

	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 36.52, Price: 0.000031, Type: "SELL", Fees: []aggragates.TradesFees{{Asset: "BTC", Fee: 0.00000113}}})
	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 73.04, Price: 0.0000317, Type: "SELL", Fees: []aggragates.TradesFees{{Asset: "BTC", Fee: 0.0000023200000000000002}}})
	tradesHistory = append(tradesHistory, aggragates.TradesHistory{Quantity: 146.08, Price: 0.0000325, Type: "SELL", Fees: []aggragates.TradesFees{{Asset: "BTC", Fee: 0.00000475}}})

	trade := aggragates.Trades{}
	trade.History = tradesHistory
	trade.PositionPrice = 0.0000318
	trade.Inverse = true
	trade.Symbol = "DOT/BTC"
	trade.PositionType = "takeProfit"
	trade.StrategyPair.TradeFilters = aggragates.TradeFilters{LotSize: 2, MinNotional: 0.0001, PriceFilter: 7}
	trade.StrategyPair.StrategySettings = append(trade.StrategyPair.StrategySettings,
		aggragates.StrategySettings{MinDepths: 6, Depths: 8, Percentage: 2, Multiplier: 2, Tolerance: 0.25, InitialBid: 0.5},
	)
	defaultActions := actions.GetDefaultActions()
	virtualExchange.ResetWallet()
	exchangeInit := GetVirtualExchange("BTC", "0.00812")
	defaultActions["updateTrade"] = EmptyUpdateTrade
	newEvent := events.Events{Trade: trade, Exchange: exchangeInit, Events: defaultActions, EventsNames: []string{"hasFunds"}}
	return newEvent
}

func TestSell(t *testing.T) {
	var tests []hasFundsTests

	tests = append(tests, hasFundsTests{SellFeeQtyInverseHasFunds, "Next buy, no history"})

	for _, test := range tests {
		event := test.Event()
		nextEvent, err := actions.Sell(event)

		if err != nil {
			t.Fatalf("Failed with error: %s", err)
		}

		t.Logf("PASS: Correct quantity %f when testing %s", nextEvent.Params.Quantity, test.Name)

	}

}
